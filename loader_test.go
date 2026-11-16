package yaml

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	yaml4 "go.yaml.in/yaml/v4"
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

// TestLoader_Load_ParseErrorLeavesTargetUntouched verifies the parse-error
// contract of the streaming Loader: when the document cannot be read, the
// target struct stays untouched — defaults must not be applied before the
// document has been parsed successfully, matching the behavior of
// Load and LoadFile.
func TestLoader_Load_ParseErrorLeavesTargetUntouched(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"x" description:"Listen host"`
	}

	loader, err := NewLoader(strings.NewReader("{a: 1\n"))
	if err != nil {
		t.Fatal(err)
	}

	var cfg Config
	err = loader.Load(&cfg)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}

	if cfg.Host != "" {
		t.Errorf("expected target to stay untouched on parse error, got host=%q", cfg.Host)
	}
}

// TestLoader_Load_IncludeErrorLeavesTargetUntouched verifies the include-error
// contract of the streaming Loader: when !include resolution fails, the target
// struct stays untouched — defaults must not be applied before the document
// has been fully resolved, matching the behavior of Load and LoadFile, where
// SetDefaults runs after resolveIncludes.
func TestLoader_Load_IncludeErrorLeavesTargetUntouched(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	t.Chdir(t.TempDir())

	if err := os.WriteFile("config.yaml", []byte("host: !include missing.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stream, err := os.Open("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	type Config struct {
		Host string `yaml:"host" default:"DEFAULT-HOST" description:"Listen host"`
	}

	loader, err := NewLoader(stream)
	if err != nil {
		t.Fatal(err)
	}

	var cfg Config
	err = loader.Load(&cfg)
	if err == nil {
		t.Fatal("expected include resolution error, got nil")
	}

	if !strings.Contains(err.Error(), "missing.yaml") {
		t.Errorf("expected the error to name the missing include file, got: %v", err)
	}
	if cfg.Host != "" {
		t.Errorf("expected target to stay untouched on include error, got host=%q", cfg.Host)
	}
}

// TestLoader_Load_ErrorPrefixesParity verifies that Loader.Load wraps its
// stage errors in the same prefixes as Load and LoadFile ("failed to parse
// YAML to AST", "failed to set default values", "failed to decode YAML AST
// into target structure", "failed to apply environment variable overrides"),
// so callers matching error text get uniform messages across every load path.
func TestLoader_Load_ErrorPrefixesParity(t *testing.T) {
	t.Run("parse error", func(t *testing.T) {
		type Config struct {
			Host string `yaml:"host" description:"Listen host"`
		}

		loader, err := NewLoader(strings.NewReader("{a: 1\n"))
		if err != nil {
			t.Fatal(err)
		}

		var cfg Config
		err = loader.Load(&cfg)
		if err == nil || !strings.HasPrefix(err.Error(), "failed to parse YAML to AST: ") {
			t.Fatalf("expected 'failed to parse YAML to AST: …' prefix, got: %v", err)
		}
	})

	t.Run("defaults error", func(t *testing.T) {
		type Config struct {
			Mode string `yaml:"mode" priority:"value" description:"Run mode"`
		}

		loader, err := NewLoader(strings.NewReader("mode: dev\n"))
		if err != nil {
			t.Fatal(err)
		}

		var cfg Config
		err = loader.Load(&cfg)
		if err == nil || !strings.HasPrefix(err.Error(), "failed to set default values: ") {
			t.Fatalf("expected 'failed to set default values: …' prefix, got: %v", err)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		type Config struct {
			Port int `yaml:"port" description:"Listen port"`
		}

		loader, err := NewLoader(strings.NewReader("port: not-a-number\n"))
		if err != nil {
			t.Fatal(err)
		}

		var cfg Config
		err = loader.Load(&cfg)
		if err == nil || !strings.HasPrefix(err.Error(), "failed to decode YAML AST into target structure: ") {
			t.Fatalf("expected 'failed to decode YAML AST into target structure: …' prefix, got: %v", err)
		}
	})

	t.Run("env error", func(t *testing.T) {
		type Config struct {
			Port int `yaml:"port" env:"TEST_LOADER_PREFIX_PORT" description:"Listen port"`
		}

		t.Setenv("TEST_LOADER_PREFIX_PORT", "not-a-number")

		loader, err := NewLoader(strings.NewReader("port: 443\n"))
		if err != nil {
			t.Fatal(err)
		}

		var cfg Config
		err = loader.Load(&cfg)
		if err == nil || !strings.HasPrefix(err.Error(), "failed to apply environment variable overrides: ") {
			t.Fatalf("expected 'failed to apply environment variable overrides: …' prefix, got: %v", err)
		}
	})

	t.Run("parity with Load", func(t *testing.T) {
		type Config struct {
			Port int `yaml:"port" description:"Listen port"`
		}

		var cfg Config
		err := Load([]byte("port: not-a-number\n"), &cfg)
		if err == nil || !strings.HasPrefix(err.Error(), "failed to decode YAML AST into target structure: ") {
			t.Fatalf("expected Load to use the same 'failed to decode YAML AST into target structure: …' prefix, got: %v", err)
		}
	})
}

// TestLoader_Load_ResolvesInclude verifies that the streaming Loader resolves
// !include directives: the value from the included file is decoded, the
// include tracker is filled, and validation passes. Relative include paths
// resolve against the working directory.
func TestLoader_Load_ResolvesInclude(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	t.Chdir(t.TempDir())

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("logging.yaml", []byte("level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("config.yaml", []byte("server:\n  logging: !include logging.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stream, err := os.Open("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	loader, err := NewLoader(stream)
	if err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("expected streaming load to resolve !include, got error: %v", err)
	}

	if cfg.Server.Logging.Level != "debug" {
		t.Errorf("expected included value, got level %q", cfg.Server.Logging.Level)
	}

	if tracked := FindIncludeFileAbs("server.logging"); tracked != filepath.Join(cwd, "logging.yaml") {
		t.Errorf("expected include tracker to record %s, got %q", filepath.Join(cwd, "logging.yaml"), tracked)
	}
	if rel := FindIncludeFile("server.logging"); rel != "logging.yaml" {
		t.Errorf("expected include tracker relative path logging.yaml, got %q", rel)
	}
}

// TestLoader_Load_TwoDocsSequential verifies that consecutive documents of a
// stream each resolve their own !include directives and that the tracker
// reflects the most recently loaded document.
func TestLoader_Load_TwoDocsSequential(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	t.Chdir(t.TempDir())

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("a.yaml", []byte("level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("b.yaml", []byte("level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("multi.yaml", []byte("server:\n  logging: !include a.yaml\n---\nserver:\n  logging: !include b.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stream, err := os.Open("multi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Config struct {
		Server struct {
			Logging Logging `yaml:"logging"`
		} `yaml:"server"`
	}

	loader, err := NewLoader(stream)
	if err != nil {
		t.Fatal(err)
	}

	var first Config
	if err := loader.Load(&first); err != nil {
		t.Fatalf("first document: %v", err)
	}
	if first.Server.Logging.Level != "info" {
		t.Errorf("unexpected first document: %+v", first)
	}
	if tracked := FindIncludeFileAbs("server.logging"); tracked != filepath.Join(cwd, "a.yaml") {
		t.Errorf("expected tracker to point at %s after first document, got %q", filepath.Join(cwd, "a.yaml"), tracked)
	}

	var second Config
	if err := loader.Load(&second); err != nil {
		t.Fatalf("second document: %v", err)
	}
	if second.Server.Logging.Level != "debug" {
		t.Errorf("unexpected second document: %+v", second)
	}
	if tracked := FindIncludeFileAbs("server.logging"); tracked != filepath.Join(cwd, "b.yaml") {
		t.Errorf("expected tracker to point at %s after second document, got %q", filepath.Join(cwd, "b.yaml"), tracked)
	}

	var third Config
	if err := loader.Load(&third); !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF on exhausted stream, got %v", err)
	}
}

// TestLoader_Load_NoIncludesUnchanged verifies that the streaming Loader keeps
// its previous behavior for documents without !include directives, and that
// decode-stage options are still passed through.
func TestLoader_Load_NoIncludesUnchanged(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"localhost" description:"Listen host"`
		Port int    `yaml:"port" validate:"min=1,max=65535" description:"Listen port"`
	}

	loader, err := NewLoader(strings.NewReader("host: example.com\nport: 443\n"))
	if err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Host != "example.com" || cfg.Port != 443 {
		t.Errorf("unexpected config state: %+v", cfg)
	}

	if rel := FindIncludeFile("host"); rel != "" {
		t.Errorf("expected empty include tracker for document without includes, got %q", rel)
	}

	// Decode-stage options are preserved: WithKnownFields must reject an
	// unknown top-level key.
	stream := strings.NewReader("host: example.com\nport: 443\nunknown: 1\n")
	loader, err = NewLoader(stream, WithKnownFields())
	if err != nil {
		t.Fatal(err)
	}

	var strict Config
	err = loader.Load(&strict)
	if err == nil {
		t.Fatal("expected WithKnownFields to reject unknown key, got nil")
	}
	if strings.Contains(err.Error(), "!include") {
		t.Errorf("unexpected include-related error: %v", err)
	}
}

// TestLoad_TrackerRecordsAbsoluteForLoadBytes verifies that the bytes-based
// Load resolves !include directives against the real working directory and
// records an absolute path in the include tracker, as documented by
// FindIncludeFileAbs.
func TestLoad_TrackerRecordsAbsoluteForLoadBytes(t *testing.T) {
	ResetIncludeTracker()
	t.Cleanup(ResetIncludeTracker)

	t.Chdir(t.TempDir())

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("logging.yaml", []byte("level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type Logging struct {
		Level string `yaml:"level" default:"info" description:"Log level"`
	}
	type Config struct {
		Logging Logging `yaml:"logging" description:"Logging settings"`
	}

	var cfg Config
	if err := Load([]byte("logging: !include logging.yaml\n"), &cfg); err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected included value, got level %q", cfg.Logging.Level)
	}

	abs := FindIncludeFileAbs("logging")
	if !filepath.IsAbs(abs) {
		t.Fatalf("expected absolute tracked path, got %q", abs)
	}
	if abs != filepath.Join(cwd, "logging.yaml") {
		t.Errorf("expected tracked path %q, got %q", filepath.Join(cwd, "logging.yaml"), abs)
	}
}

// TestLoad_KnownFieldsRejectsUnknownKey verifies that decode-stage options
// reach the decode step: WithKnownFields must reject an unknown key exactly
// like the upstream yaml4.Load does for the same input and option.
func TestLoad_KnownFieldsRejectsUnknownKey(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"localhost" description:"Listen host"`
	}

	data := []byte("host: example.com\nunknown: 1\n")

	var cfg Config
	err := Load(data, &cfg, WithKnownFields())
	if err == nil {
		t.Fatal("expected WithKnownFields to reject unknown key, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected error to name the unknown field, got: %v", err)
	}

	// Drop-in equivalence: upstream yaml4.Load on the same input and option
	// must fail the same way.
	var upstream Config
	if uerr := yaml4.Load(data, &upstream, yaml4.WithKnownFields()); uerr == nil {
		t.Fatal("expected upstream yaml4.Load to reject the same input, got nil")
	}
}

// TestLoadFile_KnownFieldsRejectsUnknownKey verifies that decode-stage options
// reach the decode step of the file-based API: WithKnownFields must reject an
// unknown key.
func TestLoadFile_KnownFieldsRejectsUnknownKey(t *testing.T) {
	type Config struct {
		Port int `yaml:"port" validate:"min=1,max=65535" description:"Listen port"`
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("port: 443\nunknown: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := LoadFile(file, &cfg, WithKnownFields()); err == nil {
		t.Fatal("expected WithKnownFields to reject unknown key, got nil")
	}
}

// TestLoad_KnownFieldsAcceptsValidConfig verifies that strict mode does not
// reject a fully known document and the decoded values land in the target
// (a guard against an over-strict decode step).
func TestLoad_KnownFieldsAcceptsValidConfig(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"localhost" description:"Listen host"`
		Port int    `yaml:"port" default:"8080" description:"Listen port"`
	}

	var cfg Config
	if err := Load([]byte("host: example.com\nport: 9090\n"), &cfg, WithKnownFields()); err != nil {
		t.Fatalf("expected valid strict load, got error: %v", err)
	}

	if cfg.Host != "example.com" || cfg.Port != 9090 {
		t.Errorf("unexpected config state: %+v", cfg)
	}
}

// TestLoad_UniqueKeysRejectsDuplicateKey verifies that duplicate mapping keys
// are reported end-to-end. Upstream yaml4 v4.0.0 rejects duplicate keys
// unconditionally at construct time, so this fails on pre-fix code as well:
// it is a regression guard that the strict-option path stays intact after
// decode-stage options are routed through Node.Load.
func TestLoad_UniqueKeysRejectsDuplicateKey(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"localhost" description:"Listen host"`
	}

	data := []byte("host: first.example\nhost: second.example\n")

	var cfg Config
	if err := Load(data, &cfg, WithUniqueKeys()); err == nil {
		t.Fatal("expected WithUniqueKeys to reject duplicate key, got nil")
	}

	// Upstream parity: plain yaml4.Load rejects the same input without options.
	var upstream Config
	if uerr := yaml4.Load(data, &upstream); uerr == nil {
		t.Fatal("expected upstream yaml4.Load to reject duplicate key, got nil")
	}
}

// TestLoad_NonStrictKeepsPreviousBehavior verifies that without strict decode
// options the previous lenient behavior is preserved: unknown keys are ignored
// and format-only options do not disturb loading (a guard against over-broad
// strictness). Duplicate keys are intentionally not asserted here: upstream
// v4.0.0 rejects them unconditionally, with or without WithUniqueKeys.
func TestLoad_NonStrictKeepsPreviousBehavior(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"localhost" description:"Listen host"`
	}

	var cfg Config
	if err := Load([]byte("host: example.com\nunknown: 1\n"), &cfg); err != nil {
		t.Fatalf("expected unknown key to be ignored without options, got: %v", err)
	}
	if cfg.Host != "example.com" {
		t.Errorf("unexpected config state: %+v", cfg)
	}

	var indented Config
	if err := Load([]byte("host: third.example\n"), &indented, WithIndent(4)); err != nil {
		t.Fatalf("expected format-only option to keep load working, got: %v", err)
	}
	if indented.Host != "third.example" {
		t.Errorf("unexpected config state: %+v", indented)
	}
}
