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
	"unicode/utf8"
)

// durationType caches the reflect.Type of time.Duration for efficient type matching metrics.
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
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return nil
	}
	return setDefaultsValue(v.Elem())
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

			validateTag, hasValidate := fieldType.Tag.Lookup("validate")
			defaultValStr, hasDefault := fieldType.Tag.Lookup("default")

			if hasDefault && hasValidate && strings.Contains(validateTag, "not_empty") {
				return fmt.Errorf("field %s is invalid: 'default' and 'not_empty' are mutually exclusive", fieldType.Name)
			}

			if fieldType.Name == "Value" {
				if err := setDefaultsValue(fieldVal); err != nil {
					return err
				}
				continue
			}

			if fieldVal.Kind() == reflect.Slice {
				if hasDefault && fieldVal.IsZero() {
					if fieldVal.Type().Elem().Kind() == reflect.String {
						elements := strings.Split(defaultValStr, ",")
						sliceValues := reflect.MakeSlice(fieldVal.Type(), len(elements), len(elements))
						for idx, elem := range elements {
							sliceValues.Index(idx).Set(reflect.ValueOf(strings.TrimSpace(elem)))
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

			if envVarName != "" {
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

			if hasValueToSet && (fieldVal.IsZero() || isEnvOverride) {
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

// parseValidateTag tokenizes the validation tag string into separate rule mappings.
// It isolates parameters even if they contain punctuation like commas (e.g. inside regex patterns).
func parseValidateTag(tag string) map[string]string {
	rules := make(map[string]string)
	if tag == "" {
		return rules
	}

	markers := []string{
		"mincount=", "maxcount=", "endpoint", "not_empty",
		"regexp=", "choice=", "minlen=", "maxlen=", "format=",
		"min=", "max=", "url", "lt=", "gt=", "required_if=",
	}
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
			remaining = strings.Trim(remaining, ",")
			remaining = strings.TrimSpace(remaining)
			if remaining == "not_empty" || remaining == "endpoint" || remaining == "url" {
				rules[remaining] = ""
			}
			break
		}

		before := workingTag[:firstIdx]
		if strings.Contains(before, "not_empty") {
			rules["not_empty"] = ""
		}
		if strings.Contains(before, "endpoint") {
			rules["endpoint"] = ""
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
// It gathers all validation faults into a composite error stack instead of exiting early.
func Validate(ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return nil
	}
	var errs []error
	validateValue(v.Elem(), "", nil, v.Elem(), &errs)
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

// validateValue performs automated constraint checks down the configuration node hierarchy tree.
//
//nolint:gocyclo // Reason: massive tag boundaries processing engine requires extensive conditional logical blocks.
func validateValue(v reflect.Value, currentPath string, rules map[string]string, root reflect.Value, errs *[]error) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	if len(rules) > 0 {
		// 1.1. Basic check for unconditional field requirement
		if _, hasNotEmpty := rules["not_empty"]; hasNotEmpty && v.IsZero() {
			*errs = append(*errs, fmt.Errorf("field %s: is empty, but required by 'not_empty'", currentPath))
		}

		// 1.2. Cross-field conditional validation execution (required_if) positioned at top tier level
		if reqIfStr, hasReqIf := rules["required_if"]; hasReqIf {
			parts := strings.SplitN(reqIfStr, ":", 2)
			if len(parts) == 2 {
				targetFieldName := parts[0]
				expectedValue := parts[1]

				// Resolve targets from names or mapping configurations dynamically
				var targetField reflect.Value
				yamlTargetName := strings.ToLower(targetFieldName)

				if root.Kind() == reflect.Struct {
					rt := root.Type()
					for i := 0; i < root.NumField(); i++ {
						f := rt.Field(i)
						tag := f.Tag.Get("yaml")
						tagParts := strings.Split(tag, ",")
						if f.Name == targetFieldName || (len(tagParts) > 0 && tagParts[0] == targetFieldName) {
							targetField = root.Field(i)
							yamlTargetName = tagParts[0]
							if yamlTargetName == "" {
								yamlTargetName = strings.ToLower(f.Name)
							}
							break
						}
					}
				}

				if targetField.IsValid() {
					actualValueStr := fmt.Sprintf("%v", targetField.Interface())
					if targetField.Kind() == reflect.String {
						actualValueStr = targetField.String()
					}

					conditionMet := false
					isNegation := strings.HasPrefix(expectedValue, "!")
					cleanExpectedValue := strings.TrimPrefix(expectedValue, "!")

					switch cleanExpectedValue {
					case "empty":
						conditionMet = targetField.IsZero()
					case "not_empty":
						conditionMet = !targetField.IsZero()
					default:
						conditionMet = (actualValueStr == cleanExpectedValue)
					}

					if isNegation {
						conditionMet = !conditionMet
					}

					if conditionMet && v.IsZero() {
						*errs = append(*errs, fmt.Errorf("field %s: is required when field %s=%s", currentPath, yamlTargetName, actualValueStr))
					}
				}
			}
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

		if (hasMin && hasGt) || (hasMax && hasLt) {
			*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration: conflicting boundaries metrics", currentPath))
			return
		}

		hasAnyMinBound := hasMin || hasGt
		hasAnyMaxBound := hasMax || hasLt

		if !v.IsZero() || isCollectionElement || hasAnyMinBound || hasAnyMaxBound || hasMinLen || hasMaxLen || hasMinCount || hasMaxCount {

			if hasFormat || rules["endpoint"] != "" || rules["url"] != "" {
				mutIdx := 0
				if hasFormat {
					mutIdx++
				}
				if _, ok := rules["endpoint"]; ok {
					mutIdx++
				}
				if _, ok := rules["url"]; ok {
					mutIdx++
				}
				if mutIdx > 1 {
					*errs = append(*errs, fmt.Errorf("field %s: invalid configuration: format rules are strictly mutually exclusive", currentPath))
					return
				}
			}

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
							*errs = append(*errs, fmt.Errorf("field %s: value %q is forbidden by blacklist [%s]", currentPath, valStr, choiceStr))
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
						*errs = append(*errs, fmt.Errorf("field %s: value %q is invalid; allowed choices are [%s]", currentPath, valStr, choiceStr))
					}
				}
			}

			if expr, hasRegexp := rules["regexp"]; hasRegexp && v.Kind() == reflect.String {
				valStr := v.String()
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
					if err != nil {
						*errs = append(*errs, fmt.Errorf("field %s: value %q is not a valid URL: %w", currentPath, valStr, err))
					} else if parsedURL.Scheme == "" {
						*errs = append(*errs, fmt.Errorf("field %s: value %q has an empty or invalid URL scheme", currentPath, valStr))
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
				}
			}

			if (hasMinLen || hasMaxLen) && v.Kind() == reflect.String {
				strLen := utf8.RuneCountInString(v.String())
				var minLen, maxLen int
				var err error
				if hasMinLen {
					minLen, err = strconv.Atoi(minLenStr)
				}
				if hasMaxLen {
					maxLen, err = strconv.Atoi(maxLenStr)
				}
				if err != nil || minLen < 0 || maxLen < 0 {
					*errs = append(*errs, fmt.Errorf("field %s: invalid validator configuration for string length limits", currentPath))
				} else {
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
				var err error
				if hasMinCount {
					minCount, err = strconv.Atoi(minCountStr)
				}
				if hasMaxCount {
					maxCount, err = strconv.Atoi(maxCountStr)
				}
				if err != nil || minCount < 0 || maxCount < 0 {
					if hasMaxCount && maxCountStr != "" {
						maxCount, _ = strconv.Atoi(maxCountStr)
					}
				}
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

			if hasAnyMinBound || hasAnyMaxBound {
				kind := v.Kind()
				if v.Type() == durationType {
					val := v.Interface().(time.Duration)
					var minVal, maxVal, ltVal, gtVal time.Duration
					var err error
					if hasMin {
						minVal, err = time.ParseDuration(minStr)
					}
					if hasMax {
						maxVal, err = time.ParseDuration(maxStr)
					}
					if hasLt {
						ltVal, err = time.ParseDuration(ltStr)
					}
					if hasGt {
						gtVal, err = time.ParseDuration(gtStr)
					}
					if err != nil {
						*errs = append(*errs, fmt.Errorf("field %s: parse duration limit error: %w", currentPath, err))
					} else {
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
						var err error
						if hasMin {
							minVal, err = strconv.ParseInt(minStr, 10, 64)
						}
						if hasMax {
							maxVal, err = strconv.ParseInt(maxStr, 10, 64)
						}
						if hasLt {
							ltVal, err = strconv.ParseInt(ltStr, 10, 64)
						}
						if hasGt {
							gtVal, err = strconv.ParseInt(gtStr, 10, 64)
						}
						if err != nil {
							*errs = append(*errs, fmt.Errorf("field %s: parse limit error: %w", currentPath, err))
						} else {
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
						parseUintLimit := func(str string) (uint64, error) {
							limit, err := strconv.ParseInt(str, 10, 64)
							if err != nil || limit < 0 {
								return 0, fmt.Errorf("invalid uint limit %s", str)
							}
							return uint64(limit), nil
						}
						var minVal, maxVal, ltVal, gtVal uint64
						var err error
						if hasMin {
							minVal, err = parseUintLimit(minStr)
						}
						if hasMax {
							maxVal, err = parseUintLimit(maxStr)
						}
						if hasLt {
							ltVal, err = parseUintLimit(ltStr)
						}
						if hasGt {
							gtVal, err = parseUintLimit(gtStr)
						}
						if err != nil {
							*errs = append(*errs, fmt.Errorf("field %s: %w", currentPath, err))
						} else {
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
						var err error
						if hasMin {
							minVal, err = strconv.ParseFloat(minStr, 64)
						}
						if hasMax {
							maxVal, err = strconv.ParseFloat(maxStr, 64)
						}
						if hasLt {
							ltVal, err = strconv.ParseFloat(ltStr, 64)
						}
						if hasGt {
							gtVal, err = strconv.ParseFloat(gtStr, 64)
						}
						if err != nil {
							*errs = append(*errs, fmt.Errorf("field %s: parse float error: %w", currentPath, err))
						} else {
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
				validateValue(fieldVal, currentPath, rules, root, errs)
				continue
			}
			var fieldRules map[string]string
			if validateTag, hasValidate := fieldType.Tag.Lookup("validate"); hasValidate {
				fieldRules = parseValidateTag(validateTag)
			}
			validateValue(fieldVal, nextPath, fieldRules, root, errs)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			validateValue(v.Index(i), fmt.Sprintf("%s[%d]", currentPath, i), rules, root, errs)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			validateValue(v.MapIndex(key), fmt.Sprintf("%s[%v]", currentPath, key.Interface()), rules, root, errs)
		}
	}
}
