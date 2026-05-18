package yaml

import (
	yaml4 "go.yaml.in/yaml/v4"
)

// --- LEGACY COMPATIBILITY LAYER (DEPRECATED IN V4) ---

// Unmarshal analyzes incoming YAML payloads, performing standard parsing supplemented by automated default mappings and rules validation.
func Unmarshal(in []byte, out any) error {
	if err := SetDefaults(out); err != nil {
		return err
	}

	if err := yaml4.Unmarshal(in, out); err != nil {
		return err
	}

	if err := SetDefaults(out); err != nil {
		return err
	}

	return Validate(out)
}
