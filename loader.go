package yaml

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	yaml4 "go.yaml.in/yaml/v4"
)

// LoadFile reads a YAML file, resolves !include directives, applies defaults,
// and validates the result into the target structure.
//
// The include metadata of the resolved document is bound to the out pointer:
// a later DumpWithInclude on the same pointer writes back to exactly the
// include files this load recorded.
func LoadFile(filename string, out any, opts ...Option) error {
	ResetIncludeTracker()

	absPath, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	baseDir := filepath.Dir(absPath)

	var node yaml4.Node
	if err := yaml4.Load(data, &node, opts...); err != nil {
		return fmt.Errorf("failed to parse YAML to AST: %w", err)
	}

	session := newIncludeSession(out)

	if err := resolveIncludes(&node, baseDir, "", newIncludeChain(), session); err != nil {
		return err
	}

	commitIncludeSession(session)

	if err := SetDefaults(out); err != nil {
		return fmt.Errorf("failed to set default values: %w", err)
	}

	if err := node.Load(out, opts...); err != nil {
		return fmt.Errorf("failed to decode YAML AST into target structure: %w", err)
	}

	if err := applyEnvOverrides(out); err != nil {
		return fmt.Errorf("failed to apply environment variable overrides: %w", err)
	}

	return Validate(out)
}

// UnmarshalFile is a compatibility proxy for LoadFile.
// It provides a familiar API surface aligned with standard yaml.Unmarshal semantics.
func UnmarshalFile(filename string, out any, opts ...Option) error {
	return LoadFile(filename, out, opts...)
}

// Load decodes a slice of YAML-encoded bytes into the target destination.
// It parses the input bytes into an intermediate Abstract Syntax Tree (AST) using
// the provided Option configuration steps, recursively resolves all external !include
// file paths, applies declared defaults, and runs configuration constraint validation checks.
//
// Like [LoadFile], the resolved include metadata is bound to the out pointer
// so that a later DumpWithInclude on the same pointer writes back to the
// include files this load recorded.
func Load(in []byte, out any, opts ...Option) error {
	ResetIncludeTracker()

	var node yaml4.Node

	if err := yaml4.Load(in, &node, opts...); err != nil {
		return fmt.Errorf("failed to parse YAML to AST: %w", err)
	}

	session := newIncludeSession(out)

	// Relative include paths resolve against the real working directory so
	// that the tracker records absolute paths for every load entry point,
	// as documented by FindIncludeFileAbs.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	if err := resolveIncludes(&node, cwd, "", newIncludeChain(), session); err != nil {
		return err
	}

	commitIncludeSession(session)

	if err := SetDefaults(out); err != nil {
		return fmt.Errorf("failed to set default values: %w", err)
	}

	if err := node.Load(out, opts...); err != nil {
		return fmt.Errorf("failed to decode YAML AST into target structure: %w", err)
	}

	if err := applyEnvOverrides(out); err != nil {
		return fmt.Errorf("failed to apply environment variable overrides: %w", err)
	}

	return Validate(out)
}

// Unmarshal decodes a slice of YAML-encoded bytes into the target destination.
// It serves as a direct proxy wrapper around [Load] without passing extra options.
func Unmarshal(data []byte, out any) error {
	return Load(data, out)
}

// Loader provides a sequential streaming configuration pipeline wrapper
// equipped with automatic default initialization, !include resolution and
// validation layers.
type Loader struct {
	l    *yaml4.Loader
	opts []Option
}

// NewLoader returns a new [Loader] instance that reads from the provided [io.Reader]
// stream initialized with the specified parsing configuration options.
func NewLoader(r io.Reader, opts ...Option) (*Loader, error) {
	origLoader, err := yaml4.NewLoader(r, opts...)
	if err != nil {
		return nil, err
	}

	return &Loader{l: origLoader, opts: opts}, nil
}

// Load extracts the subsequent document element out of the active data stream,
// resolves !include directives against the working directory, updates default
// tags, maps configurations, and checks evaluation validation constraints.
//
// The document is decoded into an intermediate AST node, !include directives
// are resolved recursively (the include tracker is reset per Load call), and
// the resolved node is decoded into v with the options passed to [NewLoader].
// Alias-data options are not supported: the document has already been composed
// by the time the node is decoded.
//
// Unlike [Load] and [LoadFile], no include session is bound to v: a later
// DumpWithInclude of a stream-loaded object runs in the inline safe mode.
// A session previously bound to the same pointer by [Load] or [LoadFile] is
// dropped by a successful Load — the stream replaced the object's content,
// so the earlier binding no longer reflects it (a failed Load keeps it).
//
// Unlike [Decoder.Decode], parse errors — including io.EOF — are wrapped
// with the "failed to parse YAML to AST" prefix.
func (loader *Loader) Load(v any) error {
	ResetIncludeTracker()

	var node yaml4.Node
	if err := loader.l.Load(&node); err != nil {
		return fmt.Errorf("failed to parse YAML to AST: %w", err)
	}

	// Relative include paths resolve against the real working directory so
	// that the tracker records absolute paths for every load entry point,
	// as documented by FindIncludeFileAbs.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	if err := resolveIncludes(&node, cwd, "", newIncludeChain(), nil); err != nil {
		return err
	}

	// Defaults are applied only after the document has been read and its
	// !include directives resolved successfully: a stream parse error or a
	// failed include resolution must leave the target untouched, matching
	// the contract of Load and LoadFile.
	if err := SetDefaults(v); err != nil {
		return fmt.Errorf("failed to set default values: %w", err)
	}

	if err := node.Load(v, loader.opts...); err != nil {
		return fmt.Errorf("failed to decode YAML AST into target structure: %w", err)
	}

	if err := applyEnvOverrides(v); err != nil {
		return fmt.Errorf("failed to apply environment variable overrides: %w", err)
	}

	if err := Validate(v); err != nil {
		return err
	}

	// The stream replaced the object's content without binding a new session:
	// a session an earlier [Load]/[LoadFile] recorded for this pointer no
	// longer reflects the object, so it must not drive a later write-back.
	dropIncludeSession(v)

	return nil
}
