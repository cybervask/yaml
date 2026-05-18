package yaml

import (
	"io"

	yaml4 "go.yaml.in/yaml/v4"
)

// Encoder provides a classic streaming serializing platform maintained for backwards compatibility.
type Encoder struct {
	enc *yaml4.Encoder
}

// NewEncoder builds a classic stream processing Encoder mechanism.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{enc: yaml4.NewEncoder(w)}
}

// Encode converts and directs data entities directly out onto the established transport writer.
func (enc *Encoder) Encode(v any) error {
	return enc.enc.Encode(v)
}

// SetIndent changes the indentation sizing layout manually (accepts bounding spans 2-9).
func (enc *Encoder) SetIndent(spaces int) {
	enc.enc.SetIndent(spaces)
}

// CompactSeqIndent makes it so that '- ' is considered part of the indentation.
func (enc *Encoder) CompactSeqIndent() {
	enc.enc.CompactSeqIndent()
}

// DefaultSeqIndent makes it so that '- ' is not considered part of the indentation.
func (enc *Encoder) DefaultSeqIndent() {
	enc.enc.DefaultSeqIndent()
}

// Close gracefully tears down the data output encoders.
func (enc *Encoder) Close() error {
	return enc.enc.Close()
}
