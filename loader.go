package yaml

import (
	"io"

	yaml4 "go.yaml.in/yaml/v4"
)

// --- MODERN V4 RECOMMENDED API ---

// Load decodes a slice of YAML-encoded bytes into an unmarshaling destination.
// It applies static field defaults before initialization, populates dynamic
// collection layers (slices and maps) post-unmarshal, and executes configuration business validation.
func Load(in []byte, out any, opts ...Option) error {
	if err := SetDefaults(out); err != nil {
		return err
	}

	if err := yaml4.Load(in, out, opts...); err != nil {
		return err
	}

	if err := SetDefaults(out); err != nil {
		return err
	}

	return Validate(out)
}

// Loader represents a v4-compatible streaming parser wrapper equipped with integrated structural validation.
type Loader struct {
	l *yaml4.Loader
}

// NewLoader creates, initializes, and configures a streaming Loader instance.
func NewLoader(r io.Reader, opts ...Option) (*Loader, error) {
	origLoader, err := yaml4.NewLoader(r, opts...)
	if err != nil {
		return nil, err
	}

	return &Loader{l: origLoader}, nil
}

// Load extracts the subsequent document tree from the stream, resolving default tags and checking evaluation constraints.
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
