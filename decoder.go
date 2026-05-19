package yaml

import (
	"io"

	yaml4 "go.yaml.in/yaml/v4"
)

// Decoder wraps an underlying YAML stream decoder to provide additional
// initialization and validation layers during decoding.
type Decoder struct {
	dec *yaml4.Decoder
}

// NewDecoder returns a new [Decoder] that reads from the provided [io.Reader].
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{dec: yaml4.NewDecoder(r)}
}

// KnownFields configures the decoder behavior regarding unknown keys.
// If enabled, the decoding process will return an error if a key in the YAML
// mappings cannot be matched to an exported field in the target structure.
func (dec *Decoder) KnownFields(enable bool) {
	dec.dec.KnownFields(enable)
}

// Decode reads the next YAML-encoded value from its input stream, applies
// default configuration values, validates the final structural state, and
// stores the result in the value pointed to by v.
func (dec *Decoder) Decode(v any) error {
	if err := SetDefaults(v); err != nil {
		return err
	}

	if err := dec.dec.Decode(v); err != nil {
		return err
	}

	if err := SetDefaults(v); err != nil {
		return err
	}

	return Validate(v)
}
