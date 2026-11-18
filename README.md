# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

Other languages: [Русский (RU)](README.ru.md) | [中文 (ZH)](README.zh.md) | [日本語 (JA)](README.ja.md) | [Español (ES)](README.es.md)

A robust, high-performance drop-in replacement for `go.yaml.in/yaml/v4`. It preserves original function signatures and core options (`Option`) while automatically integrating deep recursive field defaults (`default:"value"`), native OS environment variable injection (`env:"VAR"`), automated CLI help generation (`Help()`), and strict tag-based validation (`validate:"..."`) right into the parsing layer.

---

## Available Validation Rules & Features

You can combine multiple validation rules inside the `validate` tag using a comma separator (e.g., `validate:"not_empty,endpoint"`).

> **Note on cyclic structures:** programmatically built cyclic pointer structures (e.g., a node referencing itself) do not hang `Validate` — recursion along the current path reports an explicit `cyclic structure detected` error for the referencing field. Shared (acyclic) subobjects referenced from several fields remain valid. In `Help` output, a self-referential type is documented once per path and the recursion is silently omitted.

> **Note on boundary limits:** a limit value that fails to parse (e.g., `validate:"minlen=abc,maxlen=5"`) is reported as an explicit configuration error naming the rule (`invalid validator configuration: minlen "abc": ...`) — an invalid limit never silently disables its check. The same applies to boundary rules (`min`, `max`, `gt`, `lt`) placed on non-numeric kinds (e.g., `bool`): the check cannot be applied, so it is reported as a configuration error instead of being silently skipped.



| Rule / Feature            | Syntax Example                     | Description                                                                                                                                | Supported Types                             |
|:--------------------------|:-----------------------------------|:-------------------------------------------------------------------------------------------------------------------------------------------|:--------------------------------------------|
| **Environment Injection** | `env:"APP_PORT"`                   | **12-Factor App:** Dynamically injects system environment variables. By default the environment variable takes absolute precedence over YAML values and tag defaults; source arbitration is available via the `priority` tag (see **Source Priority**). The `default` and `env` tags apply to scalar fields. Supports `time.Duration` values. In the load paths (`LoadFile`, `Load`, `Loader.Load`, `Decoder.Decode`) an invalid env value is reported as an error; the standalone `SetDefaults` call keeps its legacy silent behavior — an invalid env value leaves the field zero without an error and the `default` tag is not applied (the only exception: an invalid `time.Duration` literal is reported as an error).                 | `string`, `bool`, primitive numbers, `time.Duration` |
| **Source Priority**       | `priority:"value"`                 | **Env arbitration tag:** applies to fields that also carry an `env` tag; values are `value` or `env`. By default (no tag or `priority:"env"`) the environment variable takes absolute precedence over the YAML value and the `default` tag. With `priority:"value"`, a field that is non-zero after defaults and decode — a decoded YAML value, an applied non-zero `default` tag or a pre-filled standalone value — wins over a non-empty environment variable; if only one source is present, it applies regardless of the tag. A field that is still zero after defaults and decode (an explicit zero in YAML or a zero `default` tag such as `default:"0"`, `default:"false"`, `default:""`) does not count as a value for arbitration — the environment variable applies. Unknown tag values and a `priority` tag without `env` are reported as configuration errors, regardless of whether a value or the variable is present. Like `default` and `env`, the `priority` tag applies to scalar fields; on a non-scalar field (struct, slice, map) it is reported as a configuration error. | `string`, `bool`, primitive numbers, `time.Duration` |
| **Required Field**        | `validate:"required"`              | Ensures the field is assigned a non-zero value. Mutually exclusive with the `default` tag. `not_empty` is a legacy alias (kept for backward compatibility, planned for deprecation). | `string`, `struct`, `slice`, `map`, numbers |
| **Conditional Required**  | `validate:"required_if=role!=srv"` | **Cross-field Validation:** Field becomes required based on another field value. Canonical operators: `==` (equal), `!=` (not equal) — comparison is case-insensitive; `~=` (regexp match) and `^=` (regexp mismatch); operands of the canonical operators may be quoted (`'...'` / `"..."`) — quotes are stripped. Bare target names resolve against the enclosing struct first, then the root; dotted paths (`required_if=server.mode==prod`) address the root; pointer targets are dereferenced. Unknown targets, unexported targets and invalid patterns are reported as configuration errors. A `required_if` value without any recognized operator is reported as a configuration error. The legacy colon form (`key:value`, `key:!value`, macros `:empty`/`:not_empty`) is deprecated but still supported; legacy and canonical atoms may be mixed within one expression. Multiple conditions are supported: combine them with `&&` (AND) and `\|\|` (OR), and group with parentheses — `&&` binds tighter than `\|\|`. Conditions parse as an expression only when `&&` or `\|\|` is present outside quotes, or the value starts with an unquoted `(` — parentheses alone can wrap a single condition; otherwise the single-condition syntax and messages stay unchanged. Spaces around `&&`, `\|\|` and parentheses are ignored; spaces around the comparison operators (`==`, `!=`, `~=`, `^=`) inside an atom are not — a target name carrying a space does not resolve and is reported as `target field "..." not found`. An operand containing `&&` or `\|\|` must be quoted: `webhook.url~='(a\|b)'`; a quoted segment is literal. Parentheses inside an operand (after the comparison operator, e.g. `tag~=(a\|b)`) need no quotes. Declaring `required_if` twice on one field is a configuration error — merge the conditions with `&&` or `\|\|` into a single expression. A deprecated legacy value containing `&&` or `\|\|` parses as an expression (e.g. `key:a&&b` becomes the AND of `key:a` and a malformed atom), not a single literal comparison. Quantitative rules (`min`, `max`, `minlen`, `maxlen`, `mincount`, `maxcount`) on the same field are checked regardless of the expression outcome — `minlen` on an empty field reports even when the expression is false. Expression evaluation is exhaustive: shape errors (unbalanced parentheses, empty atoms) and atom configuration mistakes are reported regardless of the field value and the combined result, and a faulty atom contributes false; a repeated identical atom mistake is reported once. A violated expression is reported as `field <name>: is required when <expression>` (e.g. `field key_file: is required when mode==webhook&&(url~=^https:\|\|cert^=^$)`). | `string`, numbers, `bool`, pointers etc.    |
| **Conditional Section** | `validate:"required_if=telegram.mode==webhook,break,url,minlen=10"` | **Conditional Section:** value rules placed after `break` are evaluated only when the preceding required_if condition is met; in other modes the field is not checked at all — including non-empty values (a deliberate trade-off: a section that does not apply in the current mode is not validated). The tail is still parsed always: configuration mistakes after `break` (e.g. minlen=abc) are reported regardless of the condition — the report reads `field url: invalid validator configuration: minlen "abc": ...`. A `break` without a preceding `required_if`, a second `break`, a `required_if` after `break` and `break` inside a path rule are configuration errors. Conditional rules of slice/map collections are not inherited by their elements. | `string`, numbers, `bool`, pointers etc. |
| **Whitelist Choices**     | `validate:"choice=dev,prod"`       | **Whitelist mode:** The string value must perfectly match one of the comma-separated tokens. Supports slice elements recursively.          | `string`, `[]string`                        |
| **Blacklist Choices**     | `validate:"choice=!red,!black"`    | **Blacklist mode:** Allows any string value except for the specific exclusion tokens prefixed with `!`.                                    | `string`, `[]string`                        |
| **Numeric & Duration**    | `validate:"min=1s,max=10m"`        | Enforces atomic **inclusive** lower (`>=`) and upper (`<=`) boundaries. Natively supports numbers and `time.Duration` intervals.           | `int`, `uint`, `float`, `time.Duration`     |
| **Strict Comparison**     | `validate:"gt=5,lt=10"`            | Enforces **exclusive** strict greater than (`>`) and less than (`<`) boundaries. Also supports `time.Duration`.                            | `int`, `uint`, `float`, `time.Duration`     |
| **String Length (Runes)** | `validate:"minlen=3,maxlen=20"`    | Enforces minimum and maximum limits on string length. Professionally counts **Unicode Runes** instead of raw bytes.                        | `string`                                    |
| **Collection Capacity**   | `validate:"mincount=1,maxcount=5"` | Enforces minimum and maximum elements count inside dynamic collections.                                                                    | `slice`, `map`                              |
| **Network Formats**       | `validate:"format=ipv4"`           | Validates specific IP networks layout. Supports `format=ip`, `format=ipv4`, and `format=ipv6`. Mutually exclusive with `endpoint` and `url`. | `string`                                    |
| **Hex Encoding**          | `validate:"format=hex"`            | Validates hexadecimal strings: decodable via `hex.DecodeString` with an even number of digits. Odd lengths and non-hex characters are rejected. | `string`                                    |
| **Unique Identity**       | `validate:"format=uuid"`           | Validates the canonical UUID form (hex 8-4-4-4-12 with hyphens); the version and variant (RFC 4122) are NOT verified.                      | `string`                                    |
| **Regular Expression**    | `validate:"regexp=^[a-z]{2,4}$"`   | Validates string layout matching a regular expression pattern. Safe against embedded commas. Outer quotes (`'...'` or `"..."`) are stripped from the pattern; to match a literal quote, use the opposite quote type (e.g., `regexp="^'q'$"`). | `string`, `[]string`                        |
| **Endpoint**              | `validate:"endpoint"`              | Enforces standard network endpoints (`host:port`). Natively checks **IPv6** syntax and port bounds (1-65535).                              | `string`                                    |
| **Enforced URL**          | `validate:"url"`                   | Checks Uniform Resource Identifiers. Strictly requires an explicit protocol scheme separator (e.g., `http://`, `grpc://`) and a non-empty host: scheme-only values (`http://`) are rejected.                 | `string`                                    |
| **Per-Instance Path Rules** | `validate:"min_len.min=0,max_len.max=64"` | **Shared nested types:** Applies a rule to a nested field of this specific instance via the `<yaml_path>.<rule>=<value>` syntax; each parent instance gets its own bounds. Dangling paths are reported as errors. Rule values may contain dots. | nested struct fields |

---

## Loading & Dumping

- **Options are end-to-end:** every option passed to `Load` / `LoadFile` (e.g., `WithKnownFields`, `WithUniqueKeys`) is applied both to the AST parse and to the final decode into the target structure — strict checking works exactly like the upstream `yaml4.Load`. Format-only options (`WithIndent` and friends) do not affect loading.
- **`!include` write-back is object-bound:** `DumpWithInclude` takes its write-back paths from the include data recorded while loading that very object — pass the same pointer to `DumpWithInclude` that you passed to `Load` / `LoadFile`. A struct that never went through such a load (or a copy of the loaded value) is dumped in a safe mode: values are emitted inline, no `!include` markers are synthesized and no files are modified.
- **`!include` is resolved by all the load paths:** `LoadFile` resolves `!include` directives relative to the directory of the config file, while `Load`, `Loader.Load` and `Decoder.Decode` resolve them against the current working directory. The stream paths bind no include session to the target, and a successful stream load of a pointer that was previously loaded via `Load`/`LoadFile` drops that earlier binding: the object is dumped in the inline safe mode (see the write-back bullet above).
- **One document per include target:** a `!include` target file must hold exactly one YAML document — a trailing `---` document marker makes the load fail with `included file ... contains multiple documents`. The main config follows upstream yaml4 semantics: the one-shot paths (`LoadFile`, `Load`, `Unmarshal`) reject a trailing `---` marker as well (`expected single document, found multiple`), while the stream paths (`Loader.Load`, `Decoder.Decode`) read only the first document and silently drop the rest.

---

## Configuration Profiles (Valid & Invalid Cases)

### Example 1: Standard Application Config with Help Output (Valid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "prod"
api_url: "https://cybervask.net"
api_endpoint: "127.0.0.1:443"
server:
  logging:
    colors: true
    level: "warn"
  allowed_ips:
    - "192.168.1.1"
```

**Go Application Configuration Model:**
```go
package main

import (
	"fmt"
	"time"

	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"Minimum TLS version"`
	Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"Application-Layer Protocol Negotiation"`
}

type Logging struct {
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn" description:"Log hierarchy execution level"`
	Colors        bool             `yaml:"colors" description:"Enable ANSI colored logs output"`
	Timeout       string           `yaml:"timeout" default:"5s" description:"Internal downstream processing timeout"`
}

type Config struct {
	Env          string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"Application execution environment"`
	AppName      string        `yaml:"app_name" default:"api" validate:"minlen=3,maxlen=10" description:"Internal runtime deployment monicker"`
	APIUrl       string        `yaml:"api_url" validate:"url,required_if=env==prod" env:"API_URL" description:"Base target API destination link"`
	APIEndpoint  string        `yaml:"api_endpoint" validate:"endpoint" description:"Local binding network socket"`
	Timeout      time.Duration `yaml:"timeout" default:"5s" validate:"min=1s,max=10m" description:"Global runtime request boundary tracking parameter"`
	Crypto       TLS           `yaml:"crypto" description:"Security TLS structure configuration layer"`
	Server       struct {
		Logging    Logging  `yaml:"logging" description:"Server logging configuration parameters"`
		AllowedIPs []string `yaml:"allowed_ips" validate:"mincount=1,maxcount=10" description:"Whitelisted network remotes"`
	} `yaml:"server"`
}

func main() {
	var cfg Config
	yaml.Help(cfg)
}
```

**Automated Interactive CLI Help Output (`yaml.Help(cfg)`):**

Self-referential struct types are documented once per path — the recursive branch is silently omitted, so `Help` always terminates.
```text
yaml configuration schema documentation:

env:             Application execution environment (default: dev, validate: [choice=dev,stage,prod])
app_name:        Internal runtime deployment monicker (default: api, validate: [minlen=3,maxlen=10])
api_url:         Base target API destination link (env: API_URL, validate: [url,required_if=env==prod])
api_endpoint:    Local binding network socket (validate: [endpoint])
timeout:         Global runtime request boundary tracking parameter (default: 5s, validate: [min=1s,max=10m])
crypto:          Security TLS structure configuration layer
  min_version:   Minimum TLS version (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])
  alpn:          Application-Layer Protocol Negotiation (default: h2,http/1.1, validate: [choice=h2,http/1.1])
server:
  logging:       Server logging configuration parameters
    level:       Log hierarchy execution level (default: info, validate: [choice=debug,info,warn])
    colors:      Enable ANSI colored logs output
    timeout:     Internal downstream processing timeout (default: 5s)
  allowed_ips:   Whitelisted network remotes (validate: [mincount=1,maxcount=10])
```

---

### Example 2: Choice Whitelist Constraint Violation (Invalid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "testing" # Error: "testing" is outside the whitelist bounds [dev, stage, prod]
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### Example 3: Multiple Anomalies, String Length, Collection Size & Duration Constraints (Invalid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "prod"
app_name: "go"         # Error 1: violates minlen=3
api_url: ""            # Error 2: required_if constraint fails under env==prod
timeout: "500ms"       # Error 3: violates strict duration min=1s boundary
server:
  logging:
    level: "debug"     # (Valid)
  allowed_ips: []      # Error 4: violates collection mincount=1
```
**Returned Application Runtime Error Strings Stack:**
```text
field app_name: string length 2 is less than minlen 3
field api_url: is required when field env==prod
field timeout: value 500ms < min 1s
field server.allowed_ips: collection size 0 is less than mincount 1
```

---

## Examples

The `examples/` directory contains runnable programs covering every validation rule and feature. Run any of them from the repository root:

```bash
go run ./examples/01_choice
```

| Example | Demonstrates |
|---------|--------------|
| `01_choice` | Whitelist/blacklist `choice` rules, quoted values containing commas, valid and invalid profiles |
| `02_help` | Automated CLI help generation via `yaml.Help()` from `description` tags |
| `03_min-max` | Inclusive `min`/`max` bounds for numbers and `time.Duration`, valid and out-of-range profiles |
| `04_mincount-maxcount` | Collection size bounds for slices and maps, valid and capacity-violating profiles |
| `05_required_if` | Cross-field `required_if` with the `==`/`!=` operators, the `~=`/`^=` regexp matchers, plus `endpoint` and `url` rules |
| `06_with-include` | `!include` directive resolution during load, including a missing-file failure |
| `07_with_env_overwrite` | Environment variable injection overriding tag defaults, incl. a whitelist violation |
| `08_write-to-includes` | Atomic write-back preserving `!include` structure (`DumpWithInclude`) |
| `10_test` | Include path tracker (`FindIncludeFile`) plus the `DumpWithInclude` rejection path for scalars |
| `11_not_empty-required` | `required` rule and its legacy `not_empty` alias (kept for backward compatibility) |
| `12_gt-lt` | Strict `gt`/`lt` bounds for int, float and `time.Duration` |
| `13_minlen-maxlen` | String length limits counted in Unicode runes |
| `14_format` | `format=ip`, `format=ipv4`, `format=ipv6`, `format=uuid`, `format=hex` |
| `15_regexp` | `regexp` rule: plain patterns, embedded commas, slice elements |
| `16_validation-errors` | Aggregated report of multiple violations across different rules |
| `17_embed` | Per-instance path rules on a shared nested type (`min_len.min=0` vs `min_len.min=4`) |
| `18_required_if-operators` | Canonical `required_if` operators: `==`, `!=`, `~=` and `^=` |
| `19_priority` | Source arbitration between YAML/default values and environment variables via the `priority` tag: both sources present, single source, the untagged default, and the zero-default boundary |
| `20_required_if-logic` | Boolean expressions in `required_if`: `&&`, `\|\|`, parentheses and precedence (`&&` binds tighter than `\|\|`) |
| `21_required_if-break` | Conditional section of rules after `required_if` via `break` |
