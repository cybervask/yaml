package yaml

import (
	"fmt"
	"os"

	yaml4 "go.yaml.in/yaml/v4"
)

// Includer — это маркерная структура.
// Добавьте её в любую свою структуру анонимно, чтобы включить поддержку !include.
type Includer struct{}

// HandleIncludeNode производит низкоуровневый перехват узла YAML.
// Если узел является тегом !include, он считывает файл и парсит его в target.
func HandleIncludeNode(value *yaml4.Node, target interface{}) (bool, error) {
	if value.Tag == "!include" {
		var filename string
		if err := value.Decode(&filename); err != nil {
			return true, fmt.Errorf("failed to decode include filename: %w", err)
		}

		includeData, err := os.ReadFile(filename)
		if err != nil {
			return true, fmt.Errorf("failed to read included file %s: %w", filename, err)
		}

		// Парсим файл инклуда рекурсивно поверх уже существующих дефолтов
		if err := yaml4.Unmarshal(includeData, target); err != nil {
			return true, fmt.Errorf("failed to unmarshal included file %s: %w", filename, err)
		}
		return true, nil
	}
	return false, nil
}

// Include serves as a generic field type container enabling clean split-configuration setups
// by intercepting targets with the explicit custom !include tag specification.
type Include[T any] struct {
	Value T
}

// UnmarshalYAML hijacks the standard decoding workflow from go.yaml.in/yaml/v4.
// If an incoming sequence element exposes an explicit !include marker tag, the engine fetches,
// interprets, and maps the referenced external storage file path.
func (i *Include[T]) UnmarshalYAML(value *yaml4.Node) error {
	if err := SetDefaults(&i.Value); err != nil {
		return fmt.Errorf("failed to set defaults for included type: %w", err)
	}

	if value.Tag == "!include" {
		var filename string
		if err := value.Decode(&filename); err != nil {
			return fmt.Errorf("failed to decode include filename: %w", err)
		}

		includeData, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read included file %s: %w", filename, err)
		}

		if err := yaml4.Unmarshal(includeData, &i.Value); err != nil {
			return fmt.Errorf("failed to unmarshal included file %s: %w", filename, err)
		}
		return nil
	}

	if err := value.Decode(&i.Value); err != nil {
		return err
	}
	return nil
}

// MarshalYAML enforces transparent structure encoding layouts by bypassing the outer Include metadata field wrapper.
func (i *Include[T]) MarshalYAML() (any, error) {
	return i.Value, nil
}

// HandleInclude проверяет, является ли узел тегом !include.
// Если да, он считывает файл и десериализует его в target.
// Возвращает (true, nil), если инклуд обработан, и (false, nil), если это обычный блок.
func HandleInclude(value *yaml4.Node, target any) (bool, error) {
	if value.Tag == "!include" {
		var filename string
		if err := value.Decode(&filename); err != nil {
			return true, fmt.Errorf("failed to decode include filename: %w", err)
		}

		includeData, err := os.ReadFile(filename)
		if err != nil {
			return true, fmt.Errorf("failed to read included file %s: %w", filename, err)
		}

		// Парсим файл инклуда поверх дефолтных значений
		if err := yaml4.Unmarshal(includeData, target); err != nil {
			return true, fmt.Errorf("failed to unmarshal included file %s: %w", filename, err)
		}

		return true, nil
	}

	return false, nil
}
