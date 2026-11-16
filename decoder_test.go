package yaml

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decoderIncludeConfig is the shared test model for the Decoder.Decode
// include-resolution tests: it carries tag defaults (to prove they are not
// applied on a failed or empty stream) and a field targeted by !include.
type decoderIncludeConfig struct {
	Host string `yaml:"host" default:"localhost"`
	Port int    `yaml:"port" default:"8080"`
	Data string `yaml:"data"`
}

// envPriorityTLS mirrors a nested configuration block carrying its own env tags.
type envPriorityTLS struct {
	MinVersion string `yaml:"min_version" default:"tls1.3" env:"TEST_ENVPRIO_TLS_MIN"`
}

// envPriorityConfig is the shared test model for the env-priority contract:
// a non-empty environment variable must override both tag defaults and
// explicitly decoded YAML values in every load path (README, "Environment
// Injection": env takes absolute precedence over YAML and tag defaults).
type envPriorityConfig struct {
	Host        string         `yaml:"host" default:"localhost" env:"TEST_ENVPRIO_HOST"`
	Port        int            `yaml:"port" default:"8080" env:"TEST_ENVPRIO_PORT"`
	Region      string         `yaml:"region" default:"eu-west"`
	TLS         envPriorityTLS `yaml:"tls"`
	NotInConfig string         `yaml:"not_in_config" default:"fallback"`
}

// envPriorityLoader runs one load path against the given YAML input.
type envPriorityLoader struct {
	name string
	run  func(t *testing.T, in string, out any) error
}

// envPriorityLoaders enumerates every public load path whose env-priority
// contract is documented in README ("Environment Injection"): a non-empty
// environment variable takes absolute precedence over YAML values and tag
// defaults. All five paths run the same SetDefaults → decode → env → Validate
// pipeline, so they participate in the parity checks on equal terms.
func envPriorityLoaders() []envPriorityLoader {
	return []envPriorityLoader{
		{"LoadFile", func(t *testing.T, in string, out any) error {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(in), 0o600); err != nil {
				return err
			}
			return LoadFile(path, out)
		}},
		{"Load", func(t *testing.T, in string, out any) error {
			return Load([]byte(in), out)
		}},
		{"Unmarshal", func(t *testing.T, in string, out any) error {
			return Unmarshal([]byte(in), out)
		}},
		{"Decoder.Decode", func(t *testing.T, in string, out any) error {
			return NewDecoder(strings.NewReader(in)).Decode(out)
		}},
		{"Loader.Load", func(t *testing.T, in string, out any) error {
			loader, err := NewLoader(strings.NewReader(in))
			if err != nil {
				return err
			}
			return loader.Load(out)
		}},
	}
}

// TestEnvPriority_Load_EnvOverYAML verifies that in the Load path a non-empty
// environment variable overrides an explicitly present YAML value, including
// nested struct fields. Before the fix, Load left YAML values in
// place, contradicting the documented env > YAML > default contract.
func TestEnvPriority_Load_EnvOverYAML(t *testing.T) {
	t.Setenv("TEST_ENVPRIO_HOST", "env-host")
	t.Setenv("TEST_ENVPRIO_PORT", "9090")
	t.Setenv("TEST_ENVPRIO_TLS_MIN", "tls1.2")

	var cfg envPriorityConfig
	in := "host: yaml-host\nport: 7000\nregion: us-east\ntls:\n  min_version: tls1.3\n"
	if err := Load([]byte(in), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "env-host" {
		t.Errorf("Host: expected env override %q, got %q", "env-host", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port: expected env override 9090, got %d", cfg.Port)
	}
	if cfg.TLS.MinVersion != "tls1.2" {
		t.Errorf("TLS.MinVersion: expected env override %q, got %q", "tls1.2", cfg.TLS.MinVersion)
	}
	if cfg.Region != "us-east" {
		t.Errorf("Region (no env tag): expected YAML value %q, got %q", "us-east", cfg.Region)
	}
}

// TestEnvPriority_Decoder_EnvOverYAML pins the Decoder.Decode env-over-YAML
// precedence as a regression guard: the second engine pass must keep env
// values above decoded YAML values (previously implemented by the second
// SetDefaults, replaced by applyEnvOverrides).
func TestEnvPriority_Decoder_EnvOverYAML(t *testing.T) {
	t.Setenv("TEST_ENVPRIO_HOST", "env-host")
	t.Setenv("TEST_ENVPRIO_PORT", "9090")

	var cfg envPriorityConfig
	in := "host: yaml-host\nport: 7000\n"
	if err := NewDecoder(strings.NewReader(in)).Decode(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "env-host" {
		t.Errorf("Host: expected env override %q, got %q", "env-host", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port: expected env override 9090, got %d", cfg.Port)
	}
}

// TestEnvPriority_CrossPathParity asserts that one matrix of inputs produces
// identical results through every public load path: LoadFile, Load,
// Decoder.Decode and Loader.Load. Before the fix, the env+YAML case
// diverged (Load/LoadFile kept YAML values, Decoder.Decode/Loader.Load
// returned env values).
func TestEnvPriority_CrossPathParity(t *testing.T) {
	type envPriorityCase struct {
		name       string
		yaml       string
		envSet     bool
		wantConfig envPriorityConfig
	}

	cases := []envPriorityCase{
		{
			name:   "env over explicit YAML values",
			yaml:   "host: yaml-host\nport: 7000\ntls:\n  min_version: tls1.3\n",
			envSet: true,
			wantConfig: envPriorityConfig{
				Host:        "env-host",
				Port:        9090,
				Region:      "eu-west",
				TLS:         envPriorityTLS{MinVersion: "tls1.2"},
				NotInConfig: "fallback",
			},
		},
		{
			name:   "env over explicit YAML zero",
			yaml:   "host: \"\"\nport: 0\ntls:\n  min_version: \"\"\n",
			envSet: true,
			wantConfig: envPriorityConfig{
				Host:        "env-host",
				Port:        9090,
				Region:      "eu-west",
				TLS:         envPriorityTLS{MinVersion: "tls1.2"},
				NotInConfig: "fallback",
			},
		},
		{
			name:   "env over defaults",
			yaml:   "region: us-east\n",
			envSet: true,
			wantConfig: envPriorityConfig{
				Host:        "env-host",
				Port:        9090,
				Region:      "us-east",
				TLS:         envPriorityTLS{MinVersion: "tls1.2"},
				NotInConfig: "fallback",
			},
		},
		{
			name: "YAML only",
			yaml: "host: yaml-host\nport: 7000\nregion: us-east\ntls:\n  min_version: tls1.2\n",
			wantConfig: envPriorityConfig{
				Host:        "yaml-host",
				Port:        7000,
				Region:      "us-east",
				TLS:         envPriorityTLS{MinVersion: "tls1.2"},
				NotInConfig: "fallback",
			},
		},
		{
			name: "defaults only",
			yaml: "unused_key: 1\n",
			wantConfig: envPriorityConfig{
				Host:        "localhost",
				Port:        8080,
				Region:      "eu-west",
				TLS:         envPriorityTLS{MinVersion: "tls1.3"},
				NotInConfig: "fallback",
			},
		},
	}

	loaders := envPriorityLoaders()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envSet {
				t.Setenv("TEST_ENVPRIO_HOST", "env-host")
				t.Setenv("TEST_ENVPRIO_PORT", "9090")
				t.Setenv("TEST_ENVPRIO_TLS_MIN", "tls1.2")
			}

			results := make(map[string]envPriorityConfig, len(loaders))
			for _, loader := range loaders {
				var cfg envPriorityConfig
				if err := loader.run(t, tc.yaml, &cfg); err != nil {
					t.Fatalf("%s: unexpected error: %v", loader.name, err)
				}
				results[loader.name] = cfg

				if cfg != tc.wantConfig {
					t.Errorf("%s: config mismatch:\n got: %+v\nwant: %+v", loader.name, cfg, tc.wantConfig)
				}
			}

			base := results[loaders[0].name]
			for _, loader := range loaders[1:] {
				if results[loader.name] != base {
					t.Errorf("%s: result differs from %s: %+v vs %+v", loader.name, loaders[0].name, results[loader.name], base)
				}
			}
		})
	}
}

// TestEnvPriority_ExplicitZeroYAMLKeepsZero is a guard for the documented
// contract change: an explicit YAML zero value must survive over the tag
// default when no environment variable is set (matching the Load semantics).
// All four load paths are covered: Loader.Load runs the same single
// SetDefaults pass followed by the node decode, so an explicit YAML zero
// survives there as well.
func TestEnvPriority_ExplicitZeroYAMLKeepsZero(t *testing.T) {
	in := "host: \"\"\nport: 0\ntls:\n  min_version: \"\"\n"
	want := envPriorityConfig{
		Region:      "eu-west",
		NotInConfig: "fallback",
	}

	paths := []struct {
		name string
		run  func(t *testing.T) (envPriorityConfig, error)
	}{
		{"LoadFile", func(t *testing.T) (envPriorityConfig, error) {
			var cfg envPriorityConfig
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(in), 0o600); err != nil {
				return cfg, err
			}
			err := LoadFile(path, &cfg)
			return cfg, err
		}},
		{"Load", func(t *testing.T) (envPriorityConfig, error) {
			var cfg envPriorityConfig
			err := Load([]byte(in), &cfg)
			return cfg, err
		}},
		{"Decoder.Decode", func(t *testing.T) (envPriorityConfig, error) {
			var cfg envPriorityConfig
			err := NewDecoder(strings.NewReader(in)).Decode(&cfg)
			return cfg, err
		}},
		{"Loader.Load", func(t *testing.T) (envPriorityConfig, error) {
			var cfg envPriorityConfig
			loader, lerr := NewLoader(strings.NewReader(in))
			if lerr != nil {
				return cfg, lerr
			}
			err := loader.Load(&cfg)
			return cfg, err
		}},
	}

	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			cfg, err := path.run(t)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg != want {
				t.Errorf("explicit YAML zero was replaced:\n got: %+v\nwant: %+v", cfg, want)
			}
		})
	}
}

// TestEnvPriority_EnvOverDefault is a regression guard: with the YAML key
// absent, a non-empty environment variable keeps overriding the tag default
// in every load path.
func TestEnvPriority_EnvOverDefault(t *testing.T) {
	t.Setenv("TEST_ENVPRIO_HOST", "env-host")
	t.Setenv("TEST_ENVPRIO_TLS_MIN", "tls1.2")

	in := "region: us-east\n"
	want := envPriorityConfig{
		Host:        "env-host",
		Port:        8080,
		Region:      "us-east",
		TLS:         envPriorityTLS{MinVersion: "tls1.2"},
		NotInConfig: "fallback",
	}

	for _, loader := range envPriorityLoaders() {
		t.Run(loader.name, func(t *testing.T) {
			var cfg envPriorityConfig
			if err := loader.run(t, in, &cfg); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg != want {
				t.Errorf("%s: config mismatch:\n got: %+v\nwant: %+v", loader.name, cfg, want)
			}
		})
	}
}

// TestDecoder_Decode_ResolvesInclude verifies that Decoder.Decode resolves
// !include directives against the current working directory: an absolute
// include path pulls the target file content into the decoded field, and the
// target file itself stays untouched (decoding only reads it).
func TestDecoder_Decode_ResolvesInclude(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target.yaml")
	const targetData = "included-value\n"
	if err := os.WriteFile(target, []byte(targetData), 0o600); err != nil {
		t.Fatalf("failed to write include target: %v", err)
	}

	in := "host: main-host\ndata: !include " + target + "\n"
	var cfg decoderIncludeConfig
	if err := NewDecoder(strings.NewReader(in)).Decode(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "main-host" {
		t.Errorf("Host: expected %q, got %q", "main-host", cfg.Host)
	}
	if cfg.Data != "included-value" {
		t.Errorf("Data: expected value included from %q, got %q", target, cfg.Data)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to re-read include target: %v", err)
	}
	if string(got) != targetData {
		t.Errorf("include target file was modified:\n got: %q\nwant: %q", got, targetData)
	}
}

// TestDecoder_Decode_EOFLeavesTargetUntouched pins the FA-I1 fix: decoding
// from an exhausted stream returns io.EOF and leaves the target at its zero
// value — defaults are applied only after a document has been parsed and its
// !include directives resolved.
func TestDecoder_Decode_EOFLeavesTargetUntouched(t *testing.T) {
	var cfg decoderIncludeConfig
	err := NewDecoder(strings.NewReader("")).Decode(&cfg)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got: %v", err)
	}
	if cfg != (decoderIncludeConfig{}) {
		t.Errorf("target was modified on EOF:\n got: %+v\nwant: zero value", cfg)
	}
}

// TestDecoder_Decode_IncludeParityWithLoad asserts that a config with
// !include directives produces an identical result through Load and
// Decoder.Decode: both resolve includes against the working directory and
// run the same defaults → decode → env → Validate pipeline.
func TestDecoder_Decode_IncludeParityWithLoad(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target.yaml")
	if err := os.WriteFile(target, []byte("included-value\n"), 0o600); err != nil {
		t.Fatalf("failed to write include target: %v", err)
	}

	in := "host: main-host\ndata: !include " + target + "\n"

	var fromLoad, fromDecode decoderIncludeConfig
	if err := Load([]byte(in), &fromLoad); err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if err := NewDecoder(strings.NewReader(in)).Decode(&fromDecode); err != nil {
		t.Fatalf("Decoder.Decode: unexpected error: %v", err)
	}

	if fromLoad != fromDecode {
		t.Errorf("Decoder.Decode result differs from Load:\n Load:           %+v\n Decoder.Decode: %+v", fromLoad, fromDecode)
	}
	if fromLoad.Data != "included-value" {
		t.Errorf("Load: expected included value %q, got %q", "included-value", fromLoad.Data)
	}
}

// TestDecoder_Decode_ResolvesRelativeInclude pins the cwd resolution contract:
// a relative !include path in a stream-decoded document resolves against the
// current working directory, exactly like Load and Loader.Load.
func TestDecoder_Decode_ResolvesRelativeInclude(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "relative-target.yaml")
	if err := os.WriteFile(target, []byte("relative-included\n"), 0o600); err != nil {
		t.Fatalf("failed to write include target: %v", err)
	}

	t.Chdir(dir)

	var cfg decoderIncludeConfig
	if err := NewDecoder(strings.NewReader("host: main-host\ndata: !include relative-target.yaml\n")).Decode(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Data != "relative-included" {
		t.Errorf("Data: expected %q, got %q", "relative-included", cfg.Data)
	}
	if got := FindIncludeFileAbs("data"); got != target {
		t.Errorf("expected the tracker to record the absolute path %q, got %q", target, got)
	}
}

// TestDecoder_Decode_NestedInclude checks that a level-2 !include (a marker
// inside an included file) is resolved recursively by Decode as well.
func TestDecoder_Decode_NestedInclude(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "inner.yaml")
	outer := filepath.Join(dir, "outer.yaml")
	if err := os.WriteFile(inner, []byte("inner-value\n"), 0o600); err != nil {
		t.Fatalf("failed to write inner include: %v", err)
	}
	if err := os.WriteFile(outer, []byte("!include "+inner+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write outer include: %v", err)
	}

	var cfg decoderIncludeConfig
	if err := NewDecoder(strings.NewReader("host: main-host\ndata: !include " + outer + "\n")).Decode(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Data != "inner-value" {
		t.Errorf("Data: expected nested value %q, got %q", "inner-value", cfg.Data)
	}
}

// TestDecoder_Decode_CyclicInclude pins the cycle guard: a stream document
// whose include chain loops back on itself fails with the explicit cyclic
// error instead of exhausting memory.
func TestDecoder_Decode_CyclicInclude(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.yaml")
	b := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(a, []byte("data: !include "+b+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write a.yaml: %v", err)
	}
	if err := os.WriteFile(b, []byte("data: !include "+a+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write b.yaml: %v", err)
	}

	var cfg decoderIncludeConfig
	err := NewDecoder(strings.NewReader("host: main-host\ndata: !include " + a + "\n")).Decode(&cfg)
	if err == nil {
		t.Fatal("expected a cyclic include error, got nil")
	}
	if !strings.Contains(err.Error(), "cyclic !include detected") {
		t.Errorf("expected a cyclic include error, got: %v", err)
	}
}

// TestDecoder_Decode_MissingIncludeLeavesTargetUntouched extends the
// untouched-on-error contract to the stream decode path: a failed include
// resolution returns an error and leaves the target at its zero value —
// defaults are not applied.
func TestDecoder_Decode_MissingIncludeLeavesTargetUntouched(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-file.yaml")

	var cfg decoderIncludeConfig
	err := NewDecoder(strings.NewReader("host: main-host\ndata: !include " + missing + "\n")).Decode(&cfg)
	if err == nil {
		t.Fatal("expected an include error, got nil")
	}
	if cfg != (decoderIncludeConfig{}) {
		t.Errorf("target was modified on include failure:\n got: %+v\nwant: zero value", cfg)
	}
}

// TestDecoder_Decode_KnownFieldsAppliesToInclude pins the KnownFields proxy:
// the flag survives into the node decode, so an unknown key inside an
// included file fails the strict check exactly like an unknown key in the
// main document.
func TestDecoder_Decode_KnownFieldsAppliesToInclude(t *testing.T) {
	type strictTLS struct {
		Min string `yaml:"min"`
	}
	type strictConfig struct {
		Host string    `yaml:"host"`
		TLS  strictTLS `yaml:"tls"`
	}

	target := filepath.Join(t.TempDir(), "strict-target.yaml")
	if err := os.WriteFile(target, []byte("min: tls1.2\nrogue_key: nope\n"), 0o600); err != nil {
		t.Fatalf("failed to write include target: %v", err)
	}

	dec := NewDecoder(strings.NewReader("host: main-host\ntls: !include " + target + "\n"))
	dec.KnownFields(true)

	var cfg strictConfig
	err := dec.Decode(&cfg)
	if err == nil {
		t.Fatal("expected a known-fields error for the unknown key inside the include, got nil")
	}
	if !strings.Contains(err.Error(), "rogue_key") {
		t.Errorf("expected the error to name the unknown field, got: %v", err)
	}
}

// TestDecoder_Decode_TrackerHoldsLastDocument pins the per-Decode reset of the
// global include tracker: after a multi-document stream FindIncludeFileAbs
// reports the paths of the last decoded document only. Per-object sessions
// recorded by Load/LoadFile are not affected by the resets.
func TestDecoder_Decode_TrackerHoldsLastDocument(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	if err := os.WriteFile(first, []byte("first-value\n"), 0o600); err != nil {
		t.Fatalf("failed to write first include: %v", err)
	}
	if err := os.WriteFile(second, []byte("second-value\n"), 0o600); err != nil {
		t.Fatalf("failed to write second include: %v", err)
	}

	dec := NewDecoder(strings.NewReader("data: !include " + first + "\n---\ndata: !include " + second + "\n"))

	var cfg decoderIncludeConfig
	if err := dec.Decode(&cfg); err != nil {
		t.Fatalf("first document: unexpected error: %v", err)
	}
	if cfg.Data != "first-value" {
		t.Errorf("first document: expected %q, got %q", "first-value", cfg.Data)
	}
	if got := FindIncludeFileAbs("data"); got != first {
		t.Errorf("after the first document: expected tracker path %q, got %q", first, got)
	}

	if err := dec.Decode(&cfg); err != nil {
		t.Fatalf("second document: unexpected error: %v", err)
	}
	if cfg.Data != "second-value" {
		t.Errorf("second document: expected %q, got %q", "second-value", cfg.Data)
	}
	if got := FindIncludeFileAbs("data"); got != second {
		t.Errorf("after the second document: expected tracker path %q, got %q", second, got)
	}
}

// TestDecoder_Decode_ErrorPrefixes pins the error-wrap parity with the other
// load paths: the node decode and the environment pass
// report their failures with the same prefixes as Load/LoadFile/Loader.Load.
func TestDecoder_Decode_ErrorPrefixes(t *testing.T) {
	dec := NewDecoder(strings.NewReader("host: main-host\nrogue_main: 1\n"))
	dec.KnownFields(true)
	var cfg decoderIncludeConfig
	err := dec.Decode(&cfg)
	if err == nil {
		t.Fatal("expected a node decode error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "failed to decode YAML AST into target structure") {
		t.Errorf("expected the node decode prefix, got: %v", err)
	}

	t.Setenv("TEST_ENVPRIO_PORT", "not-a-number")
	var envCfg envPriorityConfig
	err = NewDecoder(strings.NewReader("host: main-host\nport: 1234\n")).Decode(&envCfg)
	if err == nil {
		t.Fatal("expected an env override error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "failed to apply environment variable overrides") {
		t.Errorf("expected the env override prefix, got: %v", err)
	}
}
