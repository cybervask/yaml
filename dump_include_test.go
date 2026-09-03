package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
// delegates to DumpWithInclude.
func TestMarshalWithIncludeProxy(t *testing.T) {
	t.Chdir(t.TempDir())

	type Config struct {
		Server struct {
			Logging struct {
				Level string `yaml:"level"`
			} `yaml:"logging,include"`
		} `yaml:"server"`
	}

	out, err := MarshalWithInclude(Config{})
	if err != nil {
		t.Fatalf("expected successful marshal, got: %v", err)
	}

	if !strings.Contains(string(out), "!include") {
		t.Errorf("expected !include marker in output, got:\n%s", out)
	}
}
