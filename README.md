# Smart Go-YAML Engine  

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

Other languages: [Русский (RU)](README.ru.md) | [中文 (ZH)](README.zh.md)

A robust, high-performance drop-in replacement for `go.yaml.in/yaml/v4`. It preserves original function signatures and core options (`Option`) while automatically integrating deep recursive field defaults (`default:"value"`) and strict tag-based environment validation (`validate:"..."`) right into the parsing layer.

---

## Available Validation Rules

You can combine multiple validation rules inside the `validate` tag using a comma separator (e.g., `validate:"not_empty,host_port"`).


| Rule                   | Syntax Example                     | Description                                                                                                                | Supported Types                             |
|:-----------------------|:-----------------------------------|:---------------------------------------------------------------------------------------------------------------------------|:--------------------------------------------|
| **Required Field**     | `validate:"not_empty"`             | Ensures the field is assigned a non-zero value. Mutually exclusive with the `default` tag.                                 | `string`, `struct`, `slice`, `map`, numbers |
| **Whitelist Choices**  | `validate:"choice=dev,stage,prod"` | **Whitelist mode:** The string value must perfectly match one of the comma-separated tokens.                               | `string`                                    |
| **Blacklist Choices**  | `validate:"choice=!red,!black"`    | **Blacklist mode:** Allows any string value except for the specific exclusion tokens prefixed with `!`.                    | `string`                                    |
| **Numeric Range**      | `validate:"min=1,max=100"`         | Enforces strict lower and upper boundaries. Automatically blocks negative parameters for `uint` fields.                    | `int`, `uint`, `float` variants             |
| **Regular Expression** | `validate:"regexp=^[a-z]{2,4}$"`   | Validates string layout matching a regular expression pattern. Safe against embedded commas.                               | `string`                                    |
| **Network Socket**     | `validate:"host_port"`             | Enforces standard network endpoints (`host:port`). Natively checks **IPv6** syntax and port bounds (1-65535).              | `string`                                    |
| **Enforced URL**       | `validate:"url"`                   | Checks Uniform Resource Identifiers. Strictly requires an explicit protocol scheme separator (e.g., `http://`, `grpc://`). | `string`                                    |

---

## Configuration Profiles (Valid & Invalid Cases)

### Example 1: Standard Application Config (Valid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "prod"
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
# 'workers' and 'server.logging.timeout' are omitted and will gracefully fallback to tags defaults
server:
  logging:
    colors: true
    level: "warn"
```
**Go Application Configuration Model:**
```go
type Logging struct {
	yaml.Includer `yaml:",inline"` // Activates !include support safely
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn"`
	Colors        bool             `yaml:"colors"`
	Timeout       string           `yaml:"timeout" default:"5s"`
}

type Config struct {
	Env         string  `yaml:"env" default:"dev" validate:"choice=dev,stage,prod"`
	APIUrl      string  `yaml:"api_url" validate:"url"`
	APIHostPort string  `yaml:"api_host_port" validate:"host_port"`
	Workers     int     `yaml:"workers" default:"10" validate:"min=1,max=100"`
	Server      struct {
		Logging Logging `yaml:"logging"`
	} `yaml:"server"`
}
```
**Serialized Go Structure Matrix (via `yaml.Dump`):**
```yaml
env: prod
api_url: https://cybervask.net
api_host_port: 127.0.0.1:443
workers: 10
server:
  logging:
    level: warn
    colors: true
    timeout: 5s
```

---

### Example 2: Choice Whitelist Constraint Violation (Invalid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "testing" # Error: "testing" is outside the white-list bounds [dev, stage, prod]
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field Env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### Example 3: URL Format & Port Boundaries Anomalies (Invalid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "dev"
api_url: "cybervask.net:443"  # Error: Explicit scheme divider "://" is completely missing
api_host_port: "127.0.0.1:85000" # Error: Target socket port parameter exceeds limits (> 65535)
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field APIUrl: value "cybervask.net:443" is missing a URL scheme separator (e.g., scheme://host)
```

---

### Example 4: Mutually Exclusive Structure Design Panic (Invalid Model Setup)
**Go Code Model Definition:**
```go
type DefectiveConfig struct {
    Token string `yaml:"token" default:"secret_token" validate:"not_empty"`
}
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field Token is invalid: 'default' and 'not_empty' are mutually exclusive
```

