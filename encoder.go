package yaml

import (
	"io"

	yaml4 "go.yaml.in/yaml/v4"
)

// Encoder wraps an underlying YAML stream encoder to serialize data entities
// directly to an output stream.
type Encoder struct {
	enc *yaml4.Encoder
}

// NewEncoder returns a new [Encoder] that writes to the provided [io.Writer].
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{enc: yaml4.NewEncoder(w)}
}

// Encode serializes the value pointed to by v into its YAML representation
// and writes it to the underlying output stream.
func (enc *Encoder) Encode(v any) error {
	return enc.enc.Encode(v)
}

// SetIndent configures the indentation spacing layout for the encoder.
// It typically accepts values ranging from 2 to 9 spaces.
func (enc *Encoder) SetIndent(spaces int) {
	enc.enc.SetIndent(spaces)
}

// CompactSeqIndent configures the encoder to treat the sequence block indicator
// ('- ') as part of the indentation block.
func (enc *Encoder) CompactSeqIndent() {
	enc.enc.CompactSeqIndent()
}

// DefaultSeqIndent configures the encoder to exclude the sequence block indicator
// ('- ') from the indentation block calculations.
func (enc *Encoder) DefaultSeqIndent() {
	enc.enc.DefaultSeqIndent()
}

// Close flushes any remaining buffered document data to the target stream
// and gracefully terminates the underlying encoder instance.
func (enc *Encoder) Close() error {
	return enc.enc.Close()
}
