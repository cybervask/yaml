package yaml

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// durationType caches the reflect.Type of time.Duration for efficient type matching metrics.
var durationType = reflect.TypeOf(time.Duration(0))

// SetDefaults recursively walks through the exported fields of a structure pointer
// and populates uninitialized zero-value fields with the data defined in their `default` tags.
// Unexported fields are skipped entirely: they are outside the configuration
// surface (neither the YAML decoder nor the environment can address them), so
// their default/env/priority tags are neither applied nor reported.
//
// System environment variables configured via the `env` tag are evaluated during
// execution and take absolute precedence over standard tag defaults. A
// priority:"value" tag opts out of that precedence: the value source (a default
// tag or an already held field value) outranks a set environment variable,
// while priority:"env" or an absent tag keeps it.
//
// Slice-typed fields take their default as a comma-separated element list
// converted per element with the scalar matrix of setScalarFromString
// (string elements are trimmed); a wholly empty list leaves the slice at
// its zero value.
//
// It returns an error if a design conflict is detected where both 'default' and
// a requiredness rule ('required' or its legacy alias 'not_empty') are declared
// on the same structure field, and for a malformed priority tag: an unknown
// value, a declaration without a non-empty `env` tag on the same field, or a
// declaration on a non-scalar (struct, slice, map) field, where arbitration
// has no single value to act on.
func SetDefaults(ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return nil
	}

	return setDefaultsValue(v.Elem())
}

// setScalarFromString assigns a textual configuration value to a scalar field,
// following the shared conversion matrix of the defaults engine: duration
// literals are parsed with time.ParseDuration, booleans and primitive numbers
// with the strconv family, strings are copied verbatim. Values that fail to
// parse (except durations) or that target unsupported kinds leave the field
// untouched. It returns an error only for invalid duration literals.
//
// This legacy silent-skip behavior serves the defaults engine (its `default`
// tags and its inline env evaluation); the dedicated environment override
// pass (walkEnvOverrides) uses the strict matrix of
// setScalarFromStringStrict instead.
func setScalarFromString(fieldVal reflect.Value, text, fieldName string) error {
	if fieldVal.Type() == durationType {
		d, err := time.ParseDuration(text)
		if err != nil {
			return fmt.Errorf("invalid duration %q for field %s: %w", text, fieldName, err)
		}

		fieldVal.Set(reflect.ValueOf(d))
		return nil
	}

	switch fieldVal.Kind() {
	case reflect.String:
		fieldVal.SetString(text)

	case reflect.Bool:
		b, err := strconv.ParseBool(text)
		if err == nil {
			fieldVal.SetBool(b)
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(text, 10, 64)
		if err == nil {
			fieldVal.SetInt(n)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(text, 10, 64)
		if err == nil {
			fieldVal.SetUint(n)
		}

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(text, 64)
		if err == nil {
			fieldVal.SetFloat(f)
		}

	default:
		// Unsupported kind for default/env injection: leave the field untouched.
	}

	return nil
}

// isScalarDefaultElem reports whether a slice element type participates in
// the comma-separated slice default matrix: the scalar kinds of
// setScalarFromString (strings, bools, primitive numbers, durations).
// A slice of any other element kind with a default tag is left untouched.
func isScalarDefaultElem(t reflect.Type) bool {
	if t == durationType {
		return true
	}

	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}

	return false
}

// setScalarFromStringStrict assigns a textual environment literal to a scalar
// field with the strict conversion matrix of the environment override pass:
// duration literals are parsed with time.ParseDuration, booleans and primitive
// numbers with the strconv family (floats with strconv.ParseFloat), strings
// are copied verbatim. A failed conversion of a bool, int, uint, float or
// duration literal is returned as an error naming the field and the offending
// value; unsupported kinds still leave the field untouched, because there is
// no literal-to-kind conversion to report.
func setScalarFromStringStrict(fieldVal reflect.Value, text, fieldName string) error {
	if fieldVal.Type() == durationType {
		d, err := time.ParseDuration(text)
		if err != nil {
			return fmt.Errorf("invalid duration %q for field %s: %w", text, fieldName, err)
		}

		fieldVal.Set(reflect.ValueOf(d))
		return nil
	}

	switch fieldVal.Kind() {
	case reflect.String:
		fieldVal.SetString(text)

	case reflect.Bool:
		b, err := strconv.ParseBool(text)
		if err != nil {
			return fmt.Errorf("invalid bool %q for field %s: %w", text, fieldName, err)
		}

		fieldVal.SetBool(b)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int %q for field %s: %w", text, fieldName, err)
		}

		fieldVal.SetInt(n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid uint %q for field %s: %w", text, fieldName, err)
		}

		fieldVal.SetUint(n)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return fmt.Errorf("invalid float %q for field %s: %w", text, fieldName, err)
		}

		fieldVal.SetFloat(f)

	default:
		// Unsupported kind for environment injection: there is no
		// literal-to-kind conversion, so the field is left untouched.
	}

	return nil
}

// setDefaultsValue performs the underlying recursive assignment of defaults and environment overrides.
//
//nolint:gocyclo // Reason: heavy reflection type-switch matrix structure requires high complexity.
func setDefaultsValue(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldVal := v.Field(i)
			fieldType := t.Field(i)

			// Unexported fields are outside the configuration surface: neither
			// the YAML decoder nor the environment can address them, and
			// reflection cannot write them (a Set* call would panic). Their
			// tags are not applied and not reported.
			if !fieldType.IsExported() {
				continue
			}

			validateTag, hasValidate := fieldType.Tag.Lookup("validate")
			defaultValStr, hasDefault := fieldType.Tag.Lookup("default")

			if hasDefault && hasValidate {
				// Only the required/not_empty presence matters here; a duplicate
				// required_if is a validate-pass configuration mistake and is
				// reported there, where the field path is known.
				tagRules, _ := parseValidateTag(validateTag)
				if _, hasRequired := tagRules["required"]; hasRequired {
					return fmt.Errorf("field %s is invalid: 'default' and 'required' are mutually exclusive", fieldType.Name)
				}
				if _, hasNotEmpty := tagRules["not_empty"]; hasNotEmpty {
					return fmt.Errorf("field %s is invalid: 'default' and 'not_empty' are mutually exclusive", fieldType.Name)
				}
			}

			// The priority tag is a static declaration like default+required:
			// misconfiguration is reported for every field kind regardless of
			// field values and environment state.
			priorityTag, hasPriority := fieldType.Tag.Lookup("priority")
			if hasPriority {
				if priorityTag != "value" && priorityTag != "env" {
					return fmt.Errorf("field %s is invalid: unknown 'priority' tag value %q (must be \"value\" or \"env\")", fieldType.Name, priorityTag)
				}
				if fieldType.Tag.Get("env") == "" {
					return fmt.Errorf("field %s is invalid: 'priority' tag requires 'env' tag", fieldType.Name)
				}
				// The arbitration needs one settable scalar target: on struct,
				// slice and map fields there is no such value, so honoring the
				// tag would be a silent no-op.
				switch fieldVal.Kind() {
				case reflect.Struct, reflect.Slice, reflect.Map:
					return fmt.Errorf("field %s is invalid: 'priority' tag is only supported on scalar fields", fieldType.Name)
				}
			}

			// Legacy generic Value property fallback: only struct-typed Value
			// fields are transparent wrappers whose payload the engines descend
			// into. A scalar field that merely happens to be named Value is an
			// ordinary field and keeps its own default/env handling.
			if fieldType.Name == "Value" && fieldVal.Kind() == reflect.Struct {
				if err := setDefaultsValue(fieldVal); err != nil {
					return err
				}
				continue
			}

			if fieldVal.Kind() == reflect.Slice {
				if hasDefault && fieldVal.IsZero() {
					elements := strings.Split(defaultValStr, ",")
					// A wholly empty list ("default:\"\"") is an absent value,
					// not a single empty element: the slice stays at its zero
					// value instead of materializing [""].
					hasContent := false
					for _, elem := range elements {
						if strings.TrimSpace(elem) != "" {
							hasContent = true
							break
						}
					}
					if hasContent && isScalarDefaultElem(fieldVal.Type().Elem()) {
						sliceValues := reflect.MakeSlice(fieldVal.Type(), len(elements), len(elements))
						for idx, elem := range elements {
							if err := setScalarFromString(sliceValues.Index(idx), strings.TrimSpace(elem), fieldType.Name); err != nil {
								return err
							}
						}
						fieldVal.Set(sliceValues)
					}
				}
				if err := setDefaultsValue(fieldVal); err != nil {
					return err
				}
				continue
			}

			if fieldVal.Kind() == reflect.Struct || fieldVal.Kind() == reflect.Map {
				if err := setDefaultsValue(fieldVal); err != nil {
					return err
				}
				continue
			}

			envVarName := fieldType.Tag.Get("env")
			var targetValStr string
			var hasValueToSet bool

			// priority:"value" makes the declared value source outrank the
			// environment: when a default tag is declared, the inline
			// selection skips env entirely and the default wins. After decode
			// the same arbitration is enforced against YAML content by
			// walkEnvOverrides.
			if envVarName != "" && !(priorityTag == "value" && hasDefault) {
				if envVal, isSet := os.LookupEnv(envVarName); isSet && envVal != "" {
					targetValStr = envVal
					hasValueToSet = true
				}
			}

			if !hasValueToSet && hasDefault {
				targetValStr = defaultValStr
				hasValueToSet = true
			}

			isEnvOverride := envVarName != "" && os.Getenv(envVarName) != ""

			// Under priority:"value" a non-zero field already holds its value
			// (decoded YAML, an applied default or a pre-filled standalone
			// value), so the environment force no longer applies.
			if hasValueToSet && (fieldVal.IsZero() || (isEnvOverride && priorityTag != "value")) {
				if err := setScalarFromString(fieldVal, targetValStr, fieldType.Name); err != nil {
					return err
				}
			}
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			element := v.Index(i)
			if element.Kind() == reflect.Pointer {
				if !element.IsNil() {
					if err := setDefaultsValue(element.Elem()); err != nil {
						return err
					}
				}
				continue
			}
			if element.Kind() == reflect.Struct {
				copyElem := reflect.New(element.Type()).Elem()
				copyElem.Set(element)
				if err := setDefaultsValue(copyElem); err != nil {
					return err
				}
				v.Index(i).Set(copyElem)
			}
		}

	case reflect.Map:
		for _, key := range v.MapKeys() {
			element := v.MapIndex(key)
			if element.Kind() == reflect.Pointer {
				if !element.IsNil() {
					if err := setDefaultsValue(element.Elem()); err != nil {
						return err
					}
				}
				continue
			}
			if element.Kind() == reflect.Struct {
				copyElem := reflect.New(element.Type()).Elem()
				copyElem.Set(element)
				if err := setDefaultsValue(copyElem); err != nil {
					return err
				}
				v.SetMapIndex(key, copyElem)
			}
		}
	}
	return nil
}

// applyEnvOverrides recursively walks the exported fields of a structure
// pointer and assigns values from non-empty OS environment variables declared
// via `env` tags. Unlike the defaults engine, the assignment is unconditional
// with respect to the current field value: an environment variable configured
// on a field takes absolute precedence over both tag defaults and explicitly
// decoded YAML content (README, "Environment Injection"). The single
// exception is a priority:"value" tag on a non-zero field: the held value
// (decoded YAML, an applied default or a pre-filled standalone value) wins
// and the environment is skipped; a zero field applies the environment as the
// only source.
//
// It is applied after decoding on every load path and returns an error if an
// environment value cannot be converted to its target field type: invalid
// bool, int, uint, float and duration literals are reported with the field
// and the offending value named (see setScalarFromStringStrict).
func applyEnvOverrides(ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return nil
	}

	return walkEnvOverrides(v.Elem())
}

// walkEnvOverrides performs the underlying recursive environment override
// pass. Values are converted with the strict matrix of
// setScalarFromStringStrict: an invalid literal aborts the pass with an error
// instead of silently leaving the field zero.
//
//nolint:gocyclo // Reason: heavy reflection type-switch matrix structure requires high complexity.
func walkEnvOverrides(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldVal := v.Field(i)
			fieldType := t.Field(i)

			// Unexported fields are outside the configuration surface and
			// cannot be written through reflect: skipped entirely.
			if !fieldType.IsExported() {
				continue
			}

			// Legacy generic Value property fallback: only struct-typed Value
			// fields are transparent wrappers; a scalar Value field keeps its
			// own env handling.
			if fieldType.Name == "Value" && fieldVal.Kind() == reflect.Struct {
				if err := walkEnvOverrides(fieldVal); err != nil {
					return err
				}
				continue
			}

			if fieldVal.Kind() == reflect.Slice {
				if err := walkEnvOverrides(fieldVal); err != nil {
					return err
				}
				continue
			}

			if fieldVal.Kind() == reflect.Struct || fieldVal.Kind() == reflect.Map {
				if err := walkEnvOverrides(fieldVal); err != nil {
					return err
				}
				continue
			}

			envVarName := fieldType.Tag.Get("env")
			if envVarName == "" {
				continue
			}

			// priority:"value" arbitrates between the value source and the
			// environment: a non-zero field already holds its value (decoded
			// YAML, an applied default or a pre-filled standalone value), so
			// the environment assignment is skipped. A zero field has no value
			// source and the environment applies as the only one. Tag
			// validation is not repeated here: every load path runs SetDefaults
			// (which reports invalid priority tags) before this pass.
			if fieldType.Tag.Get("priority") == "value" && !fieldVal.IsZero() {
				continue
			}

			if envVal, isSet := os.LookupEnv(envVarName); isSet && envVal != "" {
				if err := setScalarFromStringStrict(fieldVal, envVal, fieldType.Name); err != nil {
					return err
				}
			}
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			element := v.Index(i)
			if element.Kind() == reflect.Pointer {
				if !element.IsNil() {
					if err := walkEnvOverrides(element.Elem()); err != nil {
						return err
					}
				}
				continue
			}
			if element.Kind() == reflect.Struct {
				copyElem := reflect.New(element.Type()).Elem()
				copyElem.Set(element)
				if err := walkEnvOverrides(copyElem); err != nil {
					return err
				}
				v.Index(i).Set(copyElem)
			}
		}

	case reflect.Map:
		for _, key := range v.MapKeys() {
			element := v.MapIndex(key)
			if element.Kind() == reflect.Pointer {
				if !element.IsNil() {
					if err := walkEnvOverrides(element.Elem()); err != nil {
						return err
					}
				}
				continue
			}
			if element.Kind() == reflect.Struct {
				copyElem := reflect.New(element.Type()).Elem()
				copyElem.Set(element)
				if err := walkEnvOverrides(copyElem); err != nil {
					return err
				}
				v.SetMapIndex(key, copyElem)
			}
		}
	}
	return nil
}

// pathRule captures a nested-target validation rule declared on a parent field.
// Target holds the dot-separated relative path to the nested field (e.g.
// "min_len" or "limits.min_len") and Rules holds the validators that apply to
// the field living at that path.
type pathRule struct {
	Target string
	Rules  map[string]string
}

// parsePathToken attempts to parse a path-style rule token of the form
// "path.rule=value" (or the flag form "path.required") where the final dot
// segment is a known rule and the leading segments form the nested target path.
// It returns false when the token does not match the path syntax.
//
// Rule values may contain dots (e.g. regexp patterns like "^a.b.c$"): the
// recognizer backs off over such segments and re-attaches the dropped tail to
// the parsed value on a successful parameterized match, so the value is never
// truncated. A flag rule that leaves a non-empty tail behind is not a success
// either — the tail would be silently lost — so the token keeps backing off
// and is ultimately rejected in full.
//
// Values containing commas must be quoted in the tag ('a,b' or "a,b"): the
// tokenizer ends a path token at the first unquoted comma, so an unquoted
// comma still separates the token from the next rule.
func parsePathToken(token string) (target, rule, value string, ok bool) {
	// Locate the rule segment: the last dot-separated chunk must be a known rule name.
	rest := token

	// dropped holds the tail segments (each with its leading dot) set aside
	// while backing off; on success they are re-attached in reverse order.
	var dropped []string

	for {
		idx := strings.LastIndex(rest, ".")
		if idx < 0 {
			return "", "", "", false
		}

		last := rest[idx+1:]
		name := last
		hasParam := false

		if cutName, cutValue, isCut := strings.Cut(last, "="); isCut {
			name = cutName
			value = cutValue
			hasParam = true
		}

		if isKnownRule(name, hasParam) {
			if !hasParam && len(dropped) > 0 {
				// A flag rule cannot own the trailing segments: flag values are
				// empty, so the tail would be dropped silently. Keep backing off.
				value = ""
				dropped = append(dropped, rest[idx:])
				rest = rest[:idx]
				continue
			}

			target = strings.TrimSuffix(rest[:idx], ".")
			rule = name
			if target == "" {
				return "", "", "", false
			}

			for i := len(dropped) - 1; i >= 0; i-- {
				value += dropped[i]
			}

			return target, rule, value, true
		}

		// Try a shorter path prefix: the rule segment may itself contain dots
		// only inside its value (e.g. regexp patterns), so back off one segment.
		value = ""
		dropped = append(dropped, rest[idx:])
		rest = rest[:idx]
	}
}

// isKnownRule reports whether name matches a known rule with the given
// parameterization mode (parameterized rules end with "=" in allRules).
func isKnownRule(name string, param bool) bool {
	for _, rule := range allRules {
		trimmed := strings.TrimSuffix(rule, "=")
		if trimmed != name {
			continue
		}

		_, isParamRule := parameterizedRules[trimmed]

		return isParamRule == param
	}

	return false
}

// allRules lists every rule token recognized by the validate tag parser
// (parameterized rules carry a trailing "=").
var allRules = []string{
	"mincount=", "maxcount=", "endpoint", "not_empty", "required",
	"regexp=", "choice=", "minlen=", "maxlen=", "format=",
	"min=", "max=", "url", "lt=", "gt=", "required_if=", "break",
}

// parameterizedRules marks the rules that require an "=value" argument.
var parameterizedRules = map[string]bool{
	"mincount": true, "maxcount": true, "regexp": true, "choice": true,
	"minlen": true, "maxlen": true, "format": true, "min": true,
	"max": true, "lt": true, "gt": true, "required_if": true,
}

// validateSegments holds the break-segmented rule mappings of one validate
// tag: the head carries the unconditional rules (including the required_if
// condition that precedes break), the tail carries the value rules that run
// only when that condition is met. A tag without break fills the head only.
type validateSegments struct {
	head     map[string]string
	tail     map[string]string
	hasBreak bool
}

// fullRules returns the complete rule map of the tag: the head and, when a
// break is present, the conditional tail merged together (tail values win on
// a key collision, mirroring the map-replace semantics of a flat tag).
// Consumers that do not apply the break semantics — the defaults-presence
// check, path-rule extraction — see the full picture through this map.
func (s validateSegments) fullRules() map[string]string {
	merged := make(map[string]string, len(s.head)+len(s.tail))
	for ruleKey, ruleValue := range s.head {
		merged[ruleKey] = ruleValue
	}
	for ruleKey, ruleValue := range s.tail {
		merged[ruleKey] = ruleValue
	}

	return merged
}

// breakSegments carries the break-segmented rules of one validate tag into
// validateValue: the head must hold the required_if condition governing the
// tail; the tail value rules apply only when that condition is met.
type breakSegments struct {
	head map[string]string
	tail map[string]string
}

// parseValidateTag tokenizes the validation tag string into a single flat
// rule mapping. It isolates parameters even if they contain punctuation like
// commas (e.g. inside regex patterns or quoted values). A repeated
// required_if on one field is a configuration mistake: the map keeps the
// last value exactly like before (help rendering and the default-presence
// check are unaffected), and the error names both values with the merge
// hint; the validation pass reports it with the field path.
//
// Break configuration mistakes (a break without a preceding required_if, a
// second break, a required_if after break, break inside a path entry) are
// reported through the error as well; see parseValidateTagSegments.
func parseValidateTag(tag string) (map[string]string, error) {
	segments, err := parseValidateTagSegments(tag)

	return segments.fullRules(), err
}

// parseValidateTagSegments tokenizes the validation tag string into the
// break-segmented rule mappings. A standalone break token splits the tag:
// rules parsed before it land in the head, rules parsed after it in the
// tail. The break itself is a service token and never lands in a rule map.
// The tail condition is the required_if parsed before break, so a tag with
// break must carry exactly one required_if in its head: a break without a
// preceding required_if, a second break token and a required_if placed after
// break are configuration mistakes reported as the error (parsing continues,
// so the returned segments stay as complete as possible). break inside a
// path entry ("item.break") is rejected too: the tail condition governs the
// owner field's own value rules, not the rules of a nested target. A
// distorted "break=..." form is a mistake as well: break takes no value, and
// the '='-tail would otherwise be silently skipped as an unknown token.
//
// Path-style rules are never conditional: they address nested fields rather
// than the owner value, so a path token after break is collected into the
// head and applies unconditionally. The placement is bounded by the
// parameter scanner: a path token directly after required_if is swallowed
// into the condition operand (a comma before a path token does not stop the
// parameter scan — only a rule start does), so its rule is lost; place path
// rules before required_if or after break.
//
// The break token is recognized only at a rule position: inside a
// parameter value ("choice=break", a required_if atom "mode==break") the
// word is data, not the service token.
func parseValidateTagSegments(tag string) (validateSegments, error) {
	segments := validateSegments{
		head: make(map[string]string),
		tail: make(map[string]string),
	}
	rules := segments.head
	var segErr error

	// reportSegErr keeps the first configuration mistake: later ones never
	// overwrite it, matching the single-error surface of the flat parser.
	reportSegErr := func(err error) {
		if segErr == nil {
			segErr = err
		}
	}

	if tag == "" {
		return segments, nil
	}

	// isRuleStart checks if position idx starts with a known rule
	isRuleStart := func(s string, idx int) bool {
		for _, rule := range allRules {
			if strings.HasPrefix(s[idx:], rule) {
				return true
			}
		}

		return false
	}

	i := 0
	n := len(tag)

	for i < n {
		// Skip delimiters
		for i < n && (tag[i] == ',' || tag[i] == ' ' || tag[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}

		// Find which rule starts here
		foundRule := ""
		ruleLen := 0
		isParam := false

		for _, rule := range allRules {
			if strings.HasPrefix(tag[i:], rule) {
				afterRule := i + len(rule)
				if strings.HasSuffix(rule, "=") {
					// Parameterized rule
					foundRule = strings.TrimSuffix(rule, "=")
					ruleLen = len(rule)
					isParam = true
					break
				} else {
					// Flag rule - ensure it's not a prefix of another word
					if afterRule >= n {
						foundRule = rule
						ruleLen = len(rule)
						break
					}
					nextCh := tag[afterRule]
					if nextCh == ',' || nextCh == ' ' || nextCh == '\t' || nextCh == '=' {
						foundRule = rule
						ruleLen = len(rule)
						break
					}
				}
			}
		}

		if foundRule == "break" {
			// The service break token: it splits the tag instead of landing
			// in a rule map. Configuration mistakes are reported, parsing
			// continues into the tail.
			if i+ruleLen < n && tag[i+ruleLen] == '=' {
				// A distorted "break=..." form: the flag-rule scan accepts
				// '=' as the next character to keep flag rules uniform, but
				// the service token takes no value, and the '='-tail is
				// silently skipped as an unknown token. The split semantics
				// stay (the tail keeps parsing), the mistake surfaces.
				reportSegErr(fmt.Errorf("break rule takes no value: the service token is written as a standalone break"))
			}
			if segments.hasBreak {
				reportSegErr(fmt.Errorf("duplicate break rule: a tag may carry a single break token"))
			} else if _, hasReqIf := segments.head["required_if"]; !hasReqIf {
				reportSegErr(fmt.Errorf("break rule must be preceded by a required_if rule: the tail rules run only when that condition is met"))
			}

			segments.hasBreak = true
			rules = segments.tail
			i += ruleLen

			continue
		}

		if foundRule == "" {
			// Try the path-rule syntax: <nested.path>.<rule>=<value>
			// The token scan tracks quotes like the parameterized branch
			// below: commas inside quoted values ('a,b' or "a,b") do not
			// terminate the token, so path-rule values keep their commas
			// instead of being silently truncated at the first unquoted one.
			tokenEnd := i
			inQuotes := false
			quoteChar := byte(0)
			for tokenEnd < n {
				ch := tag[tokenEnd]

				if ch == ',' && !inQuotes {
					// The token ends at the first unquoted comma.
					break
				}

				switch {
				case (ch == '\'' || ch == '"') && !inQuotes:
					inQuotes = true
					quoteChar = ch
				case ch == quoteChar && inQuotes:
					inQuotes = false
					quoteChar = 0
				}

				tokenEnd++
			}

			if target, rule, value, ok := parsePathToken(tag[i:tokenEnd]); ok {
				if rule == "break" {
					// "item.break": the service token on a nested target is
					// the same configuration-mistake class as the misplaced
					// forms above.
					reportSegErr(fmt.Errorf("break rule inside a path entry is not supported: the tail condition governs the owner field's value rules only"))
					i = tokenEnd

					continue
				}

				entry := rule
				if value != "" {
					entry += "=" + value
				}

				key := "path:" + target
				if existing, hasKey := rules[key]; hasKey && existing != "" {
					rules[key] = existing + "," + entry
				} else {
					rules[key] = entry
				}

				i = tokenEnd

				continue
			}

			// Unknown rule, skip to next comma
			for i < n && tag[i] != ',' {
				i++
			}
			if i < n {
				i++
			}

			continue
		}

		i += ruleLen

		if !isParam {
			rules[foundRule] = ""
			continue
		}

		// Parse parameter value, respecting quotes
		var value strings.Builder
		inQuotes := false
		quoteChar := byte(0)

		for i < n {
			ch := tag[i]

			// Handle quote boundaries
			if (ch == '\'' || ch == '"') && !inQuotes {
				inQuotes = true
				quoteChar = ch
				value.WriteByte(ch)
				i++
				continue
			}
			if ch == quoteChar && inQuotes {
				inQuotes = false
				quoteChar = 0
				value.WriteByte(ch)
				i++
				continue
			}

			// Check for rule-separating comma
			if ch == ',' && !inQuotes {
				lookahead := i + 1
				for lookahead < n && (tag[lookahead] == ' ' || tag[lookahead] == '\t') {
					lookahead++
				}
				if lookahead >= n || isRuleStart(tag, lookahead) {
					break
				}
			}

			value.WriteByte(ch)
			i++
		}

		newValue := strings.TrimSpace(value.String())
		if foundRule == "required_if" && segments.hasBreak {
			// The tail condition is defined by the required_if preceding
			// break: a later one is a configuration mistake and stays
			// unrecorded — nothing would govern it.
			reportSegErr(fmt.Errorf("required_if rule must be placed before break: the tail condition is defined by the required_if preceding it"))

			continue
		}
		if foundRule == "required_if" {
			if existing, has := rules[foundRule]; has && segErr == nil {
				segErr = fmt.Errorf("duplicate required_if rule: merge the conditions with \"&&\" or \"||\" into a single expression instead of repeating the rule (%q would silently replace %q)", newValue, existing)
			}
		}

		rules[foundRule] = newValue
	}

	return segments, segErr
}

// Validate executes deep, recursive structural checks across the application configuration models.
// It gathers all validation faults into a composite error stack instead of exiting early.
func Validate(ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return nil
	}
	var errs []error
	visited := make(map[uintptr]struct{})
	validateValue(v.Elem(), "", nil, v.Elem(), v.Elem(), &errs, visited, false, nil)
	if len(errs) > 0 {
		var sb strings.Builder
		for i, err := range errs {
			if i > 0 {
				sb.WriteString("\n")
			}

			sb.WriteString(err.Error())
		}

		return fmt.Errorf("%s", sb.String())
	}

	return nil
}

// extractPathRules pulls every "path:<target>" entry out of a parsed rule map
// and re-tokenizes the collected rule list through parseValidateTag. The path
// entries are removed from the map so that they are never mistaken for rules
// targeting the owner field itself. A duplicate required_if inside a path
// entry is the same configuration mistake as on a direct tag and aborts the
// extraction with the parser error.
func extractPathRules(rules map[string]string) ([]pathRule, error) {
	var extracted []pathRule

	for key, value := range rules {
		if !strings.HasPrefix(key, "path:") {
			continue
		}

		delete(rules, key)
		parsed, err := parseValidateTag(value)
		if err != nil {
			return nil, err
		}

		extracted = append(extracted, pathRule{
			Target: strings.TrimPrefix(key, "path:"),
			Rules:  parsed,
		})
	}

	return extracted, nil
}

// resolvePathTarget walks the dot-separated target path from the owner value
// and returns the referenced field value. Struct segments are matched by yaml
// tag name or Go field name, and pointers are dereferenced along the way.
func resolvePathTarget(owner reflect.Value, target string) (reflect.Value, bool) {
	current := owner

	if current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return reflect.Value{}, false
		}

		current = current.Elem()
	}

	for _, segment := range strings.Split(target, ".") {
		if current.Kind() != reflect.Struct {
			return reflect.Value{}, false
		}

		next := resolveStructField(current, segment)
		if !next.IsValid() {
			return reflect.Value{}, false
		}

		current = next
	}

	return current, true
}

// matchStructField locates a struct field by its Go field name or the first
// component of its yaml tag and returns the raw field value together with the
// yaml name to use in error messages (the lowercased Go field name when the
// tag is empty). Pointer scopes are dereferenced and non-struct scopes yield
// no match. The returned field value is raw: callers decide whether to unwrap
// pointer fields themselves.
func matchStructField(v reflect.Value, name string) (field reflect.Value, yamlName string, found bool) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}, "", false
		}

		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}, "", false
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tagParts := strings.Split(f.Tag.Get("yaml"), ",")
		if f.Name == name || tagParts[0] == name {
			yamlTargetName := tagParts[0]
			if yamlTargetName == "" {
				yamlTargetName = strings.ToLower(f.Name)
			}

			return v.Field(i), yamlTargetName, true
		}
	}

	return reflect.Value{}, "", false
}

// resolveStructField locates a struct field by yaml tag name (first tag
// component) or by its Go field name and returns its value. A non-nil pointer
// field is dereferenced so path rules operate on the pointed-to value.
func resolveStructField(v reflect.Value, segment string) reflect.Value {
	field, _, found := matchStructField(v, segment)
	if !found {
		return reflect.Value{}
	}

	if field.Kind() == reflect.Pointer && !field.IsNil() {
		return field.Elem()
	}

	return field
}

// findStructTarget locates a struct field by its Go field name or the first
// component of its yaml tag and returns the field value together with the
// yaml name to use in error messages. A missing match is reported by the zero
// (invalid) target value, which callers check via IsValid.
func findStructTarget(v reflect.Value, name string) (target reflect.Value, yamlName string) {
	target, yamlName, _ = matchStructField(v, name)

	return target, yamlName
}

// sameStructValue reports whether two struct values reference the same
// underlying struct instance. Values that are not addressable structs of the
// same type are conservatively treated as different, which only costs a
// redundant fallback lookup for required_if target resolution.
func sameStructValue(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() || a.Kind() != reflect.Struct || b.Kind() != reflect.Struct || a.Type() != b.Type() {
		return false
	}

	if !a.CanAddr() || !b.CanAddr() {
		return false
	}

	return a.Addr().UnsafePointer() == b.Addr().UnsafePointer()
}

// requiredIfCondition is a parsed required_if condition: the target field
// name, the comparison operator and the comparison operand.
type requiredIfCondition struct {
	target string
	op     string
	value  string
}

// parseRequiredIfCondition splits a required_if expression into the condition
// target, the comparison operator and the comparison operand.
//
// The canonical operators are "==" (value equality), "!=" (value inequality),
// "~=" (regexp match) and "^=" (regexp mismatch); the deprecated colon forms
// "key:value" and "key:!value" are still supported for backward compatibility.
// The leftmost separator wins, so legacy operands that embed canonical
// operators ("key:abc!=def") keep their legacy parsing.
//
// ok reports whether any operator was found; operator-less expressions are
// reported as configuration errors by the caller, never silently skipped.
func parseRequiredIfCondition(expr string) (requiredIfCondition, bool) {
	bestIdx := -1
	bestOp := ""

	for _, op := range []string{"==", "!=", "~=", "^=", ":"} {
		if idx := strings.Index(expr, op); idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
			bestIdx = idx
			bestOp = op
		}
	}

	if bestIdx < 0 {
		return requiredIfCondition{}, false
	}

	return requiredIfCondition{
		target: expr[:bestIdx],
		op:     bestOp,
		value:  expr[bestIdx+len(bestOp):],
	}, true
}

// validateValue performs automated constraint checks down the configuration node hierarchy tree.
//
// The scope argument carries the struct owning the currently validated value;
// cross-field rules such as required_if resolve bare target names against it
// before falling back to the root.
//
// The visited argument tracks pointer addresses on the current validation
// path: a pointer revisited within the same path is reported as a cyclic
// structure instead of recursing forever. Entries are removed on exit, so
// shared (DAG) subobjects reached from several paths are validated normally.
//
// The inheritedRules argument marks the recursion into slice and map
// elements, which receive the collection's rules: rule-declaration mistakes
// are reported once on the collection itself and never repeated per element,
// while value faults still report at every element in place.
//
// The brk argument carries the break segmentation of the tag on the field
// that declares it (nil otherwise): the required_if preceding break is
// evaluated exactly once here, and the tail value rules join the effective
// rules only when the condition is met. Conditional tail rules never reach
// the collection elements: their configuration mistakes are reported on the
// collection itself, and the condition is not re-evaluated per element.
//
//nolint:gocyclo // Reason: massive tag boundaries processing engine requires extensive conditional logical blocks.
func validateValue(v reflect.Value, currentPath string, rules map[string]string, root, scope reflect.Value, errs *[]error, visited map[uintptr]struct{}, inheritedRules bool, brk *breakSegments) {
	// A nil pointer is kept as-is: it is not dereferenced (would panic) and
	// not dropped here (that silently skipped required, not_empty and
	// required_if on unset pointer fields). The rule block below is nil-safe
	// (IsZero works on pointers and every kind-specific validator branch
	// filters on String/Slice/Map or numeric kinds) and the trailing
	// recursion switch has no Pointer case, so no recursion happens either.
	if v.Kind() == reflect.Pointer && !v.IsNil() {
		addr := v.Pointer()
		if _, seen := visited[addr]; seen {
			*errs = append(*errs, fmt.Errorf("field %s: cyclic structure detected", currentPath))

			return
		}

		visited[addr] = struct{}{}
		defer delete(visited, addr)
		v = v.Elem()
	}

	// breakInherited holds the rules collection elements receive when the
	// tag carries break: the unconditional head minus the evaluated
	// required_if. nil keeps the plain inheritance of the flat rule map.
	var breakInherited map[string]string

	if len(rules) > 0 {
		// Rule-declaration mistakes (unknown required_if targets, invalid
		// condition patterns, conflicting boundaries, an empty choice list,
		// ...) belong to the rule holder itself. Slice and map elements carry
		// the collection's rules for value checks, so these errors are
		// reported once on the collection and never repeated per element;
		// value faults keep reporting at every element in place.
		addRuleConfigErr := func(format string, args ...any) {
			if inheritedRules {
				return
			}

			*errs = append(*errs, fmt.Errorf(format, args...))
		}

		// Break segmentation: the required_if preceding break defines the
		// tail condition and is evaluated exactly once, here, with its
		// regular semantics — configuration mistakes are always reported,
		// and the required error is appended when the condition is met on a
		// zero field.
		if brk != nil {
			conditionMet := false
			if reqIfStr, hasReqIf := brk.head["required_if"]; hasReqIf {
				conditionMet = evaluateRequiredIf(reqIfStr, v, currentPath, root, scope, addRuleConfigErr, errs)
			}

			if !conditionMet {
				// The accepted trade-off of the break contract: when the
				// condition is false the tail value rules are skipped
				// entirely — including non-empty field values. Their
				// configuration mistakes still surface through the audit
				// below, so they never depend on the condition.
				auditBreakTailConfig(brk.head, brk.tail, v, currentPath, addRuleConfigErr)
			}

			// The effective rules for the value phase below: the
			// unconditional head minus the already-evaluated required_if,
			// joined by the tail when the condition is met.
			headOnly := make(map[string]string, len(brk.head))
			for ruleKey, ruleValue := range brk.head {
				if ruleKey == "required_if" {
					// Evaluated above exactly once; a second evaluation
					// would duplicate its reports.
					continue
				}

				headOnly[ruleKey] = ruleValue
			}

			rules = headOnly
			if conditionMet {
				rules = make(map[string]string, len(headOnly)+len(brk.tail))
				for ruleKey, ruleValue := range headOnly {
					rules[ruleKey] = ruleValue
				}
				for ruleKey, ruleValue := range brk.tail {
					rules[ruleKey] = ruleValue
				}
			}

			// Collection elements inherit the unconditional head rules only:
			// the tail stays conditional, and the condition is not
			// re-evaluated per element.
			breakInherited = headOnly
			brk = nil
		}

		// 1.1. Basic check for unconditional field requirement
		_, hasNotEmpty := rules["not_empty"]
		_, hasRequired := rules["required"]
		isRequired := hasNotEmpty || hasRequired
		if isRequired && v.IsZero() {
			*errs = append(*errs, fmt.Errorf("field %s: is empty, but required", currentPath))
		}

		// 1.2. Cross-field conditional validation execution (required_if) positioned at top tier level
		if reqIfStr, hasReqIf := rules["required_if"]; hasReqIf {
			// Boolean expressions ("&&", "||", grouping parentheses) are
			// lexed, parsed and evaluated by required_if_expr.go; the legacy
			// single-condition form keeps its exact pre-expression behavior.
			evaluateRequiredIf(reqIfStr, v, currentPath, root, scope, addRuleConfigErr, errs)
		}

		isCollectionElement := strings.Contains(currentPath, "[")

		minStr, hasMin := rules["min"]
		maxStr, hasMax := rules["max"]
		ltStr, hasLt := rules["lt"]
		gtStr, hasGt := rules["gt"]

		minLenStr, hasMinLen := rules["minlen"]
		maxLenStr, hasMaxLen := rules["maxlen"]
		minCountStr, hasMinCount := rules["mincount"]
		maxCountStr, hasMaxCount := rules["maxcount"]
		formatStr, hasFormat := rules["format"]

		_, hasEndpoint := rules["endpoint"]
		_, hasURL := rules["url"]

		_, hasChoice := rules["choice"]

		if (hasMin && hasGt) || (hasMax && hasLt) {
			// Configuration fault: reported and validation continues — the
			// composite stack gathers every fault instead of exiting early.
			addRuleConfigErr("field %s: invalid validator configuration: conflicting boundaries metrics", currentPath)
		}

		// Network format rules (format/endpoint/url) are mutually exclusive
		// with each other: a configuration mistake that must fire regardless
		// of the field value, same as the conflicting boundaries check above.
		if hasFormat || hasEndpoint || hasURL {
			mutIdx := 0
			if hasFormat {
				mutIdx++
			}
			if hasEndpoint {
				mutIdx++
			}
			if hasURL {
				mutIdx++
			}
			if mutIdx > 1 {
				addRuleConfigErr("field %s: invalid configuration: format rules are strictly mutually exclusive", currentPath)
			}
		}

		// An empty choice list (a bare "choice=" or one made only of empty
		// elements) can never constrain anything: a configuration mistake
		// reported before the value gate, regardless of the field value —
		// same class as the conflicting boundaries and mutual exclusion
		// checks above.
		if choiceStr, hasChoice := rules["choice"]; hasChoice && len(parseChoiceValues(choiceStr)) == 0 {
			addRuleConfigErr("field %s: invalid validator configuration: empty choice list", currentPath)
		}

		hasAnyMinBound := hasMin || hasGt
		hasAnyMaxBound := hasMax || hasLt

		if !v.IsZero() || isCollectionElement || hasAnyMinBound || hasAnyMaxBound ||
			hasMinLen || hasMaxLen || hasMinCount || hasMaxCount || hasChoice {

			if choiceStr, hasChoice := rules["choice"]; hasChoice && v.Kind() == reflect.String {
				valStr := v.String()
				allowedChoices := parseChoiceValues(choiceStr)

				// Determine mode: whitelist or blacklist
				isBlacklist := true
				for _, c := range allowedChoices {
					if c != "" && !strings.HasPrefix(c, "!") {
						isBlacklist = false
						break
					}
				}

				if isBlacklist {
					for _, c := range allowedChoices {
						forbidden := strings.TrimPrefix(c, "!")
						if valStr == forbidden {
							*errs = append(*errs, fmt.Errorf("field %s: value %q is forbidden by blacklist", currentPath, valStr))
						}
					}
				} else {
					isValid := false
					for _, c := range allowedChoices {
						if valStr == c {
							isValid = true
							break
						}
					}
					if !isValid {
						displayChoices := make([]string, len(allowedChoices))
						for i, c := range allowedChoices {
							displayChoices[i] = fmt.Sprintf("%q", c)
						}
						*errs = append(*errs, fmt.Errorf("field %s: value %q is invalid; allowed choices are [%s]", currentPath, valStr, strings.Join(displayChoices, ", ")))
					}
				}
			}

			if expr, hasRegexp := rules["regexp"]; hasRegexp && v.Kind() == reflect.String {
				valStr := v.String()
				expr = stripRuleQuotes(expr)
				re, err := regexp.Compile(expr)
				if err != nil {
					*errs = append(*errs, fmt.Errorf("field %s: invalid regular expression syntax %q: %w", currentPath, expr, err))
				} else if !re.MatchString(valStr) {
					*errs = append(*errs, fmt.Errorf("field %s: value %q does not match regular expression %q", currentPath, valStr, expr))
				}
			}

			if _, hasHostPort := rules["endpoint"]; hasHostPort && v.Kind() == reflect.String {
				valStr := v.String()
				host, port, err := net.SplitHostPort(valStr)
				if err != nil {
					*errs = append(*errs, fmt.Errorf("field %s: value %q is not a valid host:port format: %w", currentPath, valStr, err))
				} else {
					p, pErr := strconv.Atoi(port)
					if pErr != nil || p < 1 || p > 65535 {
						*errs = append(*errs, fmt.Errorf("field %s: value %q contains an invalid port number", currentPath, valStr))
					}
					if strings.Contains(host, ":") {
						if ip := net.ParseIP(host); ip == nil {
							*errs = append(*errs, fmt.Errorf("field %s: value %q contains an invalid IPv6 address", currentPath, valStr))
						}
					}
				}
			}

			if _, hasURL := rules["url"]; hasURL && v.Kind() == reflect.String {
				valStr := v.String()
				if !strings.Contains(valStr, "://") {
					*errs = append(*errs, fmt.Errorf("field %s: value %q is missing a URL scheme separator (e.g., scheme://host)", currentPath, valStr))
				} else {
					parsedURL, err := url.ParseRequestURI(valStr)
					switch {
					case err != nil:
						*errs = append(*errs, fmt.Errorf("field %s: value %q is not a valid URL: %w", currentPath, valStr, err))
					case parsedURL.Scheme == "":
						*errs = append(*errs, fmt.Errorf("field %s: value %q has an empty or invalid URL scheme", currentPath, valStr))
					case parsedURL.Hostname() == "":
						// A scheme-only URL ("http://", "http:///path",
						// "http://:8080") carries no addressable content: the
						// host part is mandatory, same as the scheme.
						*errs = append(*errs, fmt.Errorf("field %s: value %q is missing a URL host (scheme alone, e.g. \"http://\", is not a valid URL)", currentPath, valStr))
					}
				}
			}

			if hasFormat && v.Kind() == reflect.String {
				valStr := v.String()

				switch formatStr {
				case "ip":
					if ip := net.ParseIP(valStr); ip == nil {
						*errs = append(*errs, fmt.Errorf("field %s: value %q is not a valid IP address", currentPath, valStr))
					}

				case "ipv4":
					if ip := net.ParseIP(valStr); ip == nil || ip.To4() == nil {
						*errs = append(*errs, fmt.Errorf("field %s: value %q is not a valid IPv4 address", currentPath, valStr))
					}

				case "ipv6":
					if ip := net.ParseIP(valStr); ip == nil || ip.To4() != nil {
						*errs = append(*errs, fmt.Errorf("field %s: value %q is not a valid IPv6 address", currentPath, valStr))
					}

				case "uuid":
					matched, _ := regexp.MatchString(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`, valStr)
					if !matched {
						*errs = append(*errs, fmt.Errorf("field %s: value %q is not a valid UUID", currentPath, valStr))
					}

				case "hex":
					if _, err := hex.DecodeString(valStr); err != nil {
						if errors.Is(err, hex.ErrLength) {
							*errs = append(*errs, fmt.Errorf("field %s: value %q is not valid hex: odd number of digits (%d)", currentPath, valStr, len(valStr)))
						} else {
							*errs = append(*errs, fmt.Errorf("field %s: value %q is not valid hex: %w", currentPath, valStr, err))
						}
					}

				default:
					// Unknown format token: report a configuration mistake instead of silently skipping.
					*errs = append(*errs, fmt.Errorf("field %s: unknown format %q (expected ip, ipv4, ipv6, uuid or hex)", currentPath, formatStr))
				}
			}

			if (hasMinLen || hasMaxLen) && v.Kind() == reflect.String {
				strLen := utf8.RuneCountInString(v.String())
				var minLen, maxLen int
				limitsOk := true
				if hasMinLen {
					n, err := strconv.Atoi(minLenStr)
					switch {
					case err != nil:
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: minlen %q: %w", currentPath, minLenStr, err))
						limitsOk = false

					case n < 0:
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: minlen (%d) cannot be negative", currentPath, n))
						limitsOk = false

					default:
						minLen = n
					}
				}
				if hasMaxLen {
					n, err := strconv.Atoi(maxLenStr)
					switch {
					case err != nil:
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: maxlen %q: %w", currentPath, maxLenStr, err))
						limitsOk = false

					case n < 0:
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: maxlen (%d) cannot be negative", currentPath, n))
						limitsOk = false

					default:
						maxLen = n
					}
				}
				if limitsOk {
					if hasMinLen && hasMaxLen && minLen > maxLen {
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: minlen (%d) cannot be greater than maxlen (%d)", currentPath, minLen, maxLen))
					} else {
						if hasMinLen && strLen < minLen {
							*errs = append(*errs, fmt.Errorf("field %s: string length %d is less than minlen %s", currentPath, strLen, minLenStr))
						}
						if hasMaxLen && strLen > maxLen {
							*errs = append(*errs, fmt.Errorf("field %s: string length %d exceeds maxlen %s", currentPath, strLen, maxLenStr))
						}
					}
				}
			}

			if (hasMinCount || hasMaxCount) && (v.Kind() == reflect.Slice || v.Kind() == reflect.Map) {
				count := v.Len()
				var minCount, maxCount int
				limitsOk := true
				if hasMinCount {
					n, err := strconv.Atoi(minCountStr)
					switch {
					case err != nil:
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: mincount %q: %w", currentPath, minCountStr, err))
						limitsOk = false

					case n < 0:
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: mincount (%d) cannot be negative", currentPath, n))
						limitsOk = false

					default:
						minCount = n
					}
				}
				if hasMaxCount {
					n, err := strconv.Atoi(maxCountStr)
					switch {
					case err != nil:
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: maxcount %q: %w", currentPath, maxCountStr, err))
						limitsOk = false

					case n < 0:
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: maxcount (%d) cannot be negative", currentPath, n))
						limitsOk = false

					default:
						maxCount = n
					}
				}
				if limitsOk {
					if hasMinCount && hasMaxCount && minCount > maxCount {
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: mincount (%d) cannot be greater than maxcount (%d)", currentPath, minCount, maxCount))
					} else {
						if hasMinCount && count < minCount {
							*errs = append(*errs, fmt.Errorf("field %s: collection size %d is less than mincount %s", currentPath, count, minCountStr))
						}
						if hasMaxCount && count > maxCount {
							*errs = append(*errs, fmt.Errorf("field %s: collection size %d exceeds maxcount %s", currentPath, count, maxCountStr))
						}
					}
				}
			}

			if hasAnyMinBound || hasAnyMaxBound {
				kind := v.Kind()
				if v.Type() == durationType {
					val := v.Interface().(time.Duration)
					var minVal, maxVal, ltVal, gtVal time.Duration
					limitsOk := true
					if hasMin {
						d, err := time.ParseDuration(minStr)
						if err != nil {
							*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: min %q: %w", currentPath, minStr, err))
							limitsOk = false
						} else {
							minVal = d
						}
					}
					if hasMax {
						d, err := time.ParseDuration(maxStr)
						if err != nil {
							*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: max %q: %w", currentPath, maxStr, err))
							limitsOk = false
						} else {
							maxVal = d
						}
					}
					if hasLt {
						d, err := time.ParseDuration(ltStr)
						if err != nil {
							*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: lt %q: %w", currentPath, ltStr, err))
							limitsOk = false
						} else {
							ltVal = d
						}
					}
					if hasGt {
						d, err := time.ParseDuration(gtStr)
						if err != nil {
							*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: gt %q: %w", currentPath, gtStr, err))
							limitsOk = false
						} else {
							gtVal = d
						}
					}
					if limitsOk {
						if hasMin && hasMax && minVal > maxVal {
							*errs = append(*errs, fmt.Errorf("field %s: configuration error: min %v > max %v", currentPath, minVal, maxVal))
						}
						if hasMin && val < minVal {
							*errs = append(*errs, fmt.Errorf("field %s: value %v < min %s", currentPath, val, minStr))
						}
						if hasMax && val > maxVal {
							*errs = append(*errs, fmt.Errorf("field %s: value %v > max %s", currentPath, val, maxStr))
						}
						if hasLt && val >= ltVal {
							*errs = append(*errs, fmt.Errorf("field %s: value %v must be < %s", currentPath, val, ltStr))
						}
						if hasGt && val <= gtVal {
							*errs = append(*errs, fmt.Errorf("field %s: value %v must be > %s", currentPath, val, gtStr))
						}
					}
				} else {
					switch {
					case kind >= reflect.Int && kind <= reflect.Int64:
						val := v.Int()
						var minVal, maxVal, ltVal, gtVal int64
						limitsOk := true
						if hasMin {
							n, err := strconv.ParseInt(minStr, 10, 64)
							if err != nil {
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: min %q: %w", currentPath, minStr, err))
								limitsOk = false
							} else {
								minVal = n
							}
						}
						if hasMax {
							n, err := strconv.ParseInt(maxStr, 10, 64)
							if err != nil {
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: max %q: %w", currentPath, maxStr, err))
								limitsOk = false
							} else {
								maxVal = n
							}
						}
						if hasLt {
							n, err := strconv.ParseInt(ltStr, 10, 64)
							if err != nil {
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: lt %q: %w", currentPath, ltStr, err))
								limitsOk = false
							} else {
								ltVal = n
							}
						}
						if hasGt {
							n, err := strconv.ParseInt(gtStr, 10, 64)
							if err != nil {
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: gt %q: %w", currentPath, gtStr, err))
								limitsOk = false
							} else {
								gtVal = n
							}
						}
						if limitsOk {
							if hasMin && hasMax && minVal > maxVal {
								*errs = append(*errs, fmt.Errorf("field %s: configuration error: min %d > max %d", currentPath, minVal, maxVal))
							}
							if hasMin && val < minVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %d < min %s", currentPath, val, minStr))
							}
							if hasMax && val > maxVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %d > max %s", currentPath, val, maxStr))
							}
							if hasLt && val >= ltVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %d must be < %s", currentPath, val, ltStr))
							}
							if hasGt && val <= gtVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %d must be > %s", currentPath, val, gtStr))
							}
						}

					case kind >= reflect.Uint && kind <= reflect.Uint64:
						val := v.Uint()
						var minVal, maxVal, ltVal, gtVal uint64
						limitsOk := true
						if hasMin {
							n, err := strconv.ParseInt(minStr, 10, 64)
							switch {
							case err != nil:
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: min %q: %w", currentPath, minStr, err))
								limitsOk = false

							case n < 0:
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: min (%d) cannot be negative", currentPath, n))
								limitsOk = false

							default:
								minVal = uint64(n)
							}
						}
						if hasMax {
							n, err := strconv.ParseInt(maxStr, 10, 64)
							switch {
							case err != nil:
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: max %q: %w", currentPath, maxStr, err))
								limitsOk = false

							case n < 0:
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: max (%d) cannot be negative", currentPath, n))
								limitsOk = false

							default:
								maxVal = uint64(n)
							}
						}
						if hasLt {
							n, err := strconv.ParseInt(ltStr, 10, 64)
							switch {
							case err != nil:
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: lt %q: %w", currentPath, ltStr, err))
								limitsOk = false

							case n < 0:
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: lt (%d) cannot be negative", currentPath, n))
								limitsOk = false

							default:
								ltVal = uint64(n)
							}
						}
						if hasGt {
							n, err := strconv.ParseInt(gtStr, 10, 64)
							switch {
							case err != nil:
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: gt %q: %w", currentPath, gtStr, err))
								limitsOk = false

							case n < 0:
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: gt (%d) cannot be negative", currentPath, n))
								limitsOk = false

							default:
								gtVal = uint64(n)
							}
						}
						if limitsOk {
							if hasMin && hasMax && minVal > maxVal {
								*errs = append(*errs, fmt.Errorf("field %s: configuration error: min %d > max %d", currentPath, minVal, maxVal))
							}
							if hasMin && val < minVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %d < min %s", currentPath, val, minStr))
							}
							if hasMax && val > maxVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %d > max %s", currentPath, val, maxStr))
							}
							if hasLt && val >= ltVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %d must be < %s", currentPath, val, ltStr))
							}
							if hasGt && val <= gtVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %d must be > %s", currentPath, val, gtStr))
							}
						}

					case kind == reflect.Float32 || kind == reflect.Float64:
						val := v.Float()
						var minVal, maxVal, ltVal, gtVal float64
						limitsOk := true
						if hasMin {
							f, err := strconv.ParseFloat(minStr, 64)
							if err != nil {
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: min %q: %w", currentPath, minStr, err))
								limitsOk = false
							} else {
								minVal = f
							}
						}
						if hasMax {
							f, err := strconv.ParseFloat(maxStr, 64)
							if err != nil {
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: max %q: %w", currentPath, maxStr, err))
								limitsOk = false
							} else {
								maxVal = f
							}
						}
						if hasLt {
							f, err := strconv.ParseFloat(ltStr, 64)
							if err != nil {
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: lt %q: %w", currentPath, ltStr, err))
								limitsOk = false
							} else {
								ltVal = f
							}
						}
						if hasGt {
							f, err := strconv.ParseFloat(gtStr, 64)
							if err != nil {
								*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: gt %q: %w", currentPath, gtStr, err))
								limitsOk = false
							} else {
								gtVal = f
							}
						}
						if limitsOk {
							if hasMin && hasMax && minVal > maxVal {
								*errs = append(*errs, fmt.Errorf("field %s: configuration error: min %f > max %f", currentPath, minVal, maxVal))
							}
							if hasMin && val < minVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %f < min %s", currentPath, val, minStr))
							}
							if hasMax && val > maxVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %f > max %s", currentPath, val, maxStr))
							}
							if hasLt && val >= ltVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %f must be < %s", currentPath, val, ltStr))
							}
							if hasGt && val <= gtVal {
								*errs = append(*errs, fmt.Errorf("field %s: value %f must be > %s", currentPath, val, gtStr))
							}
						}

					default:
						// Container kinds inherit the boundary rules per element
						// through the recursive traversal below; any other
						// non-numeric kind with boundary rules is a configuration
						// mistake: report it instead of silently disabling the check.
						if kind != reflect.Slice && kind != reflect.Map && kind != reflect.Array {
							*errs = append(*errs, fmt.Errorf("field %s: boundary validators are not supported for kind %s", currentPath, kind))
						}
					}
				}
			}
		}
	}

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldVal := v.Field(i)
			fieldType := t.Field(i)

			// Unexported fields are outside the configuration surface and
			// cannot be read back through reflect (Interface would panic):
			// skipped entirely, their validate tags are not applied.
			if !fieldType.IsExported() {
				continue
			}

			// Resolve explicit tag mapping to construct beautiful target names layouts
			yamlName := fieldType.Tag.Get("yaml")
			if yamlName == "" {
				yamlName = strings.ToLower(fieldType.Name)
			} else {
				yamlName = strings.Split(yamlName, ",")[0]
			}

			nextPath := yamlName
			if currentPath != "" {
				nextPath = currentPath + "." + yamlName
			}

			if fieldType.Name == "Value" {
				// Legacy generic Value property evaluation fallback support:
				// the wrapper payload is validated transparently with the
				// parent path and rules, but the wrapper field's own validate
				// tag is no longer dropped — own rules override inherited
				// rules of the same key.
				merged := make(map[string]string, len(rules))
				for ruleKey, ruleValue := range rules {
					merged[ruleKey] = ruleValue
				}

				if validateTag, hasValidate := fieldType.Tag.Lookup("validate"); hasValidate {
					ownSegments, ownErr := parseValidateTagSegments(validateTag)
					if ownErr != nil {
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: %w", currentPath, ownErr))
					}

					if ownSegments.hasBreak {
						// A break in the wrapper's own tag cannot split the
						// merged rule set: the inherited rules carry no
						// segmentation of their own, so a tail condition
						// would be undefined. The declaration is a
						// configuration mistake; the rules still apply as
						// one flat set below.
						*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: break rule is not supported in the own validate tag of a Value wrapper field", currentPath))
					}

					for ruleKey, ruleValue := range ownSegments.fullRules() {
						merged[ruleKey] = ruleValue
					}
				}

				validateValue(fieldVal, currentPath, merged, root, v, errs, visited, false, nil)
				continue
			}

			var fieldRules map[string]string
			var fieldBreak *breakSegments
			if validateTag, hasValidate := fieldType.Tag.Lookup("validate"); hasValidate {
				segments, parseErr := parseValidateTagSegments(validateTag)
				if parseErr != nil {
					*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: %w", nextPath, parseErr))
				}

				fieldRules = segments.head
				if segments.hasBreak {
					// The flat map handed to the path extraction below is a
					// full copy: the extraction mutates the map it is given,
					// and the segmentation maps must stay intact.
					fieldRules = segments.fullRules()
					fieldBreak = &breakSegments{head: segments.head, tail: segments.tail}
				}
			}

			// Path-style rules ("min_len.min=0") apply to nested fields of this
			// instance instead of the field itself; resolve each target and
			// validate it with the full dot path so errors stay readable.
			pathRules, pathErr := extractPathRules(fieldRules)
			if pathErr != nil {
				*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: %w", nextPath, pathErr))
			}

			if len(pathRules) > 0 {
				for _, pr := range pathRules {
					target, found := resolvePathTarget(fieldVal, pr.Target)
					if !found {
						*errs = append(*errs, fmt.Errorf("field %s: path rule %q references a non-existent field", nextPath, pr.Target))

						continue
					}

					if !target.CanInterface() {
						// The path resolved to an unexported field: reading it
						// back through reflect would panic, so the rule is a
						// configuration mistake like a non-existent target.
						*errs = append(*errs, fmt.Errorf("field %s: path rule target field %q is not exported", nextPath, pr.Target))

						continue
					}

					targetPath := nextPath + "." + pr.Target
					validateValue(target, targetPath, pr.Rules, root, v, errs, visited, false, nil)
				}
			}

			validateValue(fieldVal, nextPath, fieldRules, root, v, errs, visited, false, fieldBreak)
		}
	case reflect.Slice:
		elemRules := rules
		if breakInherited != nil {
			elemRules = breakInherited
		}

		for i := 0; i < v.Len(); i++ {
			validateValue(v.Index(i), fmt.Sprintf("%s[%d]", currentPath, i), elemRules, root, v.Index(i), errs, visited, true, nil)
		}
	case reflect.Map:
		elemRules := rules
		if breakInherited != nil {
			elemRules = breakInherited
		}

		for _, key := range v.MapKeys() {
			validateValue(v.MapIndex(key), fmt.Sprintf("%s[%v]", currentPath, key.Interface()), elemRules, root, v.MapIndex(key), errs, visited, true, nil)
		}

	default:
		// Scalar or unsupported kind: nothing to recurse into.
	}
}

// parseChoiceValues parses a string of the form "val1,'val,with,comma',val2"
// with support for quoted values containing commas.
// It returns a slice of values with outer quotes removed.
func parseChoiceValues(input string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if (ch == '\'' || ch == '"') && !inQuotes {
			inQuotes = true
			quoteChar = ch
			continue
		}
		if ch == quoteChar && inQuotes {
			inQuotes = false
			quoteChar = 0
			continue
		}

		if ch == ',' && !inQuotes {
			val := strings.TrimSpace(current.String())
			if val != "" {
				result = append(result, val)
			}
			current.Reset()
			continue
		}

		current.WriteByte(ch)
	}

	val := strings.TrimSpace(current.String())
	if val != "" {
		result = append(result, val)
	}

	return result
}

// stripRuleQuotes removes a single matched pair of surrounding quote
// characters from a rule value. Quotes are a parser-level escaping facility:
// they protect delimiters (commas, rule-like prefixes) and are not part of
// the payload itself. To match a literal quote character, wrap the value in
// an opposite quote type (e.g., regexp="^'q'$").
func stripRuleQuotes(s string) string {
	if len(s) >= 2 && s[0] == s[len(s)-1] && (s[0] == '\'' || s[0] == '"') {
		return s[1 : len(s)-1]
	}

	return s
}

// auditBreakTailConfig reports the configuration mistakes of the
// break-suppressed tail rules: when the required_if preceding break is not
// met, the tail never joins the value phase, yet its configuration mistakes
// must still surface regardless of the condition (the same
// configuration-errors-never-depend-on-the-field-value class as the
// conflicting boundaries, mutual exclusion and empty choice checks of
// validateValue). The audit mirrors the value-phase configuration checks of
// validateValue — the same checks, the same field-kind filters and the same
// message texts, evaluated with no value comparisons: only declarations are
// judged. That includes the tail regexp and the tail format token: the
// pattern is pre-compiled with the value-phase quote stripping (never
// matched) and an unknown format token names no reachable check — both are
// declaration faults, reported with the value-phase messages.
//
// Boundary conflicts that span the head and the tail (min/max in one
// segment, gt/lt in the other) are the same declaration conflict the flat
// engine reports regardless of the field value, so the audit reports them
// regardless of the condition; when the condition is met, validateValue
// names the pair exactly once on the merged rule set. A cross-segment
// min>max comparison needs the kind-specific parse of the head limit and
// stays the business of validateValue: it surfaces once the condition is
// met. Head-only pairs never reach the audit — the head always joins the
// value phase, so validateValue reports them regardless of the condition
// already.
func auditBreakTailConfig(head, tail map[string]string, v reflect.Value, currentPath string, addRuleConfigErr func(format string, args ...any)) {
	minStr, hasMin := tail["min"]
	maxStr, hasMax := tail["max"]
	ltStr, hasLt := tail["lt"]
	gtStr, hasGt := tail["gt"]

	if (hasMin && hasGt) || (hasMax && hasLt) {
		addRuleConfigErr("field %s: invalid validator configuration: conflicting boundaries metrics", currentPath)
	}

	// The head×tail pairs of the same conflicting class: the merged rule set
	// carries them the moment the condition is met, and the flat engine
	// reports the pair regardless of the field value — a not-met silence
	// would hide a declaration the user believes is active.
	_, headHasMin := head["min"]
	_, headHasMax := head["max"]
	_, headHasLt := head["lt"]
	_, headHasGt := head["gt"]
	if (headHasMin && hasGt) || (headHasGt && hasMin) || (headHasMax && hasLt) || (headHasLt && hasMax) {
		addRuleConfigErr("field %s: invalid validator configuration: conflicting boundaries metrics", currentPath)
	}

	_, hasFormat := tail["format"]
	_, hasEndpoint := tail["endpoint"]
	_, hasURL := tail["url"]

	if hasFormat || hasEndpoint || hasURL {
		mutIdx := 0
		if hasFormat {
			mutIdx++
		}
		if hasEndpoint {
			mutIdx++
		}
		if hasURL {
			mutIdx++
		}
		if mutIdx > 1 {
			addRuleConfigErr("field %s: invalid configuration: format rules are strictly mutually exclusive", currentPath)
		}
	}

	// The tail regexp is a declaration too: the value phase compiles the
	// pattern for string fields before any matching, so the audit
	// pre-compiles it with the same quote stripping and the same message —
	// the field value itself is never matched here.
	if expr, hasRegexp := tail["regexp"]; hasRegexp && v.Kind() == reflect.String {
		expr = stripRuleQuotes(expr)
		if _, err := regexp.Compile(expr); err != nil {
			addRuleConfigErr("field %s: invalid regular expression syntax %q: %w", currentPath, expr, err)
		}
	}

	// An unknown format token is rejected by the value phase for string
	// fields; the audit mirrors that check, the kind filter included — a
	// format rule on a kind the value phase never reaches stays silent here
	// as well.
	if formatStr, hasFormat := tail["format"]; hasFormat && v.Kind() == reflect.String {
		switch formatStr {
		case "ip", "ipv4", "ipv6", "uuid", "hex":
			// Known token: nothing to audit.

		default:
			addRuleConfigErr("field %s: unknown format %q (expected ip, ipv4, ipv6, uuid or hex)", currentPath, formatStr)
		}
	}

	if choiceStr, hasChoice := tail["choice"]; hasChoice && len(parseChoiceValues(choiceStr)) == 0 {
		addRuleConfigErr("field %s: invalid validator configuration: empty choice list", currentPath)
	}

	minLenStr, hasMinLen := tail["minlen"]
	maxLenStr, hasMaxLen := tail["maxlen"]
	minCountStr, hasMinCount := tail["mincount"]
	maxCountStr, hasMaxCount := tail["maxcount"]

	// The limit checks keep the field-kind filters of validateValue: a limit
	// whose value phase would never run for this kind is not audited either.
	switch {
	case v.Kind() == reflect.String:
		auditStringTailLimits(minLenStr, maxLenStr, hasMinLen, hasMaxLen, currentPath, addRuleConfigErr)
		auditUnsupportedTailBoundaries(v, hasMin || hasMax || hasLt || hasGt, currentPath, addRuleConfigErr)

	case v.Kind() == reflect.Slice || v.Kind() == reflect.Map:
		auditCollectionTailLimits(minCountStr, maxCountStr, hasMinCount, hasMaxCount, currentPath, addRuleConfigErr)

	case v.Type() == durationType:
		auditDurationTailLimits(minStr, maxStr, ltStr, gtStr, hasMin, hasMax, hasLt, hasGt, currentPath, addRuleConfigErr)

	case v.Kind() >= reflect.Int && v.Kind() <= reflect.Int64:
		auditIntTailLimits(minStr, maxStr, ltStr, gtStr, hasMin, hasMax, hasLt, hasGt, currentPath, addRuleConfigErr)

	case v.Kind() >= reflect.Uint && v.Kind() <= reflect.Uint64:
		auditUintTailLimits(minStr, maxStr, ltStr, gtStr, hasMin, hasMax, hasLt, hasGt, currentPath, addRuleConfigErr)

	case v.Kind() == reflect.Float32 || v.Kind() == reflect.Float64:
		auditFloatTailLimits(minStr, maxStr, ltStr, gtStr, hasMin, hasMax, hasLt, hasGt, currentPath, addRuleConfigErr)

	default:
		auditUnsupportedTailBoundaries(v, hasMin || hasMax || hasLt || hasGt, currentPath, addRuleConfigErr)
	}
}

// auditStringTailLimits mirrors the minlen/maxlen configuration checks of
// validateValue for string fields: unparseable and negative limits and a
// minlen greater than maxlen are reported, no value comparison happens.
func auditStringTailLimits(minLenStr, maxLenStr string, hasMinLen, hasMaxLen bool, currentPath string, addRuleConfigErr func(format string, args ...any)) {
	var minLen, maxLen int

	if hasMinLen {
		n, err := strconv.Atoi(minLenStr)
		switch {
		case err != nil:
			addRuleConfigErr("field %s: invalid validator configuration: minlen %q: %w", currentPath, minLenStr, err)

		case n < 0:
			addRuleConfigErr("field %s: invalid validator configuration: minlen (%d) cannot be negative", currentPath, n)

		default:
			minLen = n
		}
	}
	if hasMaxLen {
		n, err := strconv.Atoi(maxLenStr)
		switch {
		case err != nil:
			addRuleConfigErr("field %s: invalid validator configuration: maxlen %q: %w", currentPath, maxLenStr, err)

		case n < 0:
			addRuleConfigErr("field %s: invalid validator configuration: maxlen (%d) cannot be negative", currentPath, n)

		default:
			maxLen = n
		}
	}

	if hasMinLen && hasMaxLen && minLen > maxLen {
		addRuleConfigErr("field %s: invalid validator configuration: minlen (%d) cannot be greater than maxlen (%d)", currentPath, minLen, maxLen)
	}
}

// auditCollectionTailLimits mirrors the mincount/maxcount configuration
// checks of validateValue for slice and map fields.
func auditCollectionTailLimits(minCountStr, maxCountStr string, hasMinCount, hasMaxCount bool, currentPath string, addRuleConfigErr func(format string, args ...any)) {
	var minCount, maxCount int

	if hasMinCount {
		n, err := strconv.Atoi(minCountStr)
		switch {
		case err != nil:
			addRuleConfigErr("field %s: invalid validator configuration: mincount %q: %w", currentPath, minCountStr, err)

		case n < 0:
			addRuleConfigErr("field %s: invalid validator configuration: mincount (%d) cannot be negative", currentPath, n)

		default:
			minCount = n
		}
	}
	if hasMaxCount {
		n, err := strconv.Atoi(maxCountStr)
		switch {
		case err != nil:
			addRuleConfigErr("field %s: invalid validator configuration: maxcount %q: %w", currentPath, maxCountStr, err)

		case n < 0:
			addRuleConfigErr("field %s: invalid validator configuration: maxcount (%d) cannot be negative", currentPath, n)

		default:
			maxCount = n
		}
	}

	if hasMinCount && hasMaxCount && minCount > maxCount {
		addRuleConfigErr("field %s: invalid validator configuration: mincount (%d) cannot be greater than maxcount (%d)", currentPath, minCount, maxCount)
	}
}

// auditDurationTailLimits mirrors the min/max/lt/gt configuration checks of
// validateValue for time.Duration fields.
func auditDurationTailLimits(minStr, maxStr, ltStr, gtStr string, hasMin, hasMax, hasLt, hasGt bool, currentPath string, addRuleConfigErr func(format string, args ...any)) {
	var minVal, maxVal time.Duration

	if hasMin {
		d, err := time.ParseDuration(minStr)
		if err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: min %q: %w", currentPath, minStr, err)
		} else {
			minVal = d
		}
	}
	if hasMax {
		d, err := time.ParseDuration(maxStr)
		if err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: max %q: %w", currentPath, maxStr, err)
		} else {
			maxVal = d
		}
	}
	if hasLt {
		if _, err := time.ParseDuration(ltStr); err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: lt %q: %w", currentPath, ltStr, err)
		}
	}
	if hasGt {
		if _, err := time.ParseDuration(gtStr); err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: gt %q: %w", currentPath, gtStr, err)
		}
	}

	if hasMin && hasMax && minVal > maxVal {
		addRuleConfigErr("field %s: configuration error: min %v > max %v", currentPath, minVal, maxVal)
	}
}

// auditIntTailLimits mirrors the min/max/lt/gt configuration checks of
// validateValue for signed integer fields.
func auditIntTailLimits(minStr, maxStr, ltStr, gtStr string, hasMin, hasMax, hasLt, hasGt bool, currentPath string, addRuleConfigErr func(format string, args ...any)) {
	var minVal, maxVal int64

	if hasMin {
		n, err := strconv.ParseInt(minStr, 10, 64)
		if err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: min %q: %w", currentPath, minStr, err)
		} else {
			minVal = n
		}
	}
	if hasMax {
		n, err := strconv.ParseInt(maxStr, 10, 64)
		if err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: max %q: %w", currentPath, maxStr, err)
		} else {
			maxVal = n
		}
	}
	if hasLt {
		if _, err := strconv.ParseInt(ltStr, 10, 64); err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: lt %q: %w", currentPath, ltStr, err)
		}
	}
	if hasGt {
		if _, err := strconv.ParseInt(gtStr, 10, 64); err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: gt %q: %w", currentPath, gtStr, err)
		}
	}

	if hasMin && hasMax && minVal > maxVal {
		addRuleConfigErr("field %s: configuration error: min %d > max %d", currentPath, minVal, maxVal)
	}
}

// auditUintTailLimits mirrors the min/max/lt/gt configuration checks of
// validateValue for unsigned integer fields, including the negative-limit
// rejection that exists only in the unsigned branch.
func auditUintTailLimits(minStr, maxStr, ltStr, gtStr string, hasMin, hasMax, hasLt, hasGt bool, currentPath string, addRuleConfigErr func(format string, args ...any)) {
	var minVal, maxVal uint64

	if hasMin {
		n, err := strconv.ParseInt(minStr, 10, 64)
		switch {
		case err != nil:
			addRuleConfigErr("field %s: invalid validator configuration: min %q: %w", currentPath, minStr, err)

		case n < 0:
			addRuleConfigErr("field %s: invalid validator configuration: min (%d) cannot be negative", currentPath, n)

		default:
			minVal = uint64(n)
		}
	}
	if hasMax {
		n, err := strconv.ParseInt(maxStr, 10, 64)
		switch {
		case err != nil:
			addRuleConfigErr("field %s: invalid validator configuration: max %q: %w", currentPath, maxStr, err)

		case n < 0:
			addRuleConfigErr("field %s: invalid validator configuration: max (%d) cannot be negative", currentPath, n)

		default:
			maxVal = uint64(n)
		}
	}
	if hasLt {
		n, err := strconv.ParseInt(ltStr, 10, 64)
		switch {
		case err != nil:
			addRuleConfigErr("field %s: invalid validator configuration: lt %q: %w", currentPath, ltStr, err)

		case n < 0:
			addRuleConfigErr("field %s: invalid validator configuration: lt (%d) cannot be negative", currentPath, n)
		}
	}
	if hasGt {
		n, err := strconv.ParseInt(gtStr, 10, 64)
		switch {
		case err != nil:
			addRuleConfigErr("field %s: invalid validator configuration: gt %q: %w", currentPath, gtStr, err)

		case n < 0:
			addRuleConfigErr("field %s: invalid validator configuration: gt (%d) cannot be negative", currentPath, n)
		}
	}

	if hasMin && hasMax && minVal > maxVal {
		addRuleConfigErr("field %s: configuration error: min %d > max %d", currentPath, minVal, maxVal)
	}
}

// auditFloatTailLimits mirrors the min/max/lt/gt configuration checks of
// validateValue for floating-point fields.
func auditFloatTailLimits(minStr, maxStr, ltStr, gtStr string, hasMin, hasMax, hasLt, hasGt bool, currentPath string, addRuleConfigErr func(format string, args ...any)) {
	var minVal, maxVal float64

	if hasMin {
		f, err := strconv.ParseFloat(minStr, 64)
		if err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: min %q: %w", currentPath, minStr, err)
		} else {
			minVal = f
		}
	}
	if hasMax {
		f, err := strconv.ParseFloat(maxStr, 64)
		if err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: max %q: %w", currentPath, maxStr, err)
		} else {
			maxVal = f
		}
	}
	if hasLt {
		if _, err := strconv.ParseFloat(ltStr, 64); err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: lt %q: %w", currentPath, ltStr, err)
		}
	}
	if hasGt {
		if _, err := strconv.ParseFloat(gtStr, 64); err != nil {
			addRuleConfigErr("field %s: invalid validator configuration: gt %q: %w", currentPath, gtStr, err)
		}
	}

	if hasMin && hasMax && minVal > maxVal {
		addRuleConfigErr("field %s: configuration error: min %f > max %f", currentPath, minVal, maxVal)
	}
}

// auditUnsupportedTailBoundaries mirrors the unsupported-kind branch of the
// validateValue boundary checks: boundary rules declared on a kind that
// carries no numeric value (except collections, whose elements receive the
// boundaries through the traversal) are a configuration mistake.
func auditUnsupportedTailBoundaries(v reflect.Value, hasBoundaries bool, currentPath string, addRuleConfigErr func(format string, args ...any)) {
	kind := v.Kind()
	if hasBoundaries && kind != reflect.Slice && kind != reflect.Map && kind != reflect.Array {
		addRuleConfigErr("field %s: boundary validators are not supported for kind %s", currentPath, kind)
	}
}
