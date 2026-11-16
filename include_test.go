package yaml

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile creates a file with the given content inside a test directory.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

// TestLoadWithInclude verifies that a !include directive pulls the referenced
// file content into the configuration tree and records it in the tracker.
func TestLoadWithInclude(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "logging.yaml", "level: debug\ncolors: true\n")

	type Logging struct {
		Level  string `yaml:"level" default:"info" description:"Log level"`
		Colors bool   `yaml:"colors" description:"Colors"`
	}

	type Config struct {
		Logging Logging `yaml:"logging" description:"Logging settings"`
	}

	main := writeTestFile(t, dir, "config.yaml", "logging: !include logging.yaml\n")

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Logging.Level != "debug" || !cfg.Logging.Colors {
		t.Errorf("unexpected logging state: %+v", cfg.Logging)
	}

	if p := FindIncludeFile("logging"); p != "logging.yaml" {
		t.Errorf("expected tracked relative path logging.yaml, got %q", p)
	}
}

// TestLoadWithIncludeMissingFile verifies the error path when the referenced
// include file does not exist.
func TestLoadWithIncludeMissingFile(t *testing.T) {
	type Config struct {
		Level string `yaml:"level" description:"Log level"`
	}

	err := Load([]byte("level: !include missing.yaml\n"), &Config{})
	if err == nil {
		t.Fatal("expected missing include error, got nil")
	}

	if !strings.Contains(err.Error(), "missing.yaml") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestLoadWithIncludeEmptyFile verifies that an include file holding no YAML
// documents is rejected, with a message that names the actual problem: the
// file may well be non-empty and still contain no documents.
func TestLoadWithIncludeEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "empty.yaml", "")

	type Config struct {
		Level string `yaml:"level" description:"Log level"`
	}

	main := writeTestFile(t, dir, "config.yaml", "level: !include empty.yaml\n")

	err := LoadFile(main, &Config{})
	if err == nil {
		t.Fatal("expected empty include error, got nil")
	}

	if !strings.Contains(err.Error(), "contains no documents") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestLoadInclude_CommentsOnlyHasNoDocuments verifies that an include file
// made only of comments and blanks is rejected as well: such a file is not
// empty, it simply holds no YAML documents, so the message must say so.
func TestLoadInclude_CommentsOnlyHasNoDocuments(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "comments.yaml", "# only a comment\n\n")

	type Config struct {
		Level string `yaml:"level" description:"Log level"`
	}

	main := writeTestFile(t, dir, "config.yaml", "level: !include comments.yaml\n")

	err := LoadFile(main, &Config{})
	if err == nil {
		t.Fatal("expected no-documents error, got nil")
	}

	if !strings.Contains(err.Error(), "contains no documents") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestLoadInclude_MultiDocRejected verifies that an include file holding
// several YAML documents is rejected as a configuration error: yaml4 keeps
// only the first document in a Node, so the pre-fix code silently dropped
// every document after the first one.
func TestLoadInclude_MultiDocRejected(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "logging.yaml", "level: info\n---\nlevel: debug\n")

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Config struct {
		Logging Logging `yaml:"logging" description:"Logging settings"`
	}

	main := writeTestFile(t, dir, "config.yaml", "logging: !include logging.yaml\n")

	var cfg Config
	err := LoadFile(main, &cfg)
	if err == nil {
		t.Fatal("expected multi-document include error, got nil")
	}

	if !strings.Contains(err.Error(), "multiple documents") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestLoadInclude_SingleDocStillWorks guards the streaming rework of the
// include-file reader: a single-document include must keep loading
// exactly as before.
func TestLoadInclude_SingleDocStillWorks(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "logging.yaml", "level: debug\n")

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Config struct {
		Logging Logging `yaml:"logging" description:"Logging settings"`
	}

	main := writeTestFile(t, dir, "config.yaml", "logging: !include logging.yaml\n")

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected included value, got %+v", cfg.Logging)
	}
}

// TestLoadWithNestedInclude verifies that an include file may itself contain
// a !include directive resolved relative to its own location.
func TestLoadWithNestedInclude(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")

	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, sub, "leaf.yaml", "retries: 3\n")

	type Leaf struct {
		Retries int `yaml:"retries" description:"Retry count"`
	}

	type Config struct {
		Leaf Leaf `yaml:"leaf" description:"Leaf settings"`
	}

	main := writeTestFile(t, dir, "config.yaml", "leaf: !include sub/leaf.yaml\n")

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Leaf.Retries != 3 {
		t.Errorf("expected retries=3, got %d", cfg.Leaf.Retries)
	}
}

// TestIncludeTrackerRegistry verifies the register/lookup/reset lifecycle
// of the include tracker.
func TestIncludeTrackerRegistry(t *testing.T) {
	ResetIncludeTracker()

	RegisterIncludePath("a.b", "b.yaml", "/tmp/x/b.yaml")

	if rel := FindIncludeFile("a.b"); rel != "b.yaml" {
		t.Errorf("expected b.yaml, got %q", rel)
	}

	if abs := FindIncludeFileAbs("a.b"); abs != "/tmp/x/b.yaml" {
		t.Errorf("expected /tmp/x/b.yaml, got %q", abs)
	}

	if empty := FindIncludeFile("unknown.path"); empty != "" {
		t.Errorf("expected empty path for unknown key, got %q", empty)
	}

	ResetIncludeTracker()

	if rel := FindIncludeFile("a.b"); rel != "" {
		t.Errorf("expected empty path after reset, got %q", rel)
	}
}

// TestDumpIncludeTagFlagRejected verifies the Dump contract after the removal
// of the dead include-extraction path: Dump is a pure serialization
// entry point, and a field carrying the include tag flag is rejected by the
// underlying marshaller with an error instead of being silently processed.
// Include write-back belongs to DumpWithInclude.
func TestDumpIncludeTagFlagRejected(t *testing.T) {
	type Logging struct {
		Level string `yaml:"level" description:"Log level"`
	}
	type Config struct {
		Logging Logging `yaml:"logging,include"`
	}

	cfg := Config{Logging: Logging{Level: "trace"}}

	out, err := Dump(cfg)
	if err == nil {
		t.Fatalf("expected Dump to fail on an include-tagged field, got output:\n%s", out)
	}
}

// TestLoadWithCyclicInclude verifies that mutually-including files are
// rejected with a cycle error instead of recursing until memory exhaustion.
func TestLoadWithCyclicInclude(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.yaml", "leaf: !include b.yaml\n")
	writeTestFile(t, dir, "b.yaml", "leaf: !include a.yaml\n")

	type Config struct {
		Leaf string `yaml:"leaf" description:"Leaf value"`
	}

	main := writeTestFile(t, dir, "config.yaml", "leaf: !include a.yaml\n")

	var cfg Config
	err := LoadFile(main, &cfg)
	if err == nil {
		t.Fatal("expected cyclic include error, got nil")
	}

	if !strings.Contains(err.Error(), "cyclic !include detected") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestLoadWithSelfInclude verifies that a file including itself is rejected.
func TestLoadWithSelfInclude(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "self.yaml", "leaf: !include self.yaml\n")

	type Config struct {
		Leaf string `yaml:"leaf" description:"Leaf value"`
	}

	var cfg Config
	err := Load([]byte("leaf: !include "+filepath.Join(dir, "self.yaml")+"\n"), &cfg)
	if err == nil {
		t.Fatal("expected cyclic include error, got nil")
	}

	if !strings.Contains(err.Error(), "cyclic !include detected") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestResolveIncludes_AbsolutePathNormalization verifies that absolute
// !include paths are normalized with filepath.Clean before they become
// cycle-detection keys: the same file reached through differently spelled
// absolute paths (/d/a.yaml vs /d/./a.yaml) must count as one include.
//
// A single self-include is not enough to expose the pre-fix gap: any fixed
// self-spelling repeats verbatim on the second lap, so the cycle is detected
// within a few levels even without normalization. Instead, a ring of
// maxIncludeDepth files is built where the i-th file is included through a
// spelling with i redundant "./" elements — every spelling is unique, so the
// unnormalized chain keys never collide and the ring is stopped only by the
// depth guard ("maximum !include depth 32 exceeded"). With normalization all
// spellings collapse to the plain file name and the ring is reported as a
// cycle ("cyclic !include detected").
func TestResolveIncludes_AbsolutePathNormalization(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()

	// f_i (i >= 1) includes f_{i+1} through dir + "/" + (i+1)×"./" + f_{i+1}.yaml;
	// f_{maxIncludeDepth-1} closes the ring back to f0 with maxIncludeDepth
	// redundant "./" elements — a spelling unique within the first lap.
	// filepath.Join cleans its arguments, so the unclean spellings are built
	// by string concatenation to keep the "./" elements intact.
	for i := 0; i < maxIncludeDepth; i++ {
		next := (i + 1) % maxIncludeDepth
		content := "leaf: !include " + dir + "/" + strings.Repeat("./", i+1) + fmt.Sprintf("f%d.yaml", next) + "\n"
		writeTestFile(t, dir, fmt.Sprintf("f%d.yaml", i), content)
	}

	main := writeTestFile(t, dir, "config.yaml", "leaf: !include "+filepath.Join(dir, "f0.yaml")+"\n")

	type Config struct {
		Leaf string `yaml:"leaf" description:"Leaf value"`
	}

	var cfg Config
	err := LoadFile(main, &cfg)
	if err == nil {
		t.Fatal("expected cyclic include error, got nil")
	}

	if !strings.Contains(err.Error(), "cyclic !include detected") {
		t.Fatalf("expected cyclic error for differently-spelled absolute paths, got: %v", err)
	}
}

// TestResolveIncludes_TrackerRecordsCleanedPath verifies that the include
// tracker holds the normalized (filepath.Clean) absolute path even when the
// first registration of a field went through an unclean spelling:
// FindIncludeFileAbs must return the cleaned path for write-back.
func TestResolveIncludes_TrackerRecordsCleanedPath(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	dir := t.TempDir()

	// filepath.Join cleans its arguments, so the unclean spelling is built by
	// string concatenation to keep the "/./" element intact.
	unclean := dir + "/./a.yaml"
	clean := filepath.Join(dir, "a.yaml")

	writeTestFile(t, dir, "a.yaml", "deep\n")
	main := writeTestFile(t, dir, "config.yaml", "leaf: !include "+unclean+"\n")

	type Config struct {
		Leaf string `yaml:"leaf" description:"Leaf value"`
	}

	var cfg Config
	if err := LoadFile(main, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Leaf != "deep" {
		t.Errorf("expected included value, got %q", cfg.Leaf)
	}

	if abs := FindIncludeFileAbs("leaf"); abs != clean {
		t.Errorf("expected tracker to record cleaned path %q, got %q", clean, abs)
	}
}

// TestLoadWithDiamondInclude verifies that the same file included twice via
// sibling branches (a diamond, not a cycle) stays legal.
func TestLoadWithDiamondInclude(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "shared.yaml", "value: 42\n")

	type Shared struct {
		Value int `yaml:"value" description:"Shared value"`
	}

	type Config struct {
		Left  Shared `yaml:"left" description:"Left branch"`
		Right Shared `yaml:"right" description:"Right branch"`
	}

	main := writeTestFile(t, dir, "config.yaml", "left: !include shared.yaml\nright: !include shared.yaml\n")

	var cfg Config
	err := LoadFile(main, &cfg)
	if err != nil {
		t.Fatalf("diamond include must stay legal, got: %v", err)
	}

	if cfg.Left.Value != 42 || cfg.Right.Value != 42 {
		t.Errorf("unexpected diamond values: %+v", cfg)
	}
}

// TestLoadWithIncludeDepthLimit verifies that a chain longer than the
// resolution depth limit is rejected with a dedicated error.
func TestLoadWithIncludeDepthLimit(t *testing.T) {
	dir := t.TempDir()

	// Build a linear chain of maxIncludeDepth+1 files: f0 -> f1 -> ... -> fN.
	for i := 0; i <= maxIncludeDepth; i++ {
		content := "leaf: deep\n"
		if i < maxIncludeDepth {
			content = fmt.Sprintf("leaf: !include f%d.yaml\n", i+1)
		}
		writeTestFile(t, dir, fmt.Sprintf("f%d.yaml", i), content)
	}

	type Config struct {
		Leaf string `yaml:"leaf" description:"Leaf value"`
	}

	main := writeTestFile(t, dir, "config.yaml", "leaf: !include f0.yaml\n")

	var cfg Config
	err := LoadFile(main, &cfg)
	if err == nil {
		t.Fatal("expected depth limit error, got nil")
	}

	if !strings.Contains(err.Error(), "maximum !include depth") {
		t.Errorf("unexpected error text: %v", err)
	}
}
