# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

Other languages: [Русский (RU)](README.ru.md) | [中文 (ZH)](README.zh.md)

A robust, high-performance drop-in replacement for `go.yaml.in/yaml/v4`. It preserves original function signatures and core options (`Option`) while automatically integrating deep recursive field defaults (`default:"value"`), native OS environment variable injection (`env:"VAR"`), automated CLI help generation (`Help()`), and strict tag-based validation (`validate:"..."`) right into the parsing layer.

---

## Available Validation Rules & Features

You can combine multiple validation rules inside the `validate` tag using a comma separator (e.g., `validate:"not_empty,endpoint"`).


| Rule / Feature            | Syntax Example                     | Description                                                                                                                        | Supported Types                             |
|:--------------------------|:-----------------------------------|:-----------------------------------------------------------------------------------------------------------------------------------|:--------------------------------------------|
| **Environment Injection** | `env:"APP_PORT"`                   | **12-Factor App:** Dynamically injects system environment variables. Takes absolute precedence over YAML and tag defaults.         | `string`, `bool`, primitive numbers         |
| **Required Field**        | `validate:"not_empty"`             | Ensures the field is assigned a non-zero value. Mutually exclusive with the `default` tag.                                         | `string`, `struct`, `slice`, `map`, numbers |
| **Conditional Required**  | `validate:"required_if=Mode:prod"` | **Cross-field Validation:** Field becomes required based on another field value. Supports macro tokens `:empty` and `:not_empty`.  | `string`, numbers, `bool` etc.              |
| **Whitelist Choices**     | `validate:"choice=dev,prod"`       | **Whitelist mode:** The string value must perfectly match one of the comma-separated tokens. Supports slice elements recursively.  | `string`, `[]string`                        |
| **Blacklist Choices**     | `validate:"choice=!red,!black"`    | **Blacklist mode:** Allows any string value except for the specific exclusion tokens prefixed with `!`.                            | `string`, `[]string`                        |
| **Numeric & Duration**    | `validate:"min=1s,max=10m"`        | Enforces atomic **inclusive** lower (`>=`) and upper (`<=`) boundaries. Natively supports numbers and `time.Duration` intervals.   | `int`, `uint`, `float`, `time.Duration`     |
| **Strict Comparison**     | `validate:"gt=5,lt=10"`            | Enforces **exclusive** strict greater than (`>`) and less than (`<`) boundaries. Also supports `time.Duration`.                    | `int`, `uint`, `float`, `time.Duration`     |
| **String Length (Runes)** | `validate:"minlen=3,maxlen=20"`    | Enforces minimum and maximum limits on string length. Professionally counts **Unicode Runes** instead of raw bytes.                | `string`                                    |
| **Collection Capacity**   | `validate:"mincount=1,maxcount=5`  | Enforces minimum and maximum elements count inside dynamic collections.                                                            | `slice`, `map`                              |
| **Network Formats**       | `validate:"format=ipv4"`           | Validates specific IP networks layout. Supports `format=ip`, `format=ipv4`, and `format=ipv6`. Mutually exclusive with `endpoint`. | `string`                                    |
| **Unique Identity**       | `validate:"format=uuid"`           | Formats verification using strict RFC-compliant `google/uuid` sub-parsing layer routines.                                          | `string`                                    |
| **Regular Expression**    | `validate:"regexp=^[a-z]{2,4}$"`   | Validates string layout matching a regular expression pattern. Safe against embedded commas.                                       | `string`, `[]string`                        |
| **Endpoint**              | `validate:"endpoint"`              | Enforces standard network endpoints (`host:port`). Natively checks **IPv6** syntax and port bounds (1-65535).                      | `string`                                    |
| **Enforced URL**          | `validate:"url"`                   | Checks Uniform Resource Identifiers. Strictly requires an explicit protocol scheme separator (e.g., `http://`, `grpc://`).         | `string`                                    |

### Professional Architecture Notes
* **Error Aggregation Engine:** The engine does not panic or return early on the first error. It processes the entire configuration downstream topology tree and aggregates all validation violations into a single, beautifully organized error report stack.
* **Mutually Exclusive Format Rules:** To prevent structural anomalies, networking rule assignments (`format=ip`, `format=ipv4`, `format=ipv6`, `endpoint`, and `url`) are strictly mutually exclusive on the same structural field.
* **Configuration Safety Layer:** Invalid configurations (e.g., `min=10, max=5`, `minlen=5, maxlen=2`, or mixing conflicting boundaries like `min` and `gt`) are immediately caught at start-up time during tag evaluation.

---

## Configuration Profiles (Valid & Invalid Cases)

### Example 1: Standard Application Config with Help Output (Valid Profile)

```go
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type Config struct {
	Env         string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod"`
	AppName     string        `yaml:"app_name" default:"api" validate:"minlen=3,maxlen=10"`
	APIUrl      string        `yaml:"api_url" validate:"url,required_if=Env:prod"`
	Timeout     time.Duration `yaml:"timeout" default:"5s" validate:"min=1s,max=10m"`
	BindAddress string        `yaml:"bind_addr" validate:"endpoint"`
	ServerIP    string        `yaml:"server_ip" validate:"format=ipv4"`
	ClusterID   string        `yaml:"cluster_id" validate:"format=uuid"`
	Tags        []string      `yaml:"tags" validate:"mincount=1,maxcount=5"`
}

func main() {
	var cfg Config
	yaml.Help(cfg)
}
```
