package yaml

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	yaml4 "go.yaml.in/yaml/v4"
)

// HandleIncludeNode intercepts a YAML node during decoding.
// If the node has a !include tag, it reads the referenced file and unmarshals its contents into the target.
// Returns true if the node was handled, false otherwise.
func HandleIncludeNode(value *yaml4.Node, target any) (bool, error) {
	if value.Tag == "!include" {
		var filename string
		if err := value.Decode(&filename); err != nil {
			return true, fmt.Errorf("failed to decode include filename: %w", err)
		}

		includeData, err := os.ReadFile(filename)
		if err != nil {
			return true, fmt.Errorf("failed to read included file %s: %w", filename, err)
		}

		if err := yaml4.Unmarshal(includeData, target); err != nil {
			return true, fmt.Errorf("failed to unmarshal included file %s: %w", filename, err)
		}
		return true, nil
	}
	return false, nil
}

// resolveIncludes recursively traverses a YAML AST and replaces !include scalar nodes
// with the parsed content from external files.
//
// node is the current AST node being processed.
// currentDir is the base directory for resolving relative include paths.
// yamlPathPrefix is the dot-separated path to the current field in the configuration structure.
func resolveIncludes(node *yaml4.Node, currentDir, yamlPathPrefix string) error {
	if node.Tag == "!include" && node.Kind == yaml4.ScalarNode {
		originalPath := node.Value

		fullPath := originalPath
		if !filepath.IsAbs(originalPath) {
			fullPath = filepath.Join(currentDir, originalPath)
		}

		includeData, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read included file %s: %w", fullPath, err)
		}

		var fileNode yaml4.Node
		if err := yaml4.Unmarshal(includeData, &fileNode); err != nil {
			return fmt.Errorf("failed to parse included file %s: %w", fullPath, err)
		}

		if len(fileNode.Content) == 0 {
			return fmt.Errorf("included file %s is empty", fullPath)
		}
		actualContentNode := fileNode.Content[0]

		if yamlPathPrefix != "" {
			RegisterIncludePath(yamlPathPrefix, originalPath, fullPath)
		}

		newDir := filepath.Dir(fullPath)
		if err := resolveIncludes(actualContentNode, newDir, yamlPathPrefix); err != nil {
			return err
		}

		*node = *actualContentNode
		return nil
	}

	if node.Kind == yaml4.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			key := keyNode.Value

			nextPath := key
			if yamlPathPrefix != "" {
				nextPath = yamlPathPrefix + "." + key
			}

			if err := resolveIncludes(valNode, currentDir, nextPath); err != nil {
				return err
			}
		}
	} else {
		for _, child := range node.Content {
			if err := resolveIncludes(child, currentDir, yamlPathPrefix); err != nil {
				return err
			}
		}
	}

	return nil
}

// extractIncludes scans AST mapping nodes for fields marked with the include tag,
// writes their values to external files, and replaces them with !include directives in the parent tree.
//
// node is the current AST node being processed.
// currentStruct is the Go struct value corresponding to the current YAML mapping.
// baseDir is the base directory for resolving relative output paths.
// yamlPathPrefix is the dot-separated path to the current field in the configuration structure.
func extractIncludes(node *yaml4.Node, currentStruct any, baseDir, yamlPathPrefix string) error {
	if node.Kind != yaml4.MappingNode {
		for _, child := range node.Content {
			if err := extractIncludes(child, currentStruct, baseDir, yamlPathPrefix); err != nil {
				return err
			}
		}
		return nil
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		yamlKey := keyNode.Value

		fullYamlPath := yamlKey
		if yamlPathPrefix != "" {
			fullYamlPath = yamlPathPrefix + "." + yamlKey
		}

		isInclude, customPath := parseIncludeTag(currentStruct, yamlKey)

		if isInclude {
			filePath := customPath
			if filePath == "" {
				filePath = filepath.Join(baseDir, yamlKey+".yaml")
			}

			fullPath := filePath
			if !filepath.IsAbs(filePath) {
				fullPath = filepath.Join(baseDir, filePath)
			}

			RegisterIncludePath(fullYamlPath, filePath, fullPath)

			subData, err := yaml4.Marshal(valNode)
			if err != nil {
				return fmt.Errorf("failed to marshal include node %s: %w", yamlKey, err)
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", fullPath, err)
			}

			if err := os.WriteFile(fullPath, subData, 0o644); err != nil {
				return fmt.Errorf("failed to write included file %s: %w", fullPath, err)
			}

			valNode.Kind = yaml4.ScalarNode
			valNode.Tag = "!include"
			valNode.Value = filePath
			valNode.Content = nil
		} else if valNode.Kind == yaml4.MappingNode {
			nestedStruct := getNestedStruct(currentStruct, yamlKey)
			if nestedStruct != nil {
				if err := extractIncludes(valNode, nestedStruct, baseDir, fullYamlPath); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// parseIncludeTag checks if a struct field has an include directive in its YAML tag.
// Returns true if the field should be extracted to a separate file, and the custom path if specified.
func parseIncludeTag(s any, yamlKey string) (ok bool, path string) {
	v := reflect.Indirect(reflect.ValueOf(s))
	if v.Kind() != reflect.Struct {
		return false, ""
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("yaml")
		parts := strings.Split(tag, ",")
		if len(parts) > 0 && parts[0] == yamlKey {
			for _, part := range parts {
				if part == "include" {
					return true, ""
				}
				if strings.HasPrefix(part, "include:") {
					return true, strings.TrimPrefix(part, "include:")
				}
			}
		}
	}
	return false, ""
}

// getNestedStruct returns the nested struct value corresponding to a YAML key.
// Returns nil if the key does not map to a struct field.
func getNestedStruct(s interface{}, yamlKey string) interface{} {
	v := reflect.Indirect(reflect.ValueOf(s))
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("yaml")
		parts := strings.Split(tag, ",")
		if len(parts) > 0 && parts[0] == yamlKey {
			fieldVal := v.Field(i)
			if fieldVal.CanInterface() {
				return fieldVal.Interface()
			}
		}
	}
	return nil
}
