package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	yaml4 "go.yaml.in/yaml/v4"
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

// TestLoadWithIncludeEmptyFile verifies that empty include files are rejected.
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

// TestHandleIncludeNodeAccepted verifies the exported include hook: a node
// tagged !include is decoded from the referenced file.
func TestHandleIncludeNodeAccepted(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "data.yaml", "name: demo\n")

	var node yaml4.Node
	if err := yaml4.Unmarshal([]byte("name: demo\n"), &node); err != nil {
		t.Fatal(err)
	}

	node.Tag = "!include"
	node.Kind = yaml4.ScalarNode
	node.Value = filepath.Join(dir, "data.yaml")

	var target struct {
		Name string
	}

	handled, err := HandleIncludeNode(&node, &target)
	if err != nil {
		t.Fatalf("expected successful include handling, got: %v", err)
	}

	if !handled {
		t.Error("expected !include node to be handled")
	}

	if target.Name != "demo" {
		t.Errorf("expected name=demo, got %q", target.Name)
	}
}

// TestHandleIncludeNodeDeclined verifies that plain nodes without the
// !include tag are declined by the hook without any side effects.
func TestHandleIncludeNodeDeclined(t *testing.T) {
	var node yaml4.Node
	if err := yaml4.Unmarshal([]byte("name: demo\n"), &node); err != nil {
		t.Fatal(err)
	}

	var target struct {
		Name string
	}

	handled, err := HandleIncludeNode(&node, &target)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if handled {
		t.Error("expected plain node to be declined")
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

// TestDumpExtractsIncludes verifies that extractIncludes writes tagged include
// subtrees to external files and replaces them with !include markers in the AST.
//
// The AST is built via Unmarshal because yaml.v4 itself rejects the custom
// "include" tag flag during marshaling, so Dump cannot be used end-to-end
// with include-tagged fields (see the design note in the final report).
func TestDumpExtractsIncludes(t *testing.T) {
	t.Chdir(t.TempDir())

	type Logging struct {
		Level  string `yaml:"level" description:"Log level"`
		Colors bool   `yaml:"colors" description:"Colors"`
	}

	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging,include"`
		} `yaml:"server"`
	}

	cfg := Config{}
	cfg.Server.Logging = Logging{Level: "trace", Colors: true}

	var node yaml4.Node
	if err := yaml4.Unmarshal([]byte("server:\n  logging:\n    level: trace\n    colors: true\n"), &node); err != nil {
		t.Fatal(err)
	}

	if err := extractIncludes(&node, cfg, ".", ""); err != nil {
		t.Fatalf("expected extraction to succeed, got: %v", err)
	}

	data, err := os.ReadFile("logging.yaml")
	if err != nil {
		t.Fatalf("expected extracted logging.yaml, got error: %v", err)
	}

	if !strings.Contains(string(data), "level: trace") {
		t.Errorf("unexpected extracted content:\n%s", data)
	}
}

// TestExtractIncludesCustomPath verifies that the yaml:",include:file.yaml"
// tag form extracts the subtree into the explicitly specified file.
func TestExtractIncludesCustomPath(t *testing.T) {
	t.Chdir(t.TempDir())

	type Logging struct {
		Level string `yaml:"level" description:"Log level"`
	}

	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging,include:custom-logging.yaml"`
		} `yaml:"server"`
	}

	cfg := Config{}
	cfg.Server.Logging = Logging{Level: "warn"}

	var node yaml4.Node
	if err := yaml4.Unmarshal([]byte("server:\n  logging:\n    level: warn\n"), &node); err != nil {
		t.Fatal(err)
	}

	if err := extractIncludes(&node, cfg, ".", ""); err != nil {
		t.Fatalf("expected extraction to succeed, got: %v", err)
	}

	data, err := os.ReadFile("custom-logging.yaml")
	if err != nil {
		t.Fatalf("expected custom-logging.yaml, got error: %v", err)
	}

	if !strings.Contains(string(data), "level: warn") {
		t.Errorf("unexpected extracted content:\n%s", data)
	}

	if p := FindIncludeFile("server.logging"); p != "custom-logging.yaml" {
		t.Errorf("expected tracked path custom-logging.yaml, got %q", p)
	}
}

// TestGetNestedStructNonStruct verifies that getNestedStruct and parseIncludeTag
// safely decline non-struct inputs.
func TestGetNestedStructNonStruct(t *testing.T) {
	if nested := getNestedStruct("scalar", "key"); nested != nil {
		t.Errorf("expected nil for scalar input, got %v", nested)
	}

	if ok, path := parseIncludeTag("scalar", "key"); ok || path != "" {
		t.Errorf("expected decline for scalar input, got ok=%v path=%q", ok, path)
	}
}
