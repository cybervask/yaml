package yaml

import (
	"io"

	yaml4 "go.yaml.in/yaml/v4"
)

// --- MODERN V4 RECOMMENDED API ---

// Dump encodes the provided interface runtime values into an exported slice of formatted YAML bytes.
func Dump(in any, opts ...Option) ([]byte, error) {
	return yaml4.Dump(in, opts...)
}

// Dumper represents a v4-compatible streaming document pipeline.
type Dumper struct {
	d *yaml4.Dumper
}

// NewDumper configures and constructs a live output Dumper instance.
func NewDumper(w io.Writer, opts ...Option) (*Dumper, error) {
	origDumper, err := yaml4.NewDumper(w, opts...)
	if err != nil {
		return nil, err
	}
	return &Dumper{d: origDumper}, nil
}

// Dump writes the serializable application target straight to the outgoing destination pipeline.
func (dumper *Dumper) Dump(v any) error {
	return dumper.d.Dump(v)
}

// Close flushes lingering data blocks and releases internal document buffers.
func (dumper *Dumper) Close() error {
	return dumper.d.Close()
}
