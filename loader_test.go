package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoadBasic verifies that Load decodes YAML bytes, applies defaults,
// and validates the resulting structure.
func TestLoadBasic(t *testing.T) {
	type Config struct {
		Host    string        `yaml:"host" default:"localhost" description:"Listen host"`
		Port    int           `yaml:"port" default:"8080" description:"Listen port"`
		Timeout time.Duration `yaml:"timeout" default:"5s" description:"Request timeout"`
	}

	var cfg Config
	if err := Load([]byte("host: example.com\nport: 9090\n"), &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Host != "example.com" || cfg.Port != 9090 || cfg.Timeout != 5*time.Second {
		t.Errorf("unexpected config state: %+v", cfg)
	}
}

// TestLoadValidationFailure verifies that Load reports validation faults
// triggered by the decoded content.
func TestLoadValidationFailure(t *testing.T) {
	type Config struct {
		Mode string `yaml:"mode" validate:"choice=dev,prod" description:"Run mode"`
	}

	var cfg Config
	err := Load([]byte("mode: staging\n"), &cfg)
	if err == nil {
		t.Fatal("expected validation failure, got nil")
	}

	if !strings.Contains(err.Error(), "allowed choices") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestUnmarshalAlias checks that the Unmarshal compatibility proxy behaves
// identically to Load.
func TestUnmarshalAlias(t *testing.T) {
	type Config struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}

	var cfg Config
	if err := Unmarshal([]byte("level: debug\n"), &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Level != "debug" {
		t.Errorf("expected level=debug, got %q", cfg.Level)
	}
}

// TestLoadFileRoundTrip verifies loading from a file, default application,
// and validation through the file-based API.
func TestLoadFileRoundTrip(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"localhost" description:"Listen host"`
		Port int    `yaml:"port" validate:"min=1,max=65535" description:"Listen port"`
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("port: 443\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(file, &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Host != "localhost" || cfg.Port != 443 {
		t.Errorf("unexpected config state: %+v", cfg)
	}
}

// TestLoadFileMissingFile verifies the error path when the target file
// does not exist.
func TestLoadFileMissingFile(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" description:"Listen host"`
	}

	var cfg Config
	err := LoadFile(filepath.Join(t.TempDir(), "missing.yaml"), &cfg)
	if err == nil {
		t.Fatal("expected missing file error, got nil")
	}
}

// TestLoadFileValidationFailure verifies that LoadFile runs validation
// and reports constraint violations.
func TestLoadFileValidationFailure(t *testing.T) {
	type Config struct {
		Workers int `yaml:"workers" validate:"min=1,max=4" description:"Worker count"`
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("workers: 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	err := LoadFile(file, &cfg)
	if err == nil {
		t.Fatal("expected validation failure, got nil")
	}

	if !strings.Contains(err.Error(), "max 4") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestUnmarshalFileAlias checks that the UnmarshalFile compatibility proxy
// behaves identically to LoadFile.
func TestUnmarshalFileAlias(t *testing.T) {
	type Config struct {
		Name string `yaml:"name" default:"app" description:"Service name"`
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("name: billing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := UnmarshalFile(file, &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Name != "billing" {
		t.Errorf("expected name=billing, got %q", cfg.Name)
	}
}

// TestLoaderStream verifies the streaming Loader: SetDefaults runs before the
// first document and validation runs after the document is decoded.
func TestLoaderStream(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"localhost" description:"Listen host"`
		Port int    `yaml:"port" validate:"min=1" description:"Listen port"`
	}

	stream := strings.NewReader("host: a.example\nport: 80\n---\nhost: b.example\nport: 81\n")

	loader, err := NewLoader(stream)
	if err != nil {
		t.Fatal(err)
	}

	var first Config
	if err := loader.Load(&first); err != nil {
		t.Fatalf("first document: %v", err)
	}

	if first.Host != "a.example" || first.Port != 80 {
		t.Errorf("unexpected first document: %+v", first)
	}

	var second Config
	if err := loader.Load(&second); err != nil {
		t.Fatalf("second document: %v", err)
	}

	if second.Host != "b.example" || second.Port != 81 {
		t.Errorf("unexpected second document: %+v", second)
	}
}

// TestLoaderStreamValidationFailure verifies that the streaming Loader
// reports validation faults of decoded documents.
func TestLoaderStreamValidationFailure(t *testing.T) {
	type Config struct {
		Port int `yaml:"port" validate:"min=1,max=65535" description:"Listen port"`
	}

	loader, err := NewLoader(strings.NewReader("port: 70000\n"))
	if err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := loader.Load(&cfg); err == nil {
		t.Fatal("expected validation failure, got nil")
	}
}
