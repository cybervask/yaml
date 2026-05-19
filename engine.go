package yaml

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// durationType caches the reflect.Type of time.Duration for efficient type matching.
var durationType = reflect.TypeOf(time.Duration(0))

// SetDefaults recursively walks through the exported fields of a structure pointer
// and populates uninitialized zero-value fields with the data defined in their `default` tags.
//
// System environment variables configured via the `env` tag are evaluated during
// execution and take absolute precedence over standard tag defaults.
//
// It returns an error if a design conflict is detected where both 'default' and
// 'not_empty' validate constraints are declared on the same structure field.
func SetDefaults(ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil
	}
	return setDefaultsValue(v.Elem())
}

// setDefaultsValue performs the underlying recursive assignment of defaults and environment overrides.
func setDefaultsValue(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldVal := v.Field(i)
			fieldType := t.Field(i)

			validateTag, hasValidate := fieldType.Tag.Lookup("validate")
			defaultValStr, hasDefault := fieldType.Tag.Lookup("default")

			// Validate architectural design integrity during tag compilation
			if hasDefault && hasValidate && strings.Contains(validateTag, "not_empty") {
				return fmt.Errorf("field %s is invalid: 'default' and 'not_empty' are mutually exclusive", fieldType.Name)
			}

			// Safe fallback loop step for legacy internal property wrappers
			if fieldType.Name == "Value" {
				if err := setDefaultsValue(fieldVal); err != nil {
					return err
				}
				continue
			}

			// Safely instantiate and populate uninitialized slices (e.g. default:"h2,http/1.1")
			if fieldVal.Kind() == reflect.Slice {
				if hasDefault && fieldVal.IsZero() {
					if fieldVal.Type().Elem().Kind() == reflect.String {
						elements := strings.Split(defaultValStr, ",")
						sliceValues := reflect.MakeSlice(fieldVal.Type(), len(elements), len(elements))
						for idx, elem := range elements {
							sliceValues.Index(idx).SetString(strings.TrimSpace(elem))
						}
						fieldVal.Set(sliceValues)
					}
				}
				if err := setDefaultsValue(fieldVal); err != nil {
					return err
				}
				continue
			}

			// Deep recursive processing step for downstream nested structures and maps
			if fieldVal.Kind() == reflect.Struct || fieldVal.Kind() == reflect.Map {
				if err := setDefaultsValue(fieldVal); err != nil {
					return err
				}
				continue
			}

			envVarName := fieldType.Tag.Get("env")
			var targetValStr string
			var hasValueToSet bool

			// Priority 1: System environment variable evaluation override
			if envVarName != "" {
				if envVal, isSet := os.LookupEnv(envVarName); isSet && envVal != "" {
					targetValStr = envVal
					hasValueToSet = true
				}
			}

			// Priority 2: Fallback to static tag configuration defaults
			if !hasValueToSet && hasDefault {
				targetValStr = defaultValStr
				hasValueToSet = true
			}

			// Determine if a live environment variable requires an explicit override layer
			// over an already parsed struct configuration value
			isEnvOverride := envVarName != "" && os.Getenv(envVarName) != ""

			if hasValueToSet && (fieldVal.IsZero() || isEnvOverride) {
				// Parse specialized duration configuration syntax
				if fieldVal.Type() == durationType {
					d, err := time.ParseDuration(targetValStr)
					if err != nil {
						return fmt.Errorf("invalid duration %q for field %s", targetValStr, fieldType.Name)
					}
					fieldVal.Set(reflect.ValueOf(d))
					continue
				}

				switch fieldVal.Kind() {
				case reflect.String:
					fieldVal.SetString(targetValStr)
				case reflect.Bool:
					b, err := strconv.ParseBool(targetValStr)
					if err == nil {
						fieldVal.SetBool(b)
					}
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					n, err := strconv.ParseInt(targetValStr, 10, 64)
					if err == nil {
						fieldVal.SetInt(n)
					}
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					n, err := strconv.ParseUint(targetValStr, 10, 64)
					if err == nil {
						fieldVal.SetUint(n)
					}
				}
			}
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			element := v.Index(i)
			if element.Kind() == reflect.Ptr {
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
			if element.Kind() == reflect.Ptr {
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

// parseValidateTag tokenizes the validation tag string into separate rule mappings.
// It isolates parameters even if they contain punctuation like commas (e.g. inside regex patterns).
func parseValidateTag(tag string) map[string]string {
	rules := make(map[string]string)
	if tag == "" {
		return rules
	}

	markers := []string{"choice=", "min=", "max=", "regexp=", "host_port", "url", "not_empty"}
	workingTag := tag

	for {
		firstIdx := -1
		firstMarker := ""
		for _, m := range markers {
			idx := strings.Index(workingTag, m)
			if idx != -1 && (firstIdx == -1 || idx < firstIdx) {
				firstIdx = idx
				firstMarker = m
			}
		}

		if firstIdx == -1 {
			remaining := strings.TrimSpace(workingTag)
			if remaining == "not_empty" || remaining == "host_port" || remaining == "url" {
				rules[remaining] = ""
			}
			break
		}

		before := workingTag[:firstIdx]
		if strings.Contains(before, "not_empty") {
			rules["not_empty"] = ""
		}
		if strings.Contains(before, "host_port") {
			rules["host_port"] = ""
		}
		if strings.Contains(before, "url") {
			rules["url"] = ""
		}

		workingTag = workingTag[firstIdx+len(firstMarker):]

		nextIdx := -1
		for _, m := range markers {
			idx := strings.Index(workingTag, m)
			if idx != -1 && (nextIdx == -1 || idx < nextIdx) {
				nextIdx = idx
			}
		}

		var value string
		if nextIdx == -1 {
			value = workingTag
			workingTag = ""
		} else {
			value = workingTag[:nextIdx]
			workingTag = workingTag[nextIdx:]
		}

		value = strings.TrimSpace(value)
		value = strings.TrimSuffix(value, ",")
		value = strings.TrimPrefix(value, ",")
		value = strings.TrimSpace(value)

		key := strings.TrimSuffix(firstMarker, "=")
		rules[key] = value

		if workingTag == "" {
			break
		}
	}

	return rules
}

// Validate executes deep, recursive structural checks across the application configuration models.
// It enforces tag constraints listed inside `validate` annotations, including:
//   - choice (validation against comma-separated white/blacklists)
//   - min / max (range verification for numeric kinds)
//   - regexp (regular expression pattern matching validation)
//   - host_port (verifies valid physical string network socket address structures)
//   - url (verifies valid Absolute RFC-compliant URL patterns)
//   - not_empty (guarantees properties cannot contain zero values)
func Validate(ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil
	}
	return validateValue(v.Elem(), "", nil)
}

// validateValue performs automated constraint checks down the configuration node hierarchy tree.
func validateValue(v reflect.Value, currentPath string, rules map[string]string) error {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	// Step 1: Evaluate validation constraint rules over the CURRENT node value scope
	if len(rules) > 0 {
		if _, hasNotEmpty := rules["not_empty"]; hasNotEmpty && v.IsZero() {
			return fmt.Errorf("field %s: is empty, but required by 'not_empty'", currentPath)
		}

		isCollectionElement := strings.Contains(currentPath, "[")
		minStr, hasMin := rules["min"]

		if v.IsZero() && !isCollectionElement && !hasMin {
			// Skip unassigned optional static fields safely if no explicit boundaries constraint exists
		} else {
			if choiceStr, hasChoice := rules["choice"]; hasChoice && v.Kind() == reflect.String {
				valStr := v.String()
				allowedChoices := strings.Split(choiceStr, ",")
				isBlacklist := true
				for _, c := range allowedChoices {
					c = strings.TrimSpace(c)
					if c != "" && !strings.HasPrefix(c, "!") {
						isBlacklist = false
						break
					}
				}

				if isBlacklist {
					for _, c := range allowedChoices {
						forbidden := strings.TrimPrefix(strings.TrimSpace(c), "!")
						if valStr == forbidden {
							return fmt.Errorf("field %s: value %q is forbidden by blacklist [%s]", currentPath, valStr, choiceStr)
						}
					}
				} else {
					isValid := false
					for _, c := range allowedChoices {
						if valStr == strings.TrimSpace(c) {
							isValid = true
							break
						}
					}
					if !isValid {
						return fmt.Errorf("field %s: value %q is invalid; allowed choices are [%s]", currentPath, valStr, choiceStr)
					}
				}
			}

			if expr, hasRegexp := rules["regexp"]; hasRegexp && v.Kind() == reflect.String {
				valStr := v.String()
				re, err := regexp.Compile(expr)
				if err != nil {
					return fmt.Errorf("field %s: invalid regular expression syntax %q: %w", currentPath, expr, err)
				}
				if !re.MatchString(valStr) {
					return fmt.Errorf("field %s: value %q does not match regular expression %q", currentPath, valStr, expr)
				}
			}

			if _, hasHostPort := rules["host_port"]; hasHostPort && v.Kind() == reflect.String {
				valStr := v.String()
				host, port, err := net.SplitHostPort(valStr)
				if err != nil {
					return fmt.Errorf("field %s: value %q is not a valid host:port format: %w", currentPath, valStr, err)
				}
				p, err := strconv.Atoi(port)
				if err != nil || p < 1 || p > 65535 {
					return fmt.Errorf("field %s: value %q contains an invalid port number", currentPath, valStr)
				}
				if strings.Contains(host, ":") {
					if ip := net.ParseIP(host); ip == nil {
						return fmt.Errorf("field %s: value %q contains an invalid IPv6 address", currentPath, valStr)
					}
				}
			}

			if _, hasURL := rules["url"]; hasURL && v.Kind() == reflect.String {
				valStr := v.String()
				if !strings.Contains(valStr, "://") {
					return fmt.Errorf("field %s: value %q is missing a URL scheme separator (e.g., scheme://host)", currentPath, valStr)
				}
				parsedURL, err := url.ParseRequestURI(valStr)
				if err != nil {
					return fmt.Errorf("field %s: value %q is not a valid URL: %w", currentPath, valStr, err)
				}
				if parsedURL.Scheme == "" {
					return fmt.Errorf("field %s: value %q has an empty or invalid URL scheme", currentPath, valStr)
				}
			}

			maxStr, hasMax := rules["max"]
			if hasMin && hasMax {
				kind := v.Kind()
				switch {
				case kind >= reflect.Int && kind <= reflect.Int64:
					val := v.Int()
					minVal, _ := strconv.ParseInt(minStr, 10, 64)
					maxVal, _ := strconv.ParseInt(maxStr, 10, 64)
					if val < minVal || val > maxVal {
						return fmt.Errorf("field %s: value %d out of range [%s..%s]", currentPath, val, minStr, maxStr)
					}
				case kind >= reflect.Uint && kind <= reflect.Uint64:
					val := v.Uint()
					minInt, _ := strconv.ParseInt(minStr, 10, 64)
					maxInt, _ := strconv.ParseInt(maxStr, 10, 64)
					if minInt < 0 || maxInt < 0 {
						return fmt.Errorf("field %s: invalid uint validation limits; min/max cannot be negative ([%s..%s])", currentPath, minStr, maxStr)
					}
					if val < uint64(minInt) || val > uint64(maxInt) {
						return fmt.Errorf("field %s: value %d out of range [%s..%s]", currentPath, val, minStr, maxStr)
					}
				case kind == reflect.Float32 || kind == reflect.Float64:
					val := v.Float()
					minVal, _ := strconv.ParseFloat(minStr, 64)
					maxVal, _ := strconv.ParseFloat(maxStr, 64)
					if val < minVal || val > maxVal {
						return fmt.Errorf("field %s: value %f out of range [%f..%f]", currentPath, val, minVal, maxVal)
					}
				}
			}
		}
	}

	// Step 2: Traverse downstream recursively based on structural target types topology
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldVal := v.Field(i)
			fieldType := t.Field(i)

			nextPath := fieldType.Name
			if currentPath != "" {
				nextPath = currentPath + "." + fieldType.Name
			}

			if fieldType.Name == "Value" {
				if err := validateValue(fieldVal, currentPath, rules); err != nil {
					return err
				}
				continue
			}

			var fieldRules map[string]string
			if validateTag, hasValidate := fieldType.Tag.Lookup("validate"); hasValidate {
				fieldRules = parseValidateTag(validateTag)
			}

			if err := validateValue(fieldVal, nextPath, fieldRules); err != nil {
				return err
			}
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			element := v.Index(i)
			indexedPath := fmt.Sprintf("%s[%d]", currentPath, i)
			if err := validateValue(element, indexedPath, rules); err != nil {
				return err
			}
		}

	case reflect.Map:
		for _, key := range v.MapKeys() {
			element := v.MapIndex(key)
			indexedPath := fmt.Sprintf("%s[%v]", currentPath, key.Interface())
			if err := validateValue(element, indexedPath, rules); err != nil {
				return err
			}
		}
	}

	return nil
}
