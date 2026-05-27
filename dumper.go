package yaml

import (
	"fmt"
	"io"

	yaml4 "go.yaml.in/yaml/v4"
)

// Dump serializes the provided value into a YAML document byte slice.
// It automatically detects fields marked with the structural include tag,
// extracts their contents into separate external files, and places matching
// `!include` markers in the primary document. All variadic formatting options
// from the underlying library are preserved.
func Dump(in any, opts ...Option) (out []byte, err error) {
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

	// Isolate tagged fields and write them to external target files
	if err := extractIncludes(&node, in, ".", ""); err != nil {
		return nil, err
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
// Note: Consider augmenting this method if stream-encoded structures
// also require external file extraction processing similar to [Dump].
func (dumper *Dumper) Dump(v any) error {
	return dumper.d.Dump(v)
}

// Close flushes any remaining buffered document segments to the target stream
// and releases all internal tracking resources.
func (dumper *Dumper) Close() error {
	return dumper.d.Close()
}
