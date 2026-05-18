package yaml

import (
	"io"

	yaml4 "go.yaml.in/yaml/v4"
)

// Decoder provides a classic streaming implementation for legacy compatibility layers.
type Decoder struct {
	dec *yaml4.Decoder
}

// NewDecoder wraps standard stream reader objects into a legacy Decoder engine.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{dec: yaml4.NewDecoder(r)}
}

// KnownFields ensures that keys in decoded mappings must map exactly
// to exported fields in the target unmarshaling structural model.
func (dec *Decoder) KnownFields(enable bool) {
	dec.dec.KnownFields(enable)
}

// Decode extracts configuration payload data segments while dynamically processing defaults and enforcing validation rules.
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
