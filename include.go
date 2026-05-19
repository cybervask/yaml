package yaml

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	yaml4 "go.yaml.in/yaml/v4"
)

// HandleIncludeNode performs a low-level interception of a YAML node.
// If the node exposes an explicit `!include` marker tag, it reads the external
// storage file path and parses its contents directly into the target object.
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

		// Parse the include file recursively on top of pre-existing defaults
		if err := yaml4.Unmarshal(includeData, target); err != nil {
			return true, fmt.Errorf("failed to unmarshal included file %s: %w", filename, err)
		}
		return true, nil
	}
	return false, nil
}

// resolveIncludes recursively traverses YAML nodes and replaces `!include` scalar
// tokens with the corresponding Abstract Syntax Tree (AST) generated from external files.
func resolveIncludes(node *yaml4.Node, currentDir string) error {
	// Step 1: Check whether the current node represents an include token
	if node.Tag == "!include" && node.Kind == yaml4.ScalarNode {
		filename := node.Value

		// Calculate the correct absolute path relative to the file that triggered the include directive
		fullPath := filename
		if !filepath.IsAbs(filename) {
			fullPath = filepath.Join(currentDir, filename)
		}

		includeData, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read included file %s: %w", fullPath, err)
		}

		var fileNode yaml4.Node
		if err := yaml4.Unmarshal(includeData, &fileNode); err != nil {
			return fmt.Errorf("failed to parse included file %s: %w", fullPath, err)
		}

		// The root node of a parsed file is always a DocumentNode. Its actual payload resides in Content[0].
		if len(fileNode.Content) == 0 {
			return fmt.Errorf("included file %s is empty", fullPath)
		}
		actualContentNode := fileNode.Content[0]

		// Step 2: Recursively process any nested include declarations inside the newly loaded file.
		// The base directory shifts to the parent directory of this newly read file.
		newDir := filepath.Dir(fullPath)
		if err := resolveIncludes(actualContentNode, newDir); err != nil {
			return err
		}

		// Step 3: Replace the current `!include` node with the expanded payload tree
		*node = *actualContentNode
		return nil
	}

	// Step 4: For standard nodes, recursively traverse all child elements (struct fields, sequence elements)
	for _, child := range node.Content {
		if err := resolveIncludes(child, currentDir); err != nil {
			return err
		}
	}

	return nil
}

// extractIncludes scans the AST mapping nodes to locate fields annotated with the include keyword,
// writes their values to individual configuration files, and drops an `!include` reference in the parent tree.
func extractIncludes(node *yaml4.Node, currentStruct any, baseDir string) error {
	if node.Kind != yaml4.MappingNode {
		for _, child := range node.Content {
			if err := extractIncludes(child, currentStruct, baseDir); err != nil {
				return err
			}
		}
		return nil
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		yamlKey := keyNode.Value

		isInclude, customPath := parseIncludeTag(currentStruct, yamlKey)

		if isInclude {
			filePath := customPath
			if filePath == "" {
				filePath = fmt.Sprintf("configs/%s.yaml", yamlKey)
			}

			fullPath := filePath
			if !filepath.IsAbs(filePath) {
				fullPath = filepath.Join(baseDir, filePath)
			}

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
				if err := extractIncludes(valNode, nestedStruct, baseDir); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// parseIncludeTag parses struct tags via reflection to determine if a specific key requires file extraction.
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

// getNestedStruct resolves and returns the structural interface value matching a specific YAML key.
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
