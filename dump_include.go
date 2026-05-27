package yaml

import (
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
// It writes included subtrees to their original files using atomic writes and replaces
// them with !include tags in the output. If any write fails, no files are modified.
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
		return nil, fmt.Errorf("DumpWithInclude: expected struct pointer, got %s", v.Kind())
	}

	var includes []includeInfo
	collectIncludesRecursive(v, "", &includes)

	type pendingWrite struct {
		TempPath  string
		FinalPath string
	}
	var pendingWrites []pendingWrite
	var tempFiles []string

	cleanupTemps := func() {
		for _, tmp := range tempFiles {
			_ = os.Remove(tmp)
		}
	}

	for _, inc := range includes {
		if inc.FilePath == "" {
			continue
		}
		dir := filepath.Dir(inc.FilePath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			cleanupTemps()
			return nil, fmt.Errorf("failed to create dir %s: %w", dir, err)
		}

		base := filepath.Base(inc.FilePath)
		ext := filepath.Ext(base)
		name := "." + strings.TrimSuffix(base, ext) + ext + ".tmp"

		tmpFile, err := os.CreateTemp(dir, name)
		if err != nil {
			cleanupTemps()
			return nil, fmt.Errorf("failed to create temp file for %s: %w", inc.FilePath, err)
		}
		tmpPath := tmpFile.Name()
		tempFiles = append(tempFiles, tmpPath)

		data, err := yaml4.Marshal(inc.Value)
		if err != nil {
			_ = tmpFile.Close()
			cleanupTemps()
			return nil, fmt.Errorf("failed to marshal include %s: %w", inc.YamlPath, err)
		}
		if _, err := tmpFile.Write(data); err != nil {
			_ = tmpFile.Close()
			cleanupTemps()
			return nil, fmt.Errorf("failed to write temp file %s: %w", tmpPath, err)
		}
		if err := tmpFile.Close(); err != nil {
			cleanupTemps()
			return nil, fmt.Errorf("failed to close temp file %s: %w", tmpPath, err)
		}

		if err := os.Chmod(tmpPath, 0o644); err != nil {
			cleanupTemps()
			return nil, fmt.Errorf("failed to set permissions on %s: %w", tmpPath, err)
		}

		pendingWrites = append(pendingWrites, pendingWrite{
			TempPath:  tmpPath,
			FinalPath: inc.FilePath,
		})
	}

	outMap := buildOutputStruct(v, "", cfg)
	out, err := yaml4.Marshal(outMap)
	if err != nil {
		cleanupTemps()
		return nil, fmt.Errorf("failed to marshal main config: %w", err)
	}

	for _, pw := range pendingWrites {
		if err := os.Rename(pw.TempPath, pw.FinalPath); err != nil {
			cleanupTemps()
			return nil, fmt.Errorf("failed to atomically write %s: %w", pw.FinalPath, err)
		}
	}

	return out, nil
}

// MarshalWithInclude is a compatibility proxy for DumpWithInclude.
// It aligns the naming convention with the standard yaml.Marshal function.
func MarshalWithInclude(in any, opts ...DumpOption) ([]byte, error) {
	return DumpWithInclude(in, opts...)
}

func collectIncludesRecursive(v reflect.Value, path string, out *[]includeInfo) {
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

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		yamlName := field.Tag.Get("yaml")
		if yamlName == "" {
			yamlName = strings.ToLower(field.Name)
		} else {
			yamlName = strings.Split(yamlName, ",")[0]
		}
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

		if absPath := FindIncludeFileAbs(nextPath); absPath != "" {
			*out = append(*out, includeInfo{
				YamlPath: nextPath,
				Value:    fieldValue.Interface(),
				FilePath: absPath,
			})
		}

		if fieldValue.Kind() == reflect.Struct || (fieldValue.Kind() == reflect.Pointer && fieldValue.Elem().Kind() == reflect.Struct) {
			collectIncludesRecursive(fieldValue, nextPath, out)
		}
	}
}

func buildOutputStruct(v reflect.Value, path string, cfg *DumpOpts) any {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return v.Interface()
	}

	t := v.Type()
	out := make(map[string]any)
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		yamlName := field.Tag.Get("yaml")
		if yamlName == "" {
			yamlName = strings.ToLower(field.Name)
		} else {
			yamlName = strings.Split(yamlName, ",")[0]
		}
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

		if relPath := FindIncludeFile(nextPath); relPath != "" {
			outPath := relPath

			if cfg.RelativeTo != "" {
				absPath := FindIncludeFileAbs(nextPath)
				if absPath != "" {
					if rel, err := filepath.Rel(cfg.RelativeTo, absPath); err == nil {
						outPath = rel
					}
				}
			}
			out[yamlName] = includeTag{Path: outPath}
		} else if fieldValue.Kind() == reflect.Struct || (fieldValue.Kind() == reflect.Pointer && fieldValue.Elem().Kind() == reflect.Struct) {
			nested := buildOutputStruct(fieldValue, nextPath, cfg)
			if nested != nil {
				out[yamlName] = nested
			}
		} else {
			out[yamlName] = fieldValue.Interface()
		}
	}
	return out
}
