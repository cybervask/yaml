package yaml

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	yaml4 "go.yaml.in/yaml/v4"
)

// includeInfo holds metadata about a field that was loaded via a !include directive.
type includeInfo struct {
	YamlPath string
	Value    any
	FilePath string
}

// DumpOpts holds optional parameters for DumpWithInclude.
type DumpOpts struct {
	// RelativeTo makes include paths in the output YAML relative to this directory.
	RelativeTo string
}

// DumpOption configures the behavior of DumpWithInclude.
type DumpOption func(*DumpOpts)

// WithRelativeIncludes configures DumpWithInclude to output !include paths relative to the given directory.
func WithRelativeIncludes(dir string) DumpOption {
	return func(o *DumpOpts) {
		o.RelativeTo = dir
	}
}

// includeTag implements yaml.Marshaler to output a !include directive.
type includeTag struct {
	Path string
}

// MarshalYAML implements the yaml.Marshaler interface.
func (i includeTag) MarshalYAML() (interface{}, error) {
	return yaml4.Node{
		Kind:  yaml4.ScalarNode,
		Tag:   "!include",
		Value: i.Path,
	}, nil
}

// DumpWithInclude serializes a struct to YAML while preserving !include directives.
// It writes included subtrees to their original files using atomic two-phase
// writes and replaces them with !include tags in the output.
//
// The write-back is bound to the loaded object: include paths are taken from
// the include session recorded when the SAME pointer was passed to [Load] or
// [LoadFile]. A struct that was never loaded through such a pointer — including
// a copy of a loaded value or an object loaded by the streaming [Loader] — is
// dumped in a safe mode: values are written inline, no !include markers are
// synthesized and no files are modified. Files whose paths were not recorded
// for this object's session are never written.
//
// Scope of the write-back: only !include markers that sit directly in the
// main config are written back, plus a marker that occupies the whole main
// document (a root include — the include file receives the dumped value and
// the main document becomes just the marker line). A marker inside an
// included file (second and deeper level of include nesting) is never
// written back: its file keeps its original bytes, and so does the file that
// carries such nested markers, because rewriting that file from the Go value
// would erase the markers it holds. Changes made to nested include values
// after the load are therefore not persisted.
//
// A field that carried several !include markers (e.g. the sequence
// "servers: [!include a.yaml, !include b.yaml]") has no single marker to
// substitute back; the load keeps such configs working, but the dump fails
// with an explicit error before any filesystem action.
//
// The main document is rebuilt from the Go struct: fields are emitted in
// struct order (not in the alphabetical order of a map), omitempty drops
// zero-valued fields — an omitted include field is also not written back, so
// no file is rewritten behind a marker the output would not contain — and a
// nil pointer field is emitted as "name: null", matching [Dump]. Flow-style
// ("...,flow"), inline and embedded (anonymous) fields are not special-cased
// and are emitted as regular block fields.
//
// All include files are first written to temporary files in their target
// directories; every temporary file is flushed with fsync before it is
// renamed. Then every existing target is backed up by renaming it aside
// under a unique temporary name in the same directory (a pre-existing
// "<target>.bak" file belonging to the user is never touched), and only after
// all backups succeeded are the temporary files renamed onto their final
// paths. Backing up and writing are two separate renames: between them the
// target path briefly does not exist, so the pair is not atomic against a
// crash in that window — a crashed dump leaves the target restorable only
// from its backup file. If any step fails, the pre-dump state is restored —
// backups are moved back, newly created files are removed, temporary files
// are cleaned up and the directories created by this dump are removed,
// deepest first — so no include file is left modified.
// The permission bits of an existing include file are preserved; a missing
// file is created with the default 0644 permissions.
//
// When two fields were loaded from the same include file (a diamond —
// possibly through different spellings of one and the same file, such as a
// symlink or a hardlink), they must agree: values whose marshaled content
// differs are reported as an explicit error before any filesystem action,
// while agreeing values are written back once per target. For a target
// reached through a symlink the write goes to the symlink-resolved path so
// the link itself survives; hardlink siblings of a written target keep their
// old bytes, because renaming breaks the link.
func DumpWithInclude(in any, opts ...DumpOption) ([]byte, error) {
	cfg := &DumpOpts{}
	for _, o := range opts {
		o(cfg)
	}

	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("DumpWithInclude: expected struct or struct pointer, got %s", v.Kind())
	}

	session := lookupIncludeSession(in)

	var includes []includeInfo
	collectIncludesRecursive(v, "", &includes, session)

	// A field that carried several !include markers cannot be represented by
	// the write-back, which replaces a whole field value with one marker.
	// The load keeps such configs working, so the dump fails loudly instead
	// of silently collapsing the markers and orphaning the extra files —
	// before any filesystem action is taken.
	for _, inc := range includes {
		if session.isMulti(inc.YamlPath) {
			return nil, fmt.Errorf("DumpWithInclude: multiple !include entries under one field are not supported (yaml path %s)", inc.YamlPath)
		}
	}

	type pendingWrite struct {
		TempPath    string
		FinalPath   string
		BackupPath  string
		HadOriginal bool
		Written     bool
	}
	var pendingWrites []pendingWrite
	var tempFiles []string
	var createdDirs []string

	cleanupTemps := func() {
		for _, tmp := range tempFiles {
			_ = os.Remove(tmp)
		}
	}

	// rollback restores the pre-dump state of every include target that has
	// been touched so far, walking the list in reverse order. It also removes
	// the temporary files and the directory levels created by this dump's
	// MkdirAll calls, deepest first; directory removal errors are ignored —
	// a directory that ended up holding a restored file must survive. If
	// restoring fails, the first failure is returned so the caller can
	// mention it alongside the original error.
	rollback := func() error {
		var firstErr error
		for i := len(pendingWrites) - 1; i >= 0; i-- {
			pw := &pendingWrites[i]
			if pw.HadOriginal && pw.BackupPath != "" {
				if err := os.Rename(pw.BackupPath, pw.FinalPath); err != nil && firstErr == nil {
					firstErr = fmt.Errorf("failed to restore %s from backup %s: %w", pw.FinalPath, pw.BackupPath, err)
				}
				pw.BackupPath = ""
				pw.HadOriginal = false
			} else if pw.Written && !pw.HadOriginal {
				if err := os.Remove(pw.FinalPath); err != nil && firstErr == nil {
					firstErr = fmt.Errorf("failed to remove newly written %s: %w", pw.FinalPath, err)
				}
				pw.Written = false
			}
		}
		cleanupTemps()
		for i := len(createdDirs) - 1; i >= 0; i-- {
			_ = os.Remove(createdDirs[i])
		}
		return firstErr
	}

	// fail reports err after rolling back every change made so far.
	fail := func(err error) error {
		if rerr := rollback(); rerr != nil {
			return fmt.Errorf("%w (rollback failed: %v)", err, rerr)
		}
		return err
	}

	// Pass 1 (in memory): marshal every include value up front and collapse
	// targets by file identity. Two fields loaded from the same file (a
	// diamond — through the same path or through different spellings of one
	// and the same file, such as a symlink or a hardlink) must carry
	// identical content — the write-back has a single value to commit. A
	// disagreement means one of the fields' changes would be silently
	// overwritten, so the dump fails with an explicit error before any
	// filesystem action is taken: no temporary files exist yet and there is
	// nothing to roll back.
	type includePayload struct {
		Data     []byte
		YamlPath string
	}
	type includeTarget struct {
		// FilePath is the spelling the write-back goes to: the
		// symlink-resolved path when the target exists, so renaming a
		// temporary file over it does not replace the link with a regular
		// file and leave the pointed-to file stale.
		FilePath string
		// Info is the os.Stat result of the target at dump time; nil when
		// the target did not exist yet.
		Info    os.FileInfo
		Payload includePayload
	}
	targets := make([]includeTarget, 0, len(includes))
	for _, inc := range includes {
		data, err := yaml4.Marshal(inc.Value)
		if err != nil {
			return nil, fail(fmt.Errorf("failed to marshal include %s: %w", inc.YamlPath, err))
		}

		info, err := os.Stat(inc.FilePath)
		if err != nil && !os.IsNotExist(err) {
			return nil, fail(fmt.Errorf("DumpWithInclude: failed to inspect include target %s: %w", inc.FilePath, err))
		}
		existed := err == nil

		filePath := inc.FilePath
		if existed {
			if resolved, rerr := filepath.EvalSymlinks(inc.FilePath); rerr == nil {
				filePath = resolved
			}
		}

		merged := -1
		for i := range targets {
			if targets[i].FilePath == filePath {
				merged = i
				break
			}
			if targets[i].Info != nil && existed && os.SameFile(targets[i].Info, info) {
				merged = i
				break
			}
		}
		if merged >= 0 {
			if !bytes.Equal(targets[merged].Payload.Data, data) {
				return nil, fmt.Errorf("DumpWithInclude: conflicting values for include target %s (yaml paths %s and %s)", targets[merged].FilePath, targets[merged].Payload.YamlPath, inc.YamlPath)
			}
			continue // identical content: write the target once
		}
		targets = append(targets, includeTarget{
			FilePath: filePath,
			Info:     info,
			Payload:  includePayload{Data: data, YamlPath: inc.YamlPath},
		})
	}

	// Pass 2 (filesystem): prepare exactly one temporary file per unique
	// include target, as before.
	for _, tgt := range targets {
		finalPath := tgt.FilePath
		dir := filepath.Dir(finalPath)
		trackMissingDirs(dir, &createdDirs)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fail(fmt.Errorf("failed to create dir %s: %w", dir, err))
		}

		// Preserve the permission bits of the existing include file; a
		// missing file is treated as new and gets the default 0644.
		mode := os.FileMode(0o644)
		if info, err := os.Stat(finalPath); err == nil && !info.IsDir() {
			mode = info.Mode().Perm()
		}

		base := filepath.Base(finalPath)
		ext := filepath.Ext(base)
		name := "." + strings.TrimSuffix(base, ext) + ext + ".tmp"

		tmpFile, err := os.CreateTemp(dir, name)
		if err != nil {
			return nil, fail(fmt.Errorf("failed to create temp file for %s: %w", finalPath, err))
		}
		tmpPath := tmpFile.Name()
		tempFiles = append(tempFiles, tmpPath)

		if _, err := tmpFile.Write(tgt.Payload.Data); err != nil {
			_ = tmpFile.Close()
			return nil, fail(fmt.Errorf("failed to write temp file %s: %w", tmpPath, err))
		}
		// Flush the payload to stable storage before the rename: a crash
		// after the rename must never expose an empty or truncated file.
		if err := tmpFile.Sync(); err != nil {
			_ = tmpFile.Close()
			return nil, fail(fmt.Errorf("failed to sync temp file %s: %w", tmpPath, err))
		}
		if err := tmpFile.Close(); err != nil {
			return nil, fail(fmt.Errorf("failed to close temp file %s: %w", tmpPath, err))
		}

		if err := os.Chmod(tmpPath, mode); err != nil {
			return nil, fail(fmt.Errorf("failed to set permissions on %s: %w", tmpPath, err))
		}

		pendingWrites = append(pendingWrites, pendingWrite{
			TempPath:  tmpPath,
			FinalPath: finalPath,
		})
	}

	outMap := buildOutputStruct(v, "", cfg, session)
	out, err := yaml4.Marshal(outMap)
	if err != nil {
		return nil, fail(fmt.Errorf("failed to marshal main config: %w", err))
	}

	// Backup phase: move every existing include file aside within its own
	// directory so the write phase can always complete. Nothing is modified
	// yet — the originals are only relocated under their backup names.
	for i := range pendingWrites {
		pw := &pendingWrites[i]
		info, err := os.Stat(pw.FinalPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // new file, no backup needed
			}
			return nil, fail(fmt.Errorf("failed to inspect include target %s: %w", pw.FinalPath, err))
		}
		if info.IsDir() {
			return nil, fail(fmt.Errorf("include target %s is a directory, refusing to overwrite", pw.FinalPath))
		}
		// The backup name must be unique: a fixed "<target>.bak" suffix would
		// silently overwrite (and later delete) an unrelated user file. A
		// temporary file in the target's directory guarantees no collision.
		bFile, err := os.CreateTemp(filepath.Dir(pw.FinalPath), filepath.Base(pw.FinalPath)+".bak")
		if err != nil {
			return nil, fail(fmt.Errorf("failed to create backup file for %s: %w", pw.FinalPath, err))
		}
		backup := bFile.Name()
		if err := bFile.Close(); err != nil {
			_ = os.Remove(backup)
			return nil, fail(fmt.Errorf("failed to close backup file for %s: %w", pw.FinalPath, err))
		}
		if err := os.Rename(pw.FinalPath, backup); err != nil {
			_ = os.Remove(backup)
			return nil, fail(fmt.Errorf("failed to back up %s: %w", pw.FinalPath, err))
		}
		pw.BackupPath = backup
		pw.HadOriginal = true
	}

	// Write phase: rename the prepared temporary files onto their final
	// paths. On failure every already-written target is rolled back.
	for i := range pendingWrites {
		pw := &pendingWrites[i]
		if err := os.Rename(pw.TempPath, pw.FinalPath); err != nil {
			return nil, fail(fmt.Errorf("failed to atomically write %s: %w", pw.FinalPath, err))
		}
		pw.Written = true
	}

	// Commit succeeded: drop the backups.
	for _, pw := range pendingWrites {
		if pw.HadOriginal && pw.BackupPath != "" {
			_ = os.Remove(pw.BackupPath)
		}
	}

	return out, nil
}

// MarshalWithInclude is a compatibility proxy for DumpWithInclude.
// It aligns the naming convention with the standard yaml.Marshal function.
func MarshalWithInclude(in any, opts ...DumpOption) ([]byte, error) {
	return DumpWithInclude(in, opts...)
}

// collectIncludesRecursive walks the struct hierarchy and gathers every field
// that was loaded via a !include directive, recording its YAML path, value,
// and the absolute filesystem path of the source include file. Paths are
// looked up in session — the include data bound to the dumped object; a nil
// session matches nothing, so nothing is collected for write-back.
//
// Only markers that sit directly in the main config are collected, plus a
// whole-document root include (the entire struct behind the empty path). An
// include field carrying omitempty and a zero value is skipped like any
// omitted field, so its file is not written behind a marker the output would
// not contain; the same applies when the include file itself carries nested
// markers — such a subtree is frozen on dump.
func collectIncludesRecursive(v reflect.Value, path string, out *[]includeInfo, session *includeSession) {
	if !v.IsValid() {
		return
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	// A root !include owns the whole document: the entire struct is the value
	// behind the single marker.
	if path == "" {
		if entry, ok := session.entry(""); ok {
			if !session.hasNestedUnder("") {
				*out = append(*out, includeInfo{
					YamlPath: "",
					Value:    v.Interface(),
					FilePath: entry.AbsolutePath,
				})
			}
			return
		}
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		yamlName, omitEmpty := yamlFieldTag(&field)
		if yamlName == "" || yamlName == "-" {
			continue
		}

		nextPath := yamlName
		if path != "" {
			nextPath = path + "." + yamlName
		}

		fieldValue := v.Field(i)
		if fieldValue.Kind() == reflect.Pointer && fieldValue.IsNil() {
			continue
		}

		if entry, ok := session.entry(nextPath); ok {
			if omitEmpty && isZeroValue(fieldValue) {
				continue
			}
			if session.hasNestedUnder(nextPath) {
				// The include file behind nextPath holds !include markers on
				// its own fields: a write-back of the Go value would erase
				// them, so the whole subtree stays untouched.
				continue
			}
			*out = append(*out, includeInfo{
				YamlPath: nextPath,
				Value:    fieldValue.Interface(),
				FilePath: entry.AbsolutePath,
			})
			// Session entries exist only for markers of the main config, and
			// a matched field cannot carry another such marker deeper, so
			// there is nothing to collect below it.
			continue
		}

		if fieldValue.Kind() == reflect.Struct || (fieldValue.Kind() == reflect.Pointer && fieldValue.Elem().Kind() == reflect.Struct) {
			collectIncludesRecursive(fieldValue, nextPath, out, session)
		}
	}
}

// orderedMap is an insertion-ordered mapping. The rebuilt main document is
// assembled through it so field order follows the Go struct instead of the
// alphabetical order a Go map would produce.
type orderedMap struct {
	keys []string
	vals []any
}

// MarshalYAML implements the yaml.Marshaler interface by rendering the
// ordered pairs as a mapping node.
func (m orderedMap) MarshalYAML() (any, error) {
	node := yaml4.Node{Kind: yaml4.MappingNode}
	for i, key := range m.keys {
		keyNode := yaml4.Node{Kind: yaml4.ScalarNode, Value: key}
		var valueNode yaml4.Node
		if err := valueNode.Dump(m.vals[i]); err != nil {
			return nil, err
		}
		node.Content = append(node.Content, &keyNode, &valueNode)
	}

	return &node, nil
}

// yamlFieldTag splits a struct field's yaml tag into its name and the
// omitempty option. An absent tag falls back to the lowercased field name, a
// tag with an empty name (",inline" family) keeps the empty name so the
// caller can skip the field.
func yamlFieldTag(field *reflect.StructField) (name string, omitEmpty bool) {
	tag, hasTag := field.Tag.Lookup("yaml")
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" && !hasTag {
		name = strings.ToLower(field.Name)
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			omitEmpty = true
		}
	}

	return name, omitEmpty
}

// isZeroValue reports whether v is the zero value for marshaling purposes,
// mirroring the zero test the underlying marshaller applies to omitempty
// fields, including the IsZeroer interface hook.
func isZeroValue(v reflect.Value) bool {
	kind := v.Kind()
	if z, ok := v.Interface().(yaml4.IsZeroer); ok {
		if (kind == reflect.Pointer || kind == reflect.Interface) && v.IsNil() {
			return true
		}
		return z.IsZero()
	}

	switch kind {
	case reflect.String:
		return v.String() == ""
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Struct:
		vt := v.Type()
		for i := v.NumField() - 1; i >= 0; i-- {
			if vt.Field(i).PkgPath != "" {
				continue // unexported field
			}
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	}

	return false
}

// relativeIncludePath returns the include path to emit for entry: the path as
// written in the source, or a path relative to cfg.RelativeTo when configured.
func relativeIncludePath(cfg *DumpOpts, entry includeEntry) string {
	if cfg.RelativeTo != "" {
		if rel, err := filepath.Rel(cfg.RelativeTo, entry.AbsolutePath); err == nil {
			return rel
		}
	}

	return entry.RelativePath
}

// buildOutputStruct converts a struct into an ordered mapping, replacing
// tracked include subtrees with !include markers pointing to their source
// files. Field order follows the Go struct, omitempty drops zero-valued
// fields and a nil pointer field is emitted as null — matching the plain
// [Dump] shape apart from the markers. Flow-style, inline and embedded
// (anonymous) fields are not special-cased: they are emitted as regular
// block fields. Paths are looked up in session — the include data bound to
// the dumped object; a nil session matches nothing, so every value is
// emitted inline.
func buildOutputStruct(v reflect.Value, path string, cfg *DumpOpts, session *includeSession) any {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return v.Interface()
	}

	// A root !include owns the whole document: the output is just the marker.
	if path == "" {
		if entry, ok := session.entry(""); ok {
			return includeTag{Path: relativeIncludePath(cfg, entry)}
		}
	}

	t := v.Type()
	out := orderedMap{
		keys: make([]string, 0, t.NumField()),
		vals: make([]any, 0, t.NumField()),
	}
	add := func(name string, value any) {
		out.keys = append(out.keys, name)
		out.vals = append(out.vals, value)
	}

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		yamlName, omitEmpty := yamlFieldTag(&field)
		if yamlName == "" || yamlName == "-" {
			continue
		}

		nextPath := yamlName
		if path != "" {
			nextPath = path + "." + yamlName
		}

		fieldValue := v.Field(i)
		if omitEmpty && isZeroValue(fieldValue) {
			continue
		}

		if entry, ok := session.entry(nextPath); ok {
			add(yamlName, includeTag{Path: relativeIncludePath(cfg, entry)})
			continue
		}

		if fieldValue.Kind() == reflect.Pointer && fieldValue.IsNil() {
			add(yamlName, fieldValue.Interface()) // a nil pointer renders as null
			continue
		}

		if fieldValue.Kind() == reflect.Struct || (fieldValue.Kind() == reflect.Pointer && fieldValue.Elem().Kind() == reflect.Struct) {
			add(yamlName, buildOutputStruct(fieldValue, nextPath, cfg, session))
		} else {
			add(yamlName, fieldValue.Interface())
		}
	}

	return out
}

// trackMissingDirs remembers every directory level of dir that does not exist
// yet, shallowest first, so a rollback can remove the directories created by
// the following MkdirAll deepest first. Levels whose existence cannot be
// determined are not recorded.
func trackMissingDirs(dir string, created *[]string) {
	var missing []string
	for d := dir; ; {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			break
		}
		missing = append(missing, d)
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	for i := len(missing) - 1; i >= 0; i-- {
		*created = append(*created, missing[i])
	}
}
