package yaml

import (
	yaml4 "go.yaml.in/yaml/v4"
)

// --- LEGACY COMPATIBILITY LAYER (DEPRECATED IN V4) ---

// Marshal packages raw application models into clean, standard output bytes formatting.
func Marshal(in any) ([]byte, error) {
	return yaml4.Marshal(in)
}
