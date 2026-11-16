package yaml

import (
	"fmt"
	"io"

	yaml4 "go.yaml.in/yaml/v4"
)

// Dump serializes the provided value into a YAML document byte slice.
// It is a pure serialization entry point: no include processing happens here.
// A field carrying the include tag flag ("...,include") is rejected by the
// underlying marshaller with an error. To dump a config loaded with !include
// directives — writing the include files back and emitting !include markers —
// use [DumpWithInclude]. All variadic formatting options from the underlying
// library are preserved.
func Dump(in any, opts ...Option) ([]byte, error) {
	var node yaml4.Node

	// Marshal the Go structure into an intermediate YAML slice
	data, err := yaml4.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal target to intermediate YAML: %w", err)
	}

	// Unmarshal into an AST node tree for safe structural transformation
	if err := yaml4.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse intermediate YAML to AST: %w", err)
	}

	// Generate final document bytes for the main config file, applying format options
	finalData, err := yaml4.Dump(&node, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal final AST: %w", err)
	}

	return finalData, nil
}

// Marshal serializes the provided value into a YAML document byte slice.
// It serves as a direct proxy wrapper around [Dump] without passing extra options.
func Marshal(in any) ([]byte, error) {
	return Dump(in)
}

// Dumper provides a streaming pipeline interface to encode and write documents
// sequentially to an underlying data stream.
type Dumper struct {
	d *yaml4.Dumper
}

// NewDumper returns a new [Dumper] initialized to write to the provided [io.Writer]
// using the specified configuration options.
func NewDumper(w io.Writer, opts ...Option) (*Dumper, error) {
	origDumper, err := yaml4.NewDumper(w, opts...)
	if err != nil {
		return nil, err
	}
	return &Dumper{d: origDumper}, nil
}

// Dump writes the YAML encoding of v to the internal stream writer.
//
// Note: include processing is not performed here; use [DumpWithInclude] for
// configs loaded with !include directives.
func (dumper *Dumper) Dump(v any) error {
	return dumper.d.Dump(v)
}

// Close flushes any remaining buffered document segments to the target stream
// and releases all internal tracking resources.
func (dumper *Dumper) Close() error {
	return dumper.d.Close()
}
