package yaml

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

// Help writes a beautifully formatted representation of the configuration layout
// schema directly to [os.Stderr].
func Help(v any) {
	fmt.Fprint(os.Stderr, HelpStr(v))
}

// HelpStr recursively analyzes the provided configuration structure via reflection
// and generates an aligned, human-readable documentation string.
//
// The output contains the structural hierarchy of fields, descriptive documentation,
// environment variable overrides, source-priority arbitration, defaults, and
// associated validation constraint rules.
func HelpStr(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		return ""
	}
	if t.Kind() == reflect.Pointer {
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
		meta     string // Stores aggregated env, priority, default, and validate rule annotations
	}

	var items []helpItem
	var maxPathLen int

	// visited tracks struct types on the current documentation path: a
	// self-referential type stops the recursion silently, while a non-cyclic
	// type repeated in two branches is documented both times.
	visited := make(map[reflect.Type]struct{})

	// Define recursive structural crawl lambda closure
	var walk func(reflect.Type, int)
	walk = func(currentType reflect.Type, indent int) {
		if currentType.Kind() == reflect.Pointer {
			currentType = currentType.Elem()
		}
		if currentType.Kind() != reflect.Struct {
			return
		}
		if _, seen := visited[currentType]; seen {
			return
		}

		visited[currentType] = struct{}{}
		defer delete(visited, currentType)

		for i := 0; i < currentType.NumField(); i++ {
			field := currentType.Field(i)

			// Skip unexported fields: yaml4 never decodes them, so documenting
			// them would be misleading (parity with dump_include.go).
			if !field.IsExported() {
				continue
			}

			// Extract the YAML key name or fall back to the lowercase field name
			yamlName := field.Tag.Get("yaml")
			inline := false
			if yamlName == "" {
				yamlName = strings.ToLower(field.Name)
			} else {
				// Extract the first element as the actual key name
				yamlParts := strings.Split(yamlName, ",")
				yamlName = yamlParts[0]
				for _, opt := range yamlParts[1:] {
					if opt == "inline" {
						inline = true
					}
				}
			}

			// Skip fields the decoder never reads: a "-" tag excludes the
			// field from YAML entirely, so documenting it would be misleading
			// (same rationale as the unexported skip above).
			if yamlName == "-" {
				continue
			}

			// Evaluate inner properties depth cascades
			fieldType := field.Type
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}

			// An inline struct field unfolds into the parent mapping in YAML,
			// so its nested fields are documented at the parent's level
			// instead of emitting a keyless line. An inline map has dynamic
			// keys — there is nothing static to document.
			if inline {
				if fieldType.Kind() == reflect.Struct {
					walk(fieldType, indent)
				}

				continue
			}

			// Generate structural spacing indentation tracking lines (e.g. "  tls:")
			displayPath := strings.Repeat("  ", indent) + yamlName + ":"
			if w := displayWidth(displayPath); w > maxPathLen {
				maxPathLen = w
			}

			desc := field.Tag.Get("description")

			// Assemble context parameter details dynamically
			var metaParts []string
			if env := field.Tag.Get("env"); env != "" {
				metaParts = append(metaParts, fmt.Sprintf("env: %s", env))
			}
			if pr := field.Tag.Get("priority"); pr != "" {
				metaParts = append(metaParts, fmt.Sprintf("priority: %s", pr))
			}
			if def := field.Tag.Get("default"); def != "" {
				metaParts = append(metaParts, fmt.Sprintf("default: %s", def))
			}
			if val := field.Tag.Get("validate"); val != "" {
				// The service break token is engine plumbing, not a schema
				// constraint: it stays out of the documented rule list while
				// every other segment keeps its original formatting. A tag
				// reduced to nothing by the strip carries no constraint at
				// all and is omitted entirely.
				if stripped := stripBreakToken(val); stripped != "" {
					metaParts = append(metaParts, fmt.Sprintf("validate: [%s]", stripped))
				}
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

			switch fieldType.Kind() {
			case reflect.Struct:
				walk(fieldType, indent+1)
			case reflect.Slice, reflect.Array, reflect.Map:
				// Document configurable struct elements of containers
				// (slice/array/map); scalar elements carry no nested schema.
				elem := fieldType.Elem()
				if elem.Kind() == reflect.Pointer {
					elem = elem.Elem()
				}
				if elem.Kind() == reflect.Struct {
					walk(elem, indent+1)
				}
			}
		}
	}

	walk(t, 0)

	// Format layout output strings aligning lines cleanly by the longest
	// property key span: the path column is padded by terminal display width
	// (not bytes), so single-width non-ASCII keys keep the description column
	// aligned; trailing whitespace is trimmed from every line.
	for _, item := range items {
		line := item.yamlPath +
			strings.Repeat(" ", maxPathLen+2-displayWidth(item.yamlPath)) +
			" " + item.desc + " " + item.meta
		sb.WriteString(strings.TrimRight(line, " "))
		sb.WriteByte('\n')
	}

	return sb.String()
}

// displayWidth returns the terminal display width of s for the help column
// layout: East Asian wide/fullwidth runes count as two columns, every other
// rune as one. Byte counting underpadded single-width non-ASCII keys (each
// Cyrillic letter is two bytes) and misaligned the description column.
func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runeDisplayWidth(r)
	}

	return width
}

// runeDisplayWidth approximates the terminal width of r: the common East
// Asian wide/fullwidth blocks occupy two terminal columns, every other rune
// occupies one. Combining marks are not special-cased and count as one
// column — they do not occur in the configuration keys this rendering targets.
func runeDisplayWidth(r rune) int {
	switch {
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r == 0x2329 || r == 0x232A,   // CJK angle brackets
		r >= 0x2E80 && r <= 0x303E,   // CJK radicals and symbols
		r >= 0x3041 && r <= 0x33FF,   // Hiragana through CJK compatibility
		r >= 0x3400 && r <= 0x4DBF,   // CJK unified ideographs extension A
		r >= 0x4E00 && r <= 0x9FFF,   // CJK unified ideographs
		r >= 0xA000 && r <= 0xA4CF,   // Yi syllables and radicals
		r >= 0xAC00 && r <= 0xD7A3,   // Hangul syllables
		r >= 0xF900 && r <= 0xFAFF,   // CJK compatibility ideographs
		r >= 0xFE30 && r <= 0xFE4F,   // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFF60,   // fullwidth forms
		r >= 0xFFE0 && r <= 0xFFE6,   // fullwidth sign forms
		r >= 0x20000 && r <= 0x3FFFD: // CJK unified ideographs extension B+
		return 2
	}

	return 1
}

// stripBreakToken removes the standalone break service token from a raw
// validate tag string so the help rendering never documents engine plumbing.
// The scan is quote-aware and mirrors the validate tag tokenizer: commas,
// spaces and tabs outside quoted segments separate the rule tokens, so the
// word inside a parameter value ("choice=break", a required_if atom
// "mode==break") is data and stays untouched.
//
// A tag without a standalone break token is returned bit-for-bit. Otherwise
// every break segment is cut out together with one adjacent delimiter
// (preferring the comma after it, falling back to the one before it), while
// every remaining segment keeps its original formatting — the gluing never
// rewrites the segments that stay.
func stripBreakToken(tag string) string {
	if !strings.Contains(tag, "break") {
		return tag
	}

	// token is a candidate break segment: the span covers non-delimiter
	// characters only (quotes included — a quoted "'a,break'" never equals
	// the bare token).
	type span struct{ start, end int }

	var cuts []span

	inQuotes := false
	quoteChar := byte(0)
	segStart := -1

	for i := 0; i < len(tag); i++ {
		ch := tag[i]

		if inQuotes {
			if ch == quoteChar {
				inQuotes = false
				quoteChar = 0
			}

			continue
		}

		switch {
		case ch == '\'' || ch == '"':
			inQuotes = true
			quoteChar = ch
			if segStart < 0 {
				segStart = i
			}
		case ch == ',' || ch == ' ' || ch == '\t':
			if segStart >= 0 {
				if strings.TrimSpace(tag[segStart:i]) == "break" {
					cuts = append(cuts, span{start: segStart, end: i})
				}
				segStart = -1
			}
		default:
			if segStart < 0 {
				segStart = i
			}
		}
	}

	if segStart >= 0 && strings.TrimSpace(tag[segStart:]) == "break" {
		cuts = append(cuts, span{start: segStart, end: len(tag)})
	}

	if len(cuts) == 0 {
		return tag
	}

	// cutOut widens one break segment by a single adjacent delimiter so the
	// remaining segments do not fuse: the comma after the token wins, the
	// one before it is the fallback. Leading spaces of the break token
	// belong to its own delimiter run and go away with it; the neighbors'
	// formatting is never rewritten.
	cutOut := func(seg span) (int, int) {
		start, end := seg.start, seg.end

		j := seg.end
		for j < len(tag) && (tag[j] == ' ' || tag[j] == '\t') {
			j++
		}
		if j < len(tag) && tag[j] == ',' {
			return start, j + 1
		}

		j = seg.start
		for j > 0 && (tag[j-1] == ' ' || tag[j-1] == '\t') {
			j--
		}
		if j > 0 && tag[j-1] == ',' {
			start = j - 1
		}

		return start, end
	}

	for i := len(cuts) - 1; i >= 0; i-- {
		start, end := cutOut(cuts[i])
		tag = tag[:start] + tag[end:]
	}

	return tag
}
