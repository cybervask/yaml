package yaml

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

// Help prints the beautifully formatted configuration layout documentation directly to os.Stderr.
func Help(v any) {
	fmt.Fprint(os.Stderr, HelpStr(v))
}

// HelpStr recursively analyzes the configuration structure via reflection and generates
// an aligned human-readable documentation string containing fields, descriptions, env overrides, defaults, and validation rules.
func HelpStr(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		return ""
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return "yaml configuration error: target configuration object must be a struct\n"
	}

	var sb strings.Builder
	sb.WriteString("yaml configuration schema documentation:\n\n")

	type helpItem struct {
		yamlPath string
		desc     string
		meta     string // For storing aggregated env, default, and validate rule annotations
	}

	var items []helpItem
	var maxPathLen int

	// Define recursive structural crawl lambda closure
	var walk func(reflect.Type, int, string)
	walk = func(currentType reflect.Type, indent int, prefix string) {
		if currentType.Kind() == reflect.Ptr {
			currentType = currentType.Elem()
		}
		if currentType.Kind() != reflect.Struct {
			return
		}

		for i := 0; i < currentType.NumField(); i++ {
			field := currentType.Field(i)

			// Bypass internal split-configuration tracking markers safely
			if field.Anonymous && field.Type == reflect.TypeOf(Includer{}) {
				continue
			}

			// Get yaml key name or fallback to lowercase field name
			yamlName := field.Tag.Get("yaml")
			if yamlName == "" {
				yamlName = strings.ToLower(field.Name)
			} else {
				// Correctly extract the first element as the actual key name
				yamlParts := strings.Split(yamlName, ",")
				yamlName = yamlParts[0]
			}

			// Generate balanced hierarchical spaces spacing indentation tracks (e.g. "  tls:")
			displayPath := strings.Repeat("  ", indent) + yamlName + ":"
			if len(displayPath) > maxPathLen {
				maxPathLen = len(displayPath)
			}

			desc := field.Tag.Get("description")

			// Assemble context parameter details dynamically
			var metaParts []string
			if env := field.Tag.Get("env"); env != "" {
				metaParts = append(metaParts, fmt.Sprintf("env: %s", env))
			}
			if def := field.Tag.Get("default"); def != "" {
				metaParts = append(metaParts, fmt.Sprintf("default: %s", def))
			}
			if val := field.Tag.Get("validate"); val != "" {
				metaParts = append(metaParts, fmt.Sprintf("validate: [%s]", val))
			}

			meta := ""
			if len(metaParts) > 0 {
				meta = "(" + strings.Join(metaParts, ", ") + ")"
			}

			items = append(items, helpItem{
				yamlPath: displayPath,
				desc:     desc,
				meta:     meta,
			})

			// Evaluate inner properties depth cascades
			fieldType := field.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				walk(fieldType, indent+1, prefix+yamlName+".")
			}
		}
	}

	walk(t, 0, "")

	// Format layout output strings aligning lines cleanly by the longest property key span width bounds
	formatStr := fmt.Sprintf("%%-%ds %%s %%s\n", maxPathLen+2)
	for _, item := range items {
		sb.WriteString(fmt.Sprintf(formatStr, item.yamlPath, item.desc, item.meta))
	}

	return sb.String()
}
