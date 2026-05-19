package yaml

import (
	"fmt"
	"io"

	yaml4 "go.yaml.in/yaml/v4"
)

// Load decodes a slice of YAML-encoded bytes into the target destination.
// It parses the input bytes into an intermediate Abstract Syntax Tree (AST) using
// the provided [Option] configuration steps, recursively resolves all external `!include`
// file paths, applies declared defaults, and runs configuration constraint validation checks.
func Load(in []byte, out any, opts ...Option) error {
	var node yaml4.Node

	// Delegate initial parsing to the original v4 engine to apply custom options
	if err := yaml4.Load(in, &node, opts...); err != nil {
		return fmt.Errorf("failed to parse YAML to AST: %w", err)
	}

	// Recursively resolve file inclusion tokens from the current working directory
	if err := resolveIncludes(&node, "."); err != nil {
		return err
	}

	// Populate structure zero-values with default field tag configurations
	if err := SetDefaults(out); err != nil {
		return fmt.Errorf("failed to set default values: %w", err)
	}

	// Decode the mutated AST directly into the user target object reference
	if err := node.Decode(out); err != nil {
		return fmt.Errorf("failed to decode YAML AST into target structure: %w", err)
	}

	return Validate(out)
	// return nil
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
