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

	if err := resolveIncludes(&node, baseDir, ""); err != nil {
		return err
	}

	if err := SetDefaults(out); err != nil {
		return fmt.Errorf("failed to set default values: %w", err)
	}

	if err := node.Decode(out); err != nil {
		return fmt.Errorf("failed to decode YAML AST into target structure: %w", err)
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
func Load(in []byte, out any, opts ...Option) error {
	ResetIncludeTracker()

	var node yaml4.Node

	if err := yaml4.Load(in, &node, opts...); err != nil {
		return fmt.Errorf("failed to parse YAML to AST: %w", err)
	}

	if err := resolveIncludes(&node, ".", ""); err != nil {
		return err
	}

	if err := SetDefaults(out); err != nil {
		return fmt.Errorf("failed to set default values: %w", err)
	}

	if err := node.Decode(out); err != nil {
		return fmt.Errorf("failed to decode YAML AST into target structure: %w", err)
	}

	return Validate(out)
}

// Unmarshal decodes a slice of YAML-encoded bytes into the target destination.
// It serves as a direct proxy wrapper around [Load] without passing extra options.
func Unmarshal(data []byte, out any) error {
	return Load(data, out)
}

// Loader provides a sequential streaming configuration pipeline wrapper
// equipped with automatic default initialization and validation layers.
type Loader struct {
	l *yaml4.Loader
}

// NewLoader returns a new [Loader] instance that reads from the provided [io.Reader]
// stream initialized with the specified parsing configuration options.
func NewLoader(r io.Reader, opts ...Option) (*Loader, error) {
	origLoader, err := yaml4.NewLoader(r, opts...)
	if err != nil {
		return nil, err
	}

	return &Loader{l: origLoader}, nil
}

// Load extracts the subsequent document element out of the active data stream,
// updates default tags, maps configurations, and checks evaluation validation constraints.
func (loader *Loader) Load(v any) error {
	if err := SetDefaults(v); err != nil {
		return err
	}

	if err := loader.l.Load(v); err != nil {
		return err
	}

	if err := SetDefaults(v); err != nil {
		return err
	}

	return Validate(v)
}
