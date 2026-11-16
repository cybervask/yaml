package yaml

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yaml4 "go.yaml.in/yaml/v4"
)

// TestDumpWithIncludeRoundTrip verifies the full write-back cycle: a config
// loaded with !include is dumped back with its include files updated atomically.
func TestDumpWithIncludeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	loggingPath := filepath.Join(dir, "logging.yaml")

	if err := os.WriteFile(loggingPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}

	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("server:\n  logging: !include logging.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	cfg.Server.Logging.Level = "trace"

	out, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	if !strings.Contains(string(out), "!include logging.yaml") {
		t.Errorf("expected !include marker in main document, got:\n%s", out)
	}

	data, err := os.ReadFile(loggingPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "level: trace") {
		t.Errorf("expected updated level in include file, got:\n%s", data)
	}
}

// TestDumpWithIncludeRejectsScalars verifies that DumpWithInclude fails on
// non-struct inputs.
func TestDumpWithIncludeRejectsScalars(t *testing.T) {
	if _, err := DumpWithInclude("not-a-struct"); err == nil {
		t.Fatal("expected scalar rejection error, got nil")
	}
}

// TestMarshalWithIncludeProxy verifies that the MarshalWithInclude alias
// delegates to DumpWithInclude. The test is self-contained: it owns its own
// TempDir, its own config with a !include directive, and registers tracker
// entries exclusively through its own LoadFile call.
func TestMarshalWithIncludeProxy(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	t.Chdir(t.TempDir())

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}

	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	if err := os.WriteFile("logging.yaml", []byte("level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("config.yaml", []byte("server:\n  logging: !include logging.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile("config.yaml", &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	cfg.Server.Logging.Level = "trace"

	out, err := MarshalWithInclude(&cfg, WithRelativeIncludes("."))
	if err != nil {
		t.Fatalf("expected successful marshal, got: %v", err)
	}

	if !strings.Contains(string(out), "!include logging.yaml") {
		t.Errorf("expected !include marker in output, got:\n%s", out)
	}

	data, err := os.ReadFile("logging.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "level: trace") {
		t.Errorf("expected updated value in own include file, got:\n%s", data)
	}
}

// TestDumpWithInclude_PreservesFilePermissions verifies that DumpWithInclude
// keeps the permission bits of the include file: a file stored with 0600 must
// remain 0600 after a write-back.
func TestDumpWithInclude_PreservesFilePermissions(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	loggingPath := filepath.Join(dir, "logging.yaml")
	if err := os.WriteFile(loggingPath, []byte("level: info\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}

	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("server:\n  logging: !include logging.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	cfg.Server.Logging.Level = "trace"

	if _, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir)); err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	info, err := os.Stat(loggingPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("expected include file permissions 0600 after dump, got %04o", got)
	}
}

// TestDumpWithInclude_MissingFileDefaultPerms verifies that an include target
// absent at dump time (e.g. removed after load) is recreated with the default
// 0644 permissions.
func TestDumpWithInclude_MissingFileDefaultPerms(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	loggingPath := filepath.Join(dir, "logging.yaml")
	if err := os.WriteFile(loggingPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}

	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("server:\n  logging: !include logging.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if err := os.Remove(loggingPath); err != nil {
		t.Fatal(err)
	}

	cfg.Server.Logging.Level = "trace"

	out, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump for missing include file, got error: %v", err)
	}
	if !strings.Contains(string(out), "!include logging.yaml") {
		t.Errorf("expected !include marker in main document, got:\n%s", out)
	}

	info, err := os.Stat(loggingPath)
	if err != nil {
		t.Fatalf("expected include file to be recreated, got error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("expected recreated include file permissions 0644, got %04o", got)
	}

	data, err := os.ReadFile(loggingPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "level: trace") {
		t.Errorf("expected updated value in recreated include file, got:\n%s", data)
	}
}

// TestDumpWithInclude_AllOrNothingOnFailure verifies the all-or-nothing
// guarantee: when one include target cannot be written, the dump fails and no
// other include file is left modified.
func TestDumpWithInclude_AllOrNothingOnFailure(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")

	if err := os.WriteFile(aPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("host: localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Database struct {
		Host string `yaml:"host" default:"localhost" description:"DB host"`
	}
	type Config struct {
		Server struct {
			Logging  Logging  `yaml:"logging"`
			Database Database `yaml:"database"`
		} `yaml:"server"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("server:\n  logging: !include a.yaml\n  database: !include b.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	origA, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}

	// Make the second include target impossible to rename over: a directory
	// occupying its path.
	if err := os.Remove(bPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bPath, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg.Server.Logging.Level = "trace"
	cfg.Server.Database.Host = "db.example"

	if _, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir)); err == nil {
		t.Fatal("expected dump to fail when an include target is a directory, got nil")
	}

	curA, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(origA, curA) {
		t.Errorf("first include file modified despite failed dump: original %q, got %q", origA, curA)
	}

	info, err := os.Stat(bPath)
	if err != nil {
		t.Fatalf("expected second include path to still exist, got error: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected second include path to remain a directory, got mode %v", info.Mode())
	}
}

// TestDumpWithInclude_RollbackRestoresBytes verifies that after a failed dump
// the previously written include file is restored byte-for-byte and no
// backup/temp artifacts remain in the include directory.
func TestDumpWithInclude_RollbackRestoresBytes(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")

	if err := os.WriteFile(aPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("host: localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Database struct {
		Host string `yaml:"host" default:"localhost" description:"DB host"`
	}
	type Config struct {
		Server struct {
			Logging  Logging  `yaml:"logging"`
			Database Database `yaml:"database"`
		} `yaml:"server"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("server:\n  logging: !include a.yaml\n  database: !include b.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	origA, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(bPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bPath, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg.Server.Logging.Level = "trace"
	cfg.Server.Database.Host = "db.example"

	if _, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir)); err == nil {
		t.Fatal("expected dump to fail when an include target is a directory, got nil")
	}

	curA, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(origA, curA) {
		t.Errorf("include file content not restored after rollback: original %q, got %q", origA, curA)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if name != "config.yaml" && name != "a.yaml" && name != "b.yaml" {
			t.Errorf("rollback left artifact in include directory: %s", name)
		}
	}
}

// TestResolveIncludes_NestedRootIncludeKeepsOuterFile verifies that when the
// root of an included file is itself a !include directive, the tracker keeps
// the outer registration: the main document keeps referencing the outer file,
// write-back updates the outer file, and the innermost file stays untouched.
func TestResolveIncludes_NestedRootIncludeKeepsOuterFile(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")

	if err := os.WriteFile(bPath, []byte("level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aPath, []byte("!include b.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("server:\n  logging: !include a.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Server.Logging.Level != "debug" {
		t.Errorf("expected nested include content, got level %q", cfg.Server.Logging.Level)
	}

	cfg.Server.Logging.Level = "trace"

	out, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	if !strings.Contains(string(out), "!include a.yaml") {
		t.Errorf("expected main document to reference a.yaml, got:\n%s", out)
	}

	aData, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aData), "level: trace") {
		t.Errorf("expected updated value written to a.yaml, got:\n%s", aData)
	}

	bData, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bData), "level: debug") || strings.Contains(string(bData), "trace") {
		t.Errorf("expected b.yaml to stay untouched, got:\n%s", bData)
	}
}

// TestRegisterIncludePath_FirstWins verifies that re-registering an already
// tracked YAML path does not overwrite the existing entry: the registration
// closest to the main config wins, nested root includes keep it.
func TestRegisterIncludePath_FirstWins(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	aAbs := filepath.Join(dir, "a.yaml")
	bAbs := filepath.Join(dir, "b.yaml")
	cAbs := filepath.Join(dir, "c.yaml")

	RegisterIncludePath("server.logging", "a.yaml", aAbs)
	// A nested root !include re-registers the same YAML path; it must not
	// overwrite the outer registration.
	RegisterIncludePath("server.logging", "b.yaml", bAbs)

	if rel := FindIncludeFile("server.logging"); rel != "a.yaml" {
		t.Errorf("expected first-wins relative path a.yaml, got %q", rel)
	}
	if abs := FindIncludeFileAbs("server.logging"); abs != aAbs {
		t.Errorf("expected first-wins absolute path %s, got %s", aAbs, abs)
	}

	// Distinct YAML paths are still registered as usual.
	RegisterIncludePath("server.logging.level", "c.yaml", cAbs)
	if rel := FindIncludeFile("server.logging.level"); rel != "c.yaml" {
		t.Errorf("expected nested field path to register normally, got %q", rel)
	}
	if abs := FindIncludeFileAbs("server.logging.level"); abs != cAbs {
		t.Errorf("expected nested field path absolute %s, got %s", cAbs, abs)
	}
}

// TestDumpPreservesIncludeTracker verifies that a plain Dump call — even for a
// struct without include tags — does not wipe the include registry filled by
// LoadFile, so a subsequent DumpWithInclude keeps its !include markers and
// updates the include files.
func TestDumpPreservesIncludeTracker(t *testing.T) {
	dir := t.TempDir()
	loggingPath := filepath.Join(dir, "logging.yaml")
	if err := os.WriteFile(loggingPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level"`
	}
	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}
	type Plain struct {
		Name string `yaml:"name"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("server:\n  logging: !include logging.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	cfg.Server.Logging.Level = "trace"

	// A plain dump of an unrelated structure must leave the registry intact.
	if _, err := Dump(Plain{Name: "unrelated"}); err != nil {
		t.Fatalf("expected plain dump to succeed, got: %v", err)
	}

	if abs := FindIncludeFileAbs("server.logging"); abs == "" {
		t.Fatal("expected the include registry to survive a plain Dump call")
	}

	out, err := DumpWithInclude(&cfg)
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}
	if !strings.Contains(string(out), "!include logging.yaml") {
		t.Errorf("expected !include marker after a plain Dump, got:\n%s", out)
	}

	data, err := os.ReadFile(loggingPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "level: trace") {
		t.Errorf("expected updated level in include file, got:\n%s", data)
	}
}

// TestDumpWithInclude_BackupCollisionSafe verifies that DumpWithInclude never
// touches a pre-existing "<target>.bak" file belonging to the user: backups
// are made under unique temporary names in the target's directory.
func TestDumpWithInclude_BackupCollisionSafe(t *testing.T) {
	dir := t.TempDir()
	loggingPath := filepath.Join(dir, "logging.yaml")
	if err := os.WriteFile(loggingPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bakPath := loggingPath + ".bak"
	if err := os.WriteFile(bakPath, []byte("PRECIOUS USER BACKUP\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level"`
	}
	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("server:\n  logging: !include logging.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	cfg.Server.Logging.Level = "trace"

	if _, err := DumpWithInclude(&cfg); err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	data, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("expected the user's .bak file to survive the dump, got error: %v", err)
	}
	if string(data) != "PRECIOUS USER BACKUP\n" {
		t.Errorf("expected the user's .bak content to be preserved, got:\n%s", data)
	}

	updated, err := os.ReadFile(loggingPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "level: trace") {
		t.Errorf("expected updated level in include file, got:\n%s", updated)
	}
}

// TestDumpWithInclude_SessionIsolationAcrossLoads verifies that DumpWithInclude
// writes back only to the include files recorded while THIS object was loaded:
// dumping an object loaded before the most recent Load must not touch the
// include file of another config.
func TestDumpWithInclude_SessionIsolationAcrossLoads(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")

	if err := os.WriteFile(aPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	mainA := filepath.Join(dir, "a_main.yaml")
	if err := os.WriteFile(mainA, []byte("server:\n  logging: !include a.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainB := filepath.Join(dir, "b_main.yaml")
	if err := os.WriteFile(mainB, []byte("server:\n  logging: !include b.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfgA, cfgB Config
	if err := LoadFile(mainA, &cfgA); err != nil {
		t.Fatalf("load A: %v", err)
	}
	if err := LoadFile(mainB, &cfgB); err != nil {
		t.Fatalf("load B: %v", err)
	}

	// cfgA was loaded before the most recent Load: its write-back must go to
	// its own include file, never to the file recorded by the latest load.
	cfgA.Server.Logging.Level = "trace"

	out, err := DumpWithInclude(&cfgA, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	if !strings.Contains(string(out), "!include a.yaml") {
		t.Errorf("expected !include marker for a.yaml in main document, got:\n%s", out)
	}

	aData, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aData), "level: trace") {
		t.Errorf("expected a.yaml to receive the updated value, got:\n%s", aData)
	}

	bData, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bData), "level: debug") || strings.Contains(string(bData), "trace") {
		t.Errorf("expected b.yaml to stay untouched, got:\n%s", bData)
	}
}

// TestDumpWithInclude_UnloadedObjectNoWrites verifies the safe mode: a struct
// that never went through Load/LoadFile is dumped inline — no !include markers
// are synthesized and no include files are written, even when the global
// tracker holds entries from an unrelated load.
func TestDumpWithInclude_UnloadedObjectNoWrites(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	bPath := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(bPath, []byte("level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	mainB := filepath.Join(dir, "b_main.yaml")
	if err := os.WriteFile(mainB, []byte("server:\n  logging: !include b.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var loaded Config
	if err := LoadFile(mainB, &loaded); err != nil {
		t.Fatalf("load: %v", err)
	}

	// An object of the same shape that never went through a load.
	var unloaded Config
	unloaded.Server.Logging.Level = "from-unloaded"

	out, err := DumpWithInclude(&unloaded, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected inline dump of unloaded object, got error: %v", err)
	}

	if strings.Contains(string(out), "!include") {
		t.Errorf("expected no !include markers for unloaded object, got:\n%s", out)
	}
	if !strings.Contains(string(out), "from-unloaded") {
		t.Errorf("expected the value inline in the main document, got:\n%s", out)
	}

	bData, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bData), "level: debug") || strings.Contains(string(bData), "from-unloaded") {
		t.Errorf("expected b.yaml to stay untouched, got:\n%s", bData)
	}
}

// TestDumpWithInclude_StaleTrackerKeepsWriteBack verifies the second failure
// mode of the global tracker: after another load registers unrelated paths,
// the dump of a previously loaded object must still write back through the
// object's own include data instead of silently inlining the includes.
func TestDumpWithInclude_StaleTrackerKeepsWriteBack(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	loggingPath := filepath.Join(dir, "logging.yaml")
	dbPath := filepath.Join(dir, "db.yaml")
	if err := os.WriteFile(loggingPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, []byte("host: localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Database struct {
		Host string `yaml:"host" default:"localhost" description:"DB host"`
	}
	type ConfigA struct {
		Server struct {
			Logging  Logging  `yaml:"logging"`
			Database Database `yaml:"database"`
		} `yaml:"server"`
	}
	type ConfigB struct {
		App struct {
			Host string `yaml:"host" default:"web.example" description:"App host"`
		} `yaml:"app"`
	}

	mainA := filepath.Join(dir, "a_main.yaml")
	if err := os.WriteFile(mainA, []byte("server:\n  logging: !include logging.yaml\n  database: !include db.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainB := filepath.Join(dir, "b_main.yaml")
	if err := os.WriteFile(mainB, []byte("app:\n  host: !include host.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "host.yaml"), []byte("web.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfgA ConfigA
	if err := LoadFile(mainA, &cfgA); err != nil {
		t.Fatalf("load A: %v", err)
	}
	// A later load of an unrelated config shares no include paths with A.
	var cfgB ConfigB
	if err := LoadFile(mainB, &cfgB); err != nil {
		t.Fatalf("load B: %v", err)
	}

	cfgA.Server.Logging.Level = "trace"
	cfgA.Server.Database.Host = "db.example"

	out, err := DumpWithInclude(&cfgA, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	if !strings.Contains(string(out), "!include logging.yaml") || !strings.Contains(string(out), "!include db.yaml") {
		t.Errorf("expected !include markers for both paths in main document, got:\n%s", out)
	}

	loggingData, err := os.ReadFile(loggingPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(loggingData), "level: trace") {
		t.Errorf("expected logging.yaml to receive the updated value, got:\n%s", loggingData)
	}

	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dbData), "db.example") {
		t.Errorf("expected db.yaml to receive the updated value, got:\n%s", dbData)
	}
}

// TestDumpWithInclude_SessionReplacedOnReload verifies that re-loading the
// same pointer replaces its include data: the dump reflects the most recent
// load, not the original one.
func TestDumpWithInclude_SessionReplacedOnReload(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.yaml")
	cPath := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(aPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cPath, []byte("level: warn\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	mainFirst := filepath.Join(dir, "first.yaml")
	if err := os.WriteFile(mainFirst, []byte("server:\n  logging: !include a.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainSecond := filepath.Join(dir, "second.yaml")
	if err := os.WriteFile(mainSecond, []byte("server:\n  logging: !include c.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(mainFirst, &cfg); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if err := LoadFile(mainSecond, &cfg); err != nil {
		t.Fatalf("second load: %v", err)
	}

	cfg.Server.Logging.Level = "trace"

	out, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	if !strings.Contains(string(out), "!include c.yaml") {
		t.Errorf("expected !include marker for c.yaml after reload, got:\n%s", out)
	}

	cData, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cData), "level: trace") {
		t.Errorf("expected c.yaml to receive the updated value, got:\n%s", cData)
	}

	aData, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aData), "level: info") || strings.Contains(string(aData), "trace") {
		t.Errorf("expected a.yaml to stay untouched after reload, got:\n%s", aData)
	}
}

// diamondConfigLayout writes the shared diamond fixture: two top-level fields
// loading the same include file, plus the main config that references it.
func diamondConfigLayout(t *testing.T, dir string) (mainPath, sharedPath string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, "shared.yaml"), []byte("value: 42\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mainPath = filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte("left: !include shared.yaml\nright: !include shared.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return mainPath, filepath.Join(dir, "shared.yaml")
}

// TestDumpWithInclude_DiamondInclude verifies that two fields loaded from the
// same include file (a diamond) cannot be dumped with conflicting values: the
// second write-back used to silently overwrite the first, losing one field's
// change. A conflicting dump must fail with an explicit error.
func TestDumpWithInclude_DiamondInclude(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	_, sharedPath := diamondConfigLayout(t, dir)

	type Shared struct {
		Value int `yaml:"value" description:"Shared value"`
	}
	type Config struct {
		Left  Shared `yaml:"left" description:"Left branch"`
		Right Shared `yaml:"right" description:"Right branch"`
	}

	var cfg Config
	if err := LoadFile(filepath.Join(dir, "config.yaml"), &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	cfg.Left.Value = 100
	cfg.Right.Value = 200

	_, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err == nil {
		t.Fatal("expected conflicting diamond values to be rejected, got nil")
	}

	if !strings.Contains(err.Error(), "conflicting values for include target") {
		t.Errorf("expected conflict error naming the target, got: %v", err)
	}
	if !strings.Contains(err.Error(), "yaml paths left and right") {
		t.Errorf("expected both yaml paths in conflict error, got: %v", err)
	}

	// The conflicting dump must not have written either value out.
	data, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "100") || strings.Contains(string(data), "200") {
		t.Errorf("expected include file to stay untouched on conflict, got:\n%s", data)
	}
}

// TestDumpWithInclude_DiamondConflict verifies that a conflicting diamond dump
// fails cleanly: the include file keeps its original bytes, the main config
// file stays untouched, and no temporary or backup artifacts are left behind —
// the conflict is detected before any filesystem action is taken.
func TestDumpWithInclude_DiamondConflict(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	mainPath, sharedPath := diamondConfigLayout(t, dir)

	type Shared struct {
		Value int `yaml:"value" description:"Shared value"`
	}
	type Config struct {
		Left  Shared `yaml:"left" description:"Left branch"`
		Right Shared `yaml:"right" description:"Right branch"`
	}

	var cfg Config
	if err := LoadFile(mainPath, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	origShared, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatal(err)
	}
	origMain, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Left.Value = 100
	cfg.Right.Value = 200

	if _, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir)); err == nil {
		t.Fatal("expected conflicting diamond values to be rejected, got nil")
	}

	curShared, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(origShared, curShared) {
		t.Errorf("include file modified despite conflict: original %q, got %q", origShared, curShared)
	}

	curMain, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(origMain, curMain) {
		t.Errorf("main config modified despite conflict: original %q, got %q", origMain, curMain)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if name := e.Name(); name != "config.yaml" && name != "shared.yaml" {
			t.Errorf("conflict dump left an artifact in the include directory: %s", name)
		}
	}
}

// TestDumpWithInclude_DiamondEqualValues verifies that a diamond whose branches
// agree is legal: after an unmodified load the shared target is written once,
// the file re-parses to the loaded value, and both fields are emitted as
// !include markers.
func TestDumpWithInclude_DiamondEqualValues(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	_, sharedPath := diamondConfigLayout(t, dir)

	type Shared struct {
		Value int `yaml:"value" description:"Shared value"`
	}
	type Config struct {
		Left  Shared `yaml:"left" description:"Left branch"`
		Right Shared `yaml:"right" description:"Right branch"`
	}

	var cfg Config
	if err := LoadFile(filepath.Join(dir, "config.yaml"), &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	out, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump for agreeing diamond values, got error: %v", err)
	}

	if !strings.Contains(string(out), "left: !include shared.yaml") || !strings.Contains(string(out), "right: !include shared.yaml") {
		t.Errorf("expected !include markers for both branches, got:\n%s", out)
	}

	data, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatal(err)
	}

	var shared Shared
	if err := yaml4.Unmarshal(data, &shared); err != nil {
		t.Fatalf("expected include file to re-parse, got error: %v\n%s", err, data)
	}
	if shared.Value != 42 {
		t.Errorf("expected value 42 in include file, got %d", shared.Value)
	}
}

// TestDumpWithInclude_NestedIncludeFilesUntouched verifies the scope of the
// write-back: an include file that itself carries !include markers on its
// fields (second include level) is never rewritten — rewriting it from the
// Go value would erase those markers — and the file it references keeps its
// bytes as well. The main document still carries the first-level marker.
func TestDumpWithInclude_NestedIncludeFilesUntouched(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	outerPath := filepath.Join(dir, "outer.yaml")
	innerPath := filepath.Join(dir, "inner.yaml")

	outerOriginal := "level: !include inner.yaml\n"
	innerOriginal := "trace  # original comment\n"
	if err := os.WriteFile(outerPath, []byte(outerOriginal), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(innerPath, []byte(innerOriginal), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level"`
	}
	type Config struct {
		Logging Logging `yaml:"logging"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("logging: !include outer.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}
	if cfg.Logging.Level != "trace" {
		t.Fatalf("expected nested include to resolve, got level %q", cfg.Logging.Level)
	}

	cfg.Logging.Level = "fatal"

	out, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	if !strings.Contains(string(out), "!include outer.yaml") {
		t.Errorf("expected first-level !include marker in main document, got:\n%s", out)
	}
	if strings.Contains(string(out), "fatal") {
		t.Errorf("expected the nested value to stay out of the main document, got:\n%s", out)
	}

	outerData, err := os.ReadFile(outerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(outerData) != outerOriginal {
		t.Errorf("expected outer.yaml untouched byte-for-byte, got:\n%s", outerData)
	}

	innerData, err := os.ReadFile(innerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(innerData) != innerOriginal {
		t.Errorf("expected inner.yaml untouched byte-for-byte, got:\n%s", innerData)
	}
}

// TestDumpWithInclude_MultipleIncludesInOneField verifies that a field
// carrying several !include markers (a sequence of includes) is rejected
// loudly on dump: the write-back replaces a whole field value with a single
// marker, so dumping would silently collapse both markers and orphan the
// second file. The load keeps such configs working, and the failure happens
// before any filesystem action, so no file is touched.
func TestDumpWithInclude_MultipleIncludesInOneField(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(aPath, []byte("name: alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("name: beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Server struct {
		Name string `yaml:"name"`
	}
	type Config struct {
		Servers []Server `yaml:"servers"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("servers: [!include a.yaml, !include b.yaml]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected sequence includes to keep loading, got error: %v", err)
	}
	if len(cfg.Servers) != 2 || cfg.Servers[0].Name != "alpha" || cfg.Servers[1].Name != "beta" {
		t.Fatalf("unexpected servers state: %+v", cfg.Servers)
	}

	origA, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	origB, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err == nil {
		t.Fatal("expected multiple-includes error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple !include entries under one field are not supported") {
		t.Errorf("unexpected error text: %v", err)
	}

	curA, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(origA, curA) {
		t.Errorf("a.yaml modified by the rejected dump: original %q, got %q", origA, curA)
	}

	curB, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(origB, curB) {
		t.Errorf("b.yaml modified by the rejected dump: original %q, got %q", origB, curB)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if name := e.Name(); name != "config.yaml" && name != "a.yaml" && name != "b.yaml" {
			t.Errorf("rejected dump left an artifact in the include directory: %s", name)
		}
	}
}

// TestDumpWithInclude_FieldOrderOmitEmptyAndNull verifies that the rebuilt
// main document matches the plain Dump shape apart from the include markers:
// fields follow struct order instead of an alphabetical map, omitempty drops
// zero-valued fields, and a nil pointer field is emitted as null.
func TestDumpWithInclude_FieldOrderOmitEmptyAndNull(t *testing.T) {
	type Inner struct {
		X string `yaml:"x"`
	}
	type Config struct {
		Zebra   string `yaml:"zebra"`
		Alpha   string `yaml:"alpha"`
		Skipped string `yaml:"skipped,omitempty"`
		Kept    string `yaml:"kept,omitempty"`
		Ptr     *Inner `yaml:"ptr"`
	}

	cfg := Config{Zebra: "z", Alpha: "a", Kept: "k"}

	out, err := DumpWithInclude(&cfg)
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	text := string(out)
	// Struct order: zebra before alpha — an alphabetical map would flip them.
	if i, j := strings.Index(text, "zebra:"), strings.Index(text, "alpha:"); i < 0 || j < 0 || i > j {
		t.Errorf("expected struct field order (zebra before alpha), got:\n%s", text)
	}
	if strings.Contains(text, "skipped:") {
		t.Errorf("expected the empty omitempty field to be dropped, got:\n%s", text)
	}
	if !strings.Contains(text, "kept: k") {
		t.Errorf("expected the non-empty omitempty field to be kept, got:\n%s", text)
	}
	if !strings.Contains(text, "ptr: null") {
		t.Errorf("expected the nil pointer field rendered as null, got:\n%s", text)
	}
}

// TestDumpWithInclude_MarkerKeepsStructOrder verifies that an !include marker
// takes its struct position in the rebuilt document instead of being sorted
// alphabetically among the other keys.
func TestDumpWithInclude_MarkerKeepsStructOrder(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "logging.yaml"), []byte("level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level"`
	}
	type Config struct {
		Zebra   string  `yaml:"zebra"`
		Logging Logging `yaml:"logging"`
		Alpha   string  `yaml:"alpha"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("zebra: z\nlogging: !include logging.yaml\nalpha: a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	out, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	text := string(out)
	if !strings.Contains(text, "logging: !include logging.yaml") {
		t.Errorf("expected the marker for logging, got:\n%s", text)
	}
	if i, j, k := strings.Index(text, "zebra:"), strings.Index(text, "logging:"), strings.Index(text, "alpha:"); i < 0 || j < 0 || k < 0 || i > j || j > k {
		t.Errorf("expected struct order zebra, logging, alpha, got:\n%s", text)
	}
}

// symlinkTargetFixture writes one include file plus a symlink to it and the
// main config loading the same file through both spellings.
func symlinkTargetFixture(t *testing.T, dir string) (mainPath, realPath, linkPath string) {
	t.Helper()

	realPath = filepath.Join(dir, "real.yaml")
	if err := os.WriteFile(realPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath = filepath.Join(dir, "link.yaml")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	mainPath = filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte("left: !include link.yaml\nright: !include real.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return mainPath, realPath, linkPath
}

// TestDumpWithInclude_SymlinkConflict verifies that two fields loaded from
// the same file — reached through a symlink and through its target — cannot
// be dumped with conflicting values: the targets are collapsed by file
// identity, so the disagreement is reported as a conflict error and no file
// is touched.
func TestDumpWithInclude_SymlinkConflict(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	_, realPath, linkPath := symlinkTargetFixture(t, dir)

	type Inner struct {
		Level string `yaml:"level"`
	}
	type Config struct {
		Left  Inner `yaml:"left"`
		Right Inner `yaml:"right"`
	}

	var cfg Config
	if err := LoadFile(filepath.Join(dir, "config.yaml"), &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	cfg.Left.Level = "from-left"
	cfg.Right.Level = "from-right"

	_, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err == nil {
		t.Fatal("expected conflicting values for one file behind two spellings to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "conflicting values for include target") {
		t.Errorf("unexpected error text: %v", err)
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "level: info") {
		t.Errorf("expected the shared file to stay untouched, got:\n%s", data)
	}

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected link.yaml to remain a symlink after the rejected dump")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if name := e.Name(); name != "config.yaml" && name != "real.yaml" && name != "link.yaml" {
			t.Errorf("rejected dump left an artifact in the include directory: %s", name)
		}
	}
}

// TestDumpWithInclude_SymlinkEqualPayloads verifies that agreeing values for
// one file behind two spellings produce a single write-back, aimed at the
// symlink-resolved target: the real file is updated and the symlink itself
// survives the dump.
func TestDumpWithInclude_SymlinkEqualPayloads(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	mainPath, realPath, linkPath := symlinkTargetFixture(t, dir)

	type Inner struct {
		Level string `yaml:"level"`
	}
	type Config struct {
		Left  Inner `yaml:"left"`
		Right Inner `yaml:"right"`
	}

	var cfg Config
	if err := LoadFile(mainPath, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	cfg.Left.Level = "trace"
	cfg.Right.Level = "trace"

	out, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}
	if !strings.Contains(string(out), "!include link.yaml") || !strings.Contains(string(out), "!include real.yaml") {
		t.Errorf("expected markers for both spellings, got:\n%s", out)
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "level: trace") {
		t.Errorf("expected the shared file to receive the updated value, got:\n%s", data)
	}

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected link.yaml to remain a symlink after the dump")
	}

	resolved, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resolved), "level: trace") {
		t.Errorf("expected the symlink to resolve to the updated content, got:\n%s", resolved)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if name := e.Name(); name != "config.yaml" && name != "real.yaml" && name != "link.yaml" {
			t.Errorf("dump left an artifact in the include directory: %s", name)
		}
	}
}

// TestDumpWithInclude_RollbackRemovesCreatedDirs verifies that a failed dump
// removes the directory level it created for a missing include target and
// leaves no directories or other artifacts behind.
func TestDumpWithInclude_RollbackRemovesCreatedDirs(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	aPath := filepath.Join(sub, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")

	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aPath, []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("host: localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level"`
	}
	type Database struct {
		Host string `yaml:"host"`
	}
	type Config struct {
		Logging  Logging  `yaml:"logging"`
		Database Database `yaml:"database"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("logging: !include sub/a.yaml\ndatabase: !include b.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	// The first target's directory is removed after the load, so the dump
	// has to recreate it with MkdirAll. The second target is turned into a
	// directory so the dump fails after the temp file for the first target
	// has been created.
	if err := os.RemoveAll(sub); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bPath, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg.Logging.Level = "trace"
	cfg.Database.Host = "db.example"

	if _, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir)); err == nil {
		t.Fatal("expected dump to fail when an include target is a directory, got nil")
	}

	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Errorf("expected the directory created by the dump to be removed on rollback, got stat error: %v", err)
	}

	if _, err := os.Stat(aPath); !os.IsNotExist(err) {
		t.Errorf("expected no include file to be written by the failed dump, got stat error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if name := e.Name(); name != "config.yaml" && name != "b.yaml" {
			t.Errorf("rollback left an artifact in the include directory: %s", name)
		}
	}
}

// TestDumpWithInclude_RootIncludeWriteBack verifies that a !include marker
// occupying the whole main document is tracked and written back like a field
// include: the include file receives the dumped value and the main document
// becomes just the marker line.
func TestDumpWithInclude_RootIncludeWriteBack(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()
	outerPath := filepath.Join(dir, "outer.yaml")
	if err := os.WriteFile(outerPath, []byte("level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Config struct {
		Level string `yaml:"level"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("!include outer.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}
	if cfg.Level != "debug" {
		t.Fatalf("expected the root include to resolve, got level %q", cfg.Level)
	}

	cfg.Level = "trace"

	out, err := DumpWithInclude(&cfg, WithRelativeIncludes(dir))
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	if got := strings.TrimSpace(string(out)); got != "!include outer.yaml" {
		t.Errorf("expected the main document to be just the root marker, got:\n%s", out)
	}

	data, err := os.ReadFile(outerPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "level: trace") {
		t.Errorf("expected outer.yaml to receive the updated value, got:\n%s", data)
	}
}

// TestDumpWithInclude_NonStructRejectionMessage verifies that the rejection
// of non-struct inputs names every accepted shape: the function takes a
// struct or a struct pointer, and a bare struct value is dumped successfully.
func TestDumpWithInclude_NonStructRejectionMessage(t *testing.T) {
	_, err := DumpWithInclude("not-a-struct")
	if err == nil {
		t.Fatal("expected scalar rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "expected struct or struct pointer") {
		t.Errorf("expected the accepted shapes to be named exactly, got: %v", err)
	}

	value := struct {
		Level string `yaml:"level"`
	}{Level: "trace"}
	if _, err := DumpWithInclude(value); err != nil {
		t.Errorf("expected a struct value to be accepted, got: %v", err)
	}
}

// TestDumpWithInclude_StreamLoadDropsPriorSession pins the session hygiene of
// the stream load paths: a successful Decoder.Decode of
// a pointer previously loaded via LoadFile drops the session recorded by that
// load — the stream replaced the object's content, so the earlier binding no
// longer reflects it. A later dump therefore runs in the inline safe mode
// instead of writing values decoded from the stream into the files recorded
// by the earlier load (a silent cross-file value transfer before the fix).
func TestDumpWithInclude_StreamLoadDropsPriorSession(t *testing.T) {
	dir := t.TempDir()

	loggingA := filepath.Join(dir, "loggingA.yaml")
	loggingB := filepath.Join(dir, "loggingB.yaml")
	aOriginal := "level: A-ORIGINAL\n"
	bOriginal := "level: B-VALUE\n"
	if err := os.WriteFile(loggingA, []byte(aOriginal), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(loggingB, []byte(bOriginal), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level"`
	}
	type Config struct {
		Logging Logging `yaml:"logging"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("logging: !include "+loggingA+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}
	if cfg.Logging.Level != "A-ORIGINAL" {
		t.Fatalf("expected the first load to resolve loggingA, got %q", cfg.Logging.Level)
	}

	stream := strings.NewReader("logging: !include " + loggingB + "\n")
	if err := NewDecoder(stream).Decode(&cfg); err != nil {
		t.Fatalf("expected valid stream decode, got error: %v", err)
	}
	if cfg.Logging.Level != "B-VALUE" {
		t.Fatalf("expected the stream decode to resolve loggingB, got %q", cfg.Logging.Level)
	}

	out, err := DumpWithInclude(&cfg)
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	if strings.Contains(string(out), "!include") {
		t.Errorf("expected an inline dump after the dropped session, got:\n%s", out)
	}

	aData, err := os.ReadFile(loggingA)
	if err != nil {
		t.Fatal(err)
	}
	if string(aData) != aOriginal {
		t.Errorf("expected loggingA.yaml untouched byte-for-byte, got:\n%s", aData)
	}

	bData, err := os.ReadFile(loggingB)
	if err != nil {
		t.Fatal(err)
	}
	if string(bData) != bOriginal {
		t.Errorf("expected loggingB.yaml untouched byte-for-byte, got:\n%s", bData)
	}
}

// TestDumpWithInclude_LoaderLoadDropsPriorSession mirrors
// TestDumpWithInclude_StreamLoadDropsPriorSession for the other stream path.
func TestDumpWithInclude_LoaderLoadDropsPriorSession(t *testing.T) {
	dir := t.TempDir()

	loggingA := filepath.Join(dir, "loggingA.yaml")
	loggingB := filepath.Join(dir, "loggingB.yaml")
	aOriginal := "level: A-ORIGINAL\n"
	bOriginal := "level: B-VALUE\n"
	if err := os.WriteFile(loggingA, []byte(aOriginal), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(loggingB, []byte(bOriginal), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level"`
	}
	type Config struct {
		Logging Logging `yaml:"logging"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("logging: !include "+loggingA+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	loader, lerr := NewLoader(strings.NewReader("logging: !include " + loggingB + "\n"))
	if lerr != nil {
		t.Fatalf("expected valid loader, got error: %v", lerr)
	}
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("expected valid stream load, got error: %v", err)
	}
	if cfg.Logging.Level != "B-VALUE" {
		t.Fatalf("expected the stream load to resolve loggingB, got %q", cfg.Logging.Level)
	}

	out, err := DumpWithInclude(&cfg)
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}

	if strings.Contains(string(out), "!include") {
		t.Errorf("expected an inline dump after the dropped session, got:\n%s", out)
	}

	aData, err := os.ReadFile(loggingA)
	if err != nil {
		t.Fatal(err)
	}
	if string(aData) != aOriginal {
		t.Errorf("expected loggingA.yaml untouched byte-for-byte, got:\n%s", aData)
	}

	bData, err := os.ReadFile(loggingB)
	if err != nil {
		t.Fatal(err)
	}
	if string(bData) != bOriginal {
		t.Errorf("expected loggingB.yaml untouched byte-for-byte, got:\n%s", bData)
	}
}

// TestDumpWithInclude_SessionSurvivesFailedStreamLoad pins the error
// counterpart of the drop contract: a failed stream load leaves the object
// unchanged, so the session recorded by the earlier one-shot load survives
// and the dump still writes back to its own file.
func TestDumpWithInclude_SessionSurvivesFailedStreamLoad(t *testing.T) {
	dir := t.TempDir()

	loggingA := filepath.Join(dir, "loggingA.yaml")
	aOriginal := "level: A-ORIGINAL\n"
	if err := os.WriteFile(loggingA, []byte(aOriginal), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level"`
	}
	type Config struct {
		Logging Logging `yaml:"logging"`
	}

	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("logging: !include "+loggingA+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	// A stream parse failure must leave both the object and its session as is.
	if err := NewDecoder(strings.NewReader("logging: [unclosed\n")).Decode(&cfg); err == nil {
		t.Fatal("expected a stream parse error, got nil")
	}

	cfg.Logging.Level = "CHANGED"

	out, err := DumpWithInclude(&cfg)
	if err != nil {
		t.Fatalf("expected valid dump, got error: %v", err)
	}
	if !strings.Contains(string(out), "!include "+loggingA) {
		t.Errorf("expected the surviving session to carry the marker, got:\n%s", out)
	}

	aData, err := os.ReadFile(loggingA)
	if err != nil {
		t.Fatal(err)
	}
	if string(aData) != "level: CHANGED\n" {
		t.Errorf("expected loggingA.yaml to receive the write-back, got:\n%s", aData)
	}
}
