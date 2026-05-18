package yaml

import (
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var durationType = reflect.TypeOf(time.Duration(0))

// SetDefaults parses structural models, evaluating properties recursively to populate uninitialized
// zero-state properties with raw matching default configurations declared inside `default` struct tag parameters.
// Returns an error compilation boundary if structural designs contain logically conflicting 'default' and 'not_empty' tags.
func SetDefaults(ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil
	}
	return setDefaultsValue(v.Elem())
}

func setDefaultsValue(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldVal := v.Field(i)
			fieldType := t.Field(i)

			validateTag, hasValidate := fieldType.Tag.Lookup("validate")
			_, hasDefault := fieldType.Tag.Lookup("default")
			if hasDefault && hasValidate && strings.Contains(validateTag, "not_empty") {
				return fmt.Errorf("field %s is invalid: 'default' and 'not_empty' are mutually exclusive", fieldType.Name)
			}

			if fieldType.Anonymous && fieldType.Type == reflect.TypeOf(Includer{}) {
				continue
			}

			if fieldType.Name == "Value" {
				if err := setDefaultsValue(fieldVal); err != nil {
					return err
				}
				continue
			}

			if fieldVal.Kind() == reflect.Struct || fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Map {
				if err := setDefaultsValue(fieldVal); err != nil {
					return err
				}
				continue
			}

			defaultValStr, hasDefault := fieldType.Tag.Lookup("default")
			if !hasDefault {
				continue
			}

			if fieldVal.IsZero() {
				if fieldVal.Type() == durationType {
					d, err := time.ParseDuration(defaultValStr)
					if err != nil {
						return fmt.Errorf("invalid duration %q for field %s", defaultValStr, fieldType.Name)
					}
					fieldVal.Set(reflect.ValueOf(d))
					continue
				}

				switch fieldVal.Kind() {
				case reflect.String:
					fieldVal.SetString(defaultValStr)
				case reflect.Bool:
					b, err := strconv.ParseBool(defaultValStr)
					if err == nil {
						fieldVal.SetBool(b)
					}
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					n, err := strconv.ParseInt(defaultValStr, 10, 64)
					if err == nil {
						fieldVal.SetInt(n)
					}
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					n, err := strconv.ParseUint(defaultValStr, 10, 64)
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
				element = element.Elem()
			}
			if err := setDefaultsValue(element); err != nil {
				return err
			}
		}

	case reflect.Map:
		for _, key := range v.MapKeys() {
			element := v.MapIndex(key)
			if element.Kind() == reflect.Struct {
				copyElem := reflect.New(element.Type()).Elem()
				copyElem.Set(element)
				if err := setDefaultsValue(copyElem); err != nil {
					return err
				}
				v.SetMapIndex(key, copyElem)
			} else if element.Kind() == reflect.Ptr {
				if err := setDefaultsValue(element.Elem()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Isolate internal parameter segments properly (handles structural rules mapping blocks with embedded values).
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

// Validate executes deep, recursive property sweeps across runtime models to check that assigned values
// align perfectly with tag rule restrictions listed inside `validate` annotations (choice, min/max, regexp, host_port, url, not_empty).
func Validate(ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil
	}
	return validateValue(v.Elem(), "")
}

func validateValue(v reflect.Value, fieldNamePrefix string) error {
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldVal := v.Field(i)
			fieldType := t.Field(i)

			currentPath := fieldType.Name
			if fieldNamePrefix != "" {
				currentPath = fieldNamePrefix + "." + fieldType.Name
			}

			if fieldType.Anonymous && fieldType.Type == reflect.TypeOf(Includer{}) {
				continue
			}

			if fieldType.Name == "Value" {
				if err := validateValue(fieldVal, fieldNamePrefix); err != nil {
					return err
				}
				continue
			}

			if fieldVal.Kind() == reflect.Struct || fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Map {
				if err := validateValue(fieldVal, currentPath); err != nil {
					return err
				}
				continue
			}

			validateTag, hasValidate := fieldType.Tag.Lookup("validate")
			if !hasValidate {
				continue
			}

			rules := parseValidateTag(validateTag)

			if _, hasNotEmpty := rules["not_empty"]; hasNotEmpty && fieldVal.IsZero() {
				return fmt.Errorf("field %s: is empty, but required by 'not_empty'", currentPath)
			}

			kind := fieldVal.Kind()
			if fieldVal.IsZero() {
				// Строки, слайсы, мапы, указатели при IsZero() можно пропускать, если нет not_empty
				if kind == reflect.String || kind == reflect.Slice || kind == reflect.Map || kind == reflect.Ptr {
					continue
				}
			}

			if choiceStr, hasChoice := rules["choice"]; hasChoice && fieldVal.Kind() == reflect.String {
				valStr := fieldVal.String()
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

			if expr, hasRegexp := rules["regexp"]; hasRegexp && fieldVal.Kind() == reflect.String {
				valStr := fieldVal.String()
				re, err := regexp.Compile(expr)
				if err != nil {
					return fmt.Errorf("field %s: invalid regular expression syntax %q: %w", currentPath, expr, err)
				}
				if !re.MatchString(valStr) {
					return fmt.Errorf("field %s: value %q does not match regular expression %q", currentPath, valStr, expr)
				}
			}

			if _, hasHostPort := rules["host_port"]; hasHostPort && fieldVal.Kind() == reflect.String {
				valStr := fieldVal.String()
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

			if _, hasURL := rules["url"]; hasURL && fieldVal.Kind() == reflect.String {
				valStr := fieldVal.String()
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

			minStr, hasMin := rules["min"]
			maxStr, hasMax := rules["max"]
			if hasMin && hasMax {
				kind := fieldVal.Kind()
				switch {
				case kind >= reflect.Int && kind <= reflect.Int64:
					val := fieldVal.Int()
					minVal, _ := strconv.ParseInt(minStr, 10, 64)
					maxVal, _ := strconv.ParseInt(maxStr, 10, 64)
					if val < minVal || val > maxVal {
						return fmt.Errorf("field %s: value %d out of range [%s..%s]", currentPath, val, minStr, maxStr)
					}
				case kind >= reflect.Uint && kind <= reflect.Uint64:
					val := fieldVal.Uint()
					minInt, _ := strconv.ParseInt(minStr, 10, 64)
					maxInt, _ := strconv.ParseInt(maxStr, 10, 64)
					if minInt < 0 || maxInt < 0 {
						return fmt.Errorf("field %s: invalid uint validation limits; min/max cannot be negative ([%s..%s])", currentPath, minStr, maxStr)
					}
					if val < uint64(minInt) || val > uint64(maxInt) {
						return fmt.Errorf("field %s: value %d out of range [%s..%s]", currentPath, val, minStr, maxStr)
					}
				case kind == reflect.Float32 || kind == reflect.Float64:
					val := fieldVal.Float()
					minVal, _ := strconv.ParseFloat(minStr, 64)
					maxVal, _ := strconv.ParseFloat(maxStr, 64)
					if val < minVal || val > maxVal {
						return fmt.Errorf("field %s: value %f out of range [%f..%f]", currentPath, val, minVal, maxVal)
					}
				}
			}
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			element := v.Index(i)
			if element.Kind() == reflect.Ptr {
				element = element.Elem()
			}
			if err := validateValue(element, fmt.Sprintf("%s[%d]", fieldNamePrefix, i)); err != nil {
				return err
			}
		}

	case reflect.Map:
		for _, key := range v.MapKeys() {
			element := v.MapIndex(key)
			if element.Kind() == reflect.Ptr {
				element = element.Elem()
			}
			if err := validateValue(element, fmt.Sprintf("%s[%v]", fieldNamePrefix, key.Interface())); err != nil {
				return err
			}
		}
	}
	return nil
}
