# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

其他语言: [English (EN)](README.md) | [Русский (RU)](README.ru.md) | [日本語 (JA)](README.ja.md) | [Español (ES)](README.es.md)

一个高内聚、可直接无缝替换官方 `go.yaml.in/yaml/v4` 的替代包。它完美保留了原生的函数签名和核心配置选项 (`Option`)，但在解析阶段自动集成了深层递归默认值填充 (`default:"value"`)、原生操作系统环境变量注入 (`env:"VAR"`)、自动化命令行帮助文本生成 (`Help()`) 与严格基础设施标签校验 (`validate:"..."`) 引擎。

---

## 可切用的验证规则与特性功能说明

您可以通过逗号分隔符在一个 `validate` 标签中组合多个验证规则（例如：`validate:"not_empty,endpoint"`）。

> **关于循环结构的说明：** 通过代码构建的循环指针结构（例如引用自身的节点）不会导致 `Validate` 挂起 — 沿当前路径的递归会针对引用字段返回明确的 `cyclic structure detected` 错误。被多个字段引用的共享（无环）子对象仍然有效。在 `Help` 输出中，自引用类型每个路径只记录一次，对其的递归会被静默跳过。

> **关于边界限制的说明：** 无法解析的限制值（例如 `validate:"minlen=abc,maxlen=5"`）会作为明确的配置错误报告，并指出规则名称（`invalid validator configuration: minlen "abc": ...`）— 无效的限制绝不会静默禁用其检查。同样适用于置于非数值类型（例如 `bool`）上的边界规则（`min`、`max`、`gt`、`lt`）：检查无法应用，因此会报告为配置错误，而不是静默跳过。



| 特性与规则标识       | 语法示例                               | 说明                                                                           | 支持的类型                                    |
|:--------------|:-----------------------------------|:-----------------------------------------------------------------------------|:-----------------------------------------|
| **环境变量注入**    | `env:"APP_PORT"`                   | **12-Factor App:** 动态注入和读取 OS 变量。默认情况下环境变量的优先级绝对高于 YAML 值和 `default` 标记；来源仲裁可通过 `priority` 标签实现（参见**来源优先级**）。`default` 与 `env` 标签适用于标量字段。支持 `time.Duration` 时间值。在加载路径（`LoadFile`、`Load`、`Loader.Load`、`Decoder.Decode`）中，无效的环境变量值会报告为错误；独立的 `SetDefaults` 调用保持旧版的静默行为——无效的环境变量值会让字段保持零值且不报错，`default` 标签不会被应用（唯一的例外：无效的 `time.Duration` 字面量会报告为错误）。               | `string`, `bool`, 基础数值, `time.Duration` |
| **来源优先级**    | `priority:"value"`                 | **env 仲裁标签：** 适用于同时带有 `env` 标签的字段；取值为 `value` 或 `env`。默认情况下（无标签或 `priority:"env"`）环境变量的优先级绝对高于 YAML 值和 `default` 标签。设置 `priority:"value"` 后，在应用 default 和 decode 之后非零的字段——decode 得到的 YAML 值、已应用的非零 `default` 标签或预先填充的独立值——优先于非空环境变量；若只存在一个来源，则无论标签如何都会应用该来源。在应用 default 和 decode 之后仍为零的字段（YAML 中的显式零值或零值 `default` 标签，如 `default:"0"`、`default:"false"`、`default:""`）不算作仲裁依据的值 — 应用环境变量。未知的标签取值以及没有 `env` 的 `priority` 标签会报告为配置错误，与值或环境变量是否存在无关。与 `default` 和 `env` 一样，`priority` 标签适用于标量字段；置于非标量字段（struct、slice、map）上时会报告为配置错误。 | `string`, `bool`, 基础数值, `time.Duration` |
| **必填字段**      | `validate:"required"`             | 确保字段被分配了一个非零值。与 `default` 标签互斥。`not_empty` 是其旧版别名（为向后兼容而保留，计划弃用）。                | `string`, `struct`, `slice`, `map`, 数字类型 |
| **条件关联必填**    | `validate:"required_if=role!=srv"` | **跨字段联合校验：** 依赖另一个字段的值决定当前字段是否必填。规范运算符：`==`（相等）、`!=`（不相等）——比较不区分大小写；`~=`（正则匹配）与 `^=`（正则不匹配）；规范运算符的操作数可以加引号（`'...'` / `"..."`）——引号会被去除。简单名称先在所属结构体内解析，再回退到根结构；点路径（`required_if=server.mode==prod`）从根解析；指针目标会自动解引用。目标不存在、目标未导出或正则非法时报告配置错误。`required_if` 值中缺少任何可识别的运算符时，报告配置错误。旧版冒号语法（`key:value`、`key:!value`、宏 `:empty`/`:not_empty`）已弃用（deprecated），但仍受支持；旧版原子与规范原子可以在同一个表达式中混用。支持多条件：使用 `&&`（AND）和 `\|\|`（OR）组合条件，并用括号分组 —— `&&` 的优先级高于 `\|\|`。仅当 `&&` 或 `\|\|` 出现在引号之外，或值以未加引号的 `(` 开头时，条件才按表达式解析 —— 仅用括号即可包住单个条件；否则单条件的语法与消息保持不变。`&&`、`\|\|` 和括号周围的空格会被忽略；原子内部比较运算符（`==`、`!=`、`~=`、`^=`）周围的空格不可用 —— 带空格的目标名称无法解析，并报告 `target field "..." not found`。包含 `&&` 或 `\|\|` 的操作数必须加引号：`webhook.url~='(a\|b)'`；带引号的片段按字面量处理。操作数内的括号（在比较运算符之后，例如 `tag~=(a\|b)`）无需引号。在同一个字段上声明两次 `required_if` 属于配置错误 —— 请用 `&&` 或 `\|\|` 将条件合并为一个表达式。包含 `&&` 或 `\|\|` 的已废弃 legacy 值会按表达式解析（例如 `key:a&&b` 变为原子 `key:a` 与一个错误原子的 AND），而不再是单个字面比较。同一字段上的定量规则（`min`、`max`、`minlen`、`maxlen`、`mincount`、`maxcount`）无论表达式结果如何都会检查 —— 即使表达式为 false，空字段上的 `minlen` 也会报告。表达式求值是完整的：形状错误（括号不配对、空原子）和原子的配置错误无论字段值与最终结果如何都会报告，出错的原子的结果为 false；重复的相同原子错误只报告一次。违反表达式时报告为 `field <名称>: is required when <表达式>`（例如 `field key_file: is required when mode==webhook&&(url~=^https:\|\|cert^=^$)`）。 | `string`, 数字, `bool`, 指针类型等              |
| **枚举白名单**     | `validate:"choice=dev,prod"`       | **白名单模式：** 字符串值必须与逗号分隔的标识符完全匹配。递归支持切片元素。                                     | `string`, `[]string`                     |
| **枚举黑名单**     | `validate:"choice=!red,!black"`    | **黑名单模式：** 允许任何字符串值，开头的 `!` 标识符除外。递归支持切片元素。                                  | `string`, `[]string`                     |
| **数值与时间范围**   | `validate:"min=1s,max=10m"`        | 强制执行**包含边界**的上下限限制。原生支持基础数字以及 `time.Duration` 时间间隔。                          | `int`, `uint`, `float`, `time.Duration`  |
| **严格数值比较**    | `validate:"gt=5,lt=10"`            | 强制执行**不包含边界**的严格大于（`>`）和严格小于（`<`）限制。同样支持 `time.Duration`。                    | `int`, `uint`, `float`, `time.Duration`  |
| **字符串长度(字符)** | `validate:"minlen=3,maxlen=20"`    | 强制限制字符串的长度边界。升级为计算 **Unicode 字符数 (Runes)**，而非原生字节数，完美支持多语言。                  | `string`                                 |
| **容器容量限制**    | `validate:"mincount=1,maxcount=5"` | 强制限制动态切片（Slice）或映射（Map）中允许的最小和最大元素数量。                                        | `slice`, `map`                           |
| **网络地址格式**    | `validate:"format=ipv4"`           | 校验特定的 IP 网络布局。支持 `format=ip`, `format=ipv4` 和 `format=ipv6`。与 `endpoint` 和 `url` 互斥。 | `string`                                 |
| **十六进制编码**     | `validate:"format=hex"`            | 校验十六进制字符串：可通过 `hex.DecodeString` 解码且字符数为偶数。奇数长度与非十六进制字符会被拒绝。             | `string`                                 |
| **UUID 唯一标识** | `validate:"format=uuid"`           | 校验 UUID 的规范形式（十六进制 8-4-4-4-12 段，带连字符）；版本与变体（RFC 4122）不做校验。             | `string`                                 |
| **正则表达式**     | `validate:"regexp=^[a-z]{2,4}$"`   | 验证字符串布局是否符合正则表达式。防止由于包含逗号而发生解析断裂。外层引号（`'...'` 或 `"..."`）会从模式中剥离；如需匹配字面引号，请使用相反类型的引号定界（例如 `regexp="^'q'$"`）。 | `string`, `[]string`                     |
| **网络终结点**     | `validate:"endpoint"`              | 强制执行标准网络终结点 (`host:port`)。原生检查 **IPv6** 语法和端口边界（1-65535）。                    | `string`                                 |
| **严格的 URL**   | `validate:"url"`                   | 校验统一资源定位符。严格要求显式的协议方案分隔符（如 `http://`, `grpc://`）以及非空的主机名：仅含方案（`http://`）的值会被拒绝。                            | `string`                                 |
| **实例级路径规则** | `validate:"min_len.min=0,max_len.max=64"` | **共享嵌套类型：** 通过 `<yaml路径>.<规则>=<值>` 语法将规则应用于该实例的嵌套字段；每个父实例拥有自己的边界。悬空路径会报告为错误。路径规则的值可以包含点号。 | 嵌套 struct 字段 |

---

## 加载与导出

- **选项端到端生效：** 传给 `Load` / `LoadFile` 的每个选项（例如 `WithKnownFields`、`WithUniqueKeys`）既作用于 AST 解析阶段，也作用于最终解码到目标结构的阶段——严格检查与上游 `yaml4.Load` 完全一致。仅影响格式的选项（如 `WithIndent`）不影响加载。
- **`!include` 回写与对象绑定：** `DumpWithInclude` 从加载该对象时记录的 include 数据中获取回写路径——请将加载时传入 `Load` / `LoadFile` 的同一个指针传给 `DumpWithInclude`。未经过此类加载的结构体（或已加载值的副本）将以安全模式导出：值内联输出，不生成 `!include` 标记，也不修改任何文件。
- **`!include` 在所有加载路径中解析：** `LoadFile` 相对于配置文件所在目录解析 `!include` 指令，而 `Load`、`Loader.Load` 与 `Decoder.Decode` 相对于当前工作目录解析。流式路径不会将 include 会话绑定到目标对象，并且对先前通过 `Load`/`LoadFile` 加载的指针执行成功的流式加载会解除先前的绑定：该对象将以 inline safe mode 导出（参见上文的回写条目）。
- **每个 include 目标文件仅限一个文档：** 作为 `!include` 目标的文件必须恰好包含一个 YAML 文档——结尾的 `---` 文档标记会导致加载失败（`included file ... contains multiple documents`）。主配置文件遵循上游 yaml4 语义：单次执行路径（`LoadFile`、`Load`、`Unmarshal`）同样会拒绝结尾的 `---` 标记（`expected single document, found multiple`），而流式路径（`Loader.Load`、`Decoder.Decode`）只读取第一个文档并静默丢弃其余文档。

---

## 配置配置文件示例（有效与无效场景）

### 示例 1：集成自动化命令行帮助信息输出的标准配置（有效场景）
**输入 YAML 规范数据 (`config.yaml`)：**
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

**Go 语言应用程序配置模型结构：**
```go
package main

import (
	"fmt"
	"time"

	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"支持的最小 TLS 协议版本"`
	Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"应用层协议协商 ALPN 列表"`
}

type Logging struct {
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn" description:"服务端通用日志记录器级别"`
	Colors        bool             `yaml:"colors" description:"是否开启高亮日志终端颜色显示"`
	Timeout       string           `yaml:"timeout" default:"5s" description:"内部流水线请求级超时保护时间"`
}

type Config struct {
	Env          string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"程序运行时核心环境状态配置"`
	AppName      string        `yaml:"app_name" default:"api" validate:"minlen=3,maxlen=10" description:"程序内部标识别名"`
	APIUrl       string        `yaml:"api_url" validate:"url,required_if=env==prod" env:"API_URL" description:"目标 API 全局基础网关分发链路地址"`
	APIEndpoint  string        `yaml:"api_endpoint" validate:"endpoint" description:"本地通信网络终结点绑定接口"`
	Timeout      time.Duration `yaml:"timeout" default:"5s" validate:"min=1s,max=10m" description:"全局异步 IO 核心超时生命周期追踪参数"`
	Crypto       TLS           `yaml:"crypto" description:"TLS 加密通信安全架构配置节点"`
	Server       struct {
		Logging    Logging  `yaml:"logging" description:"服务端通用日志记录器参数"`
		AllowedIPs []string `yaml:"allowed_ips" validate:"mincount=1,maxcount=10" description:"信任的远程连接白名单"`
	} `yaml:"server"`
}

func main() {
	var cfg Config
	yaml.Help(cfg)
}
```

**自动生成的交互式命令行 CLI 帮助输出详情 (`yaml.Help(cfg)`):**

自引用的结构体类型每个路径只记录一次 — 递归分支会被静默跳过，因此 `Help` 总是能够终止。
```text
yaml configuration schema documentation:

env:             程序运行时核心环境状态配置 (default: dev, validate: [choice=dev,stage,prod])
app_name:        程序内部标识别名 (default: api, validate: [minlen=3,maxlen=10])
api_url:         目标 API 全局基础网关分发链路地址 (env: API_URL, validate: [url,required_if=env==prod])
api_endpoint:    本地通信网络终结点绑定接口 (validate: [endpoint])
timeout:         全局异步 IO 核心超时生命周期追踪参数 (default: 5s, validate: [min=1s,max=10m])
crypto:          TLS 加密通信安全架构配置节点
  min_version:   支持的最小 TLS 协议版本 (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])
  alpn:          应用层协议协商 ALPN 列表 (default: h2,http/1.1, validate: [choice=h2,http/1.1])
server:
  logging:       服务端通用日志记录器参数
    level:       服务端通用日志记录器级别 (default: info, validate: [choice=debug,info,warn])
    colors:      是否开启高亮日志终端颜色显示
    timeout:     内部流水线请求级超时保护时间 (default: 5s)
  allowed_ips:   信任的远程连接白名单 (validate: [mincount=1,maxcount=10])
```

---

### 示例 2：枚举白名单校验条件冲突（无效场景）
**输入 YAML 规范数据 (`config.yaml`)：**
```yaml
env: "testing" # 错误：“testing”不在预设的白名单允许范围限制内 [dev, stage, prod]
```
**系统抛出的运行时错误字符串文本 (`err.Error()`):**
```text
field env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### 示例 3：多错误拦截收集、字符串长度、集合容量与时间约束规则触发拦截（无效场景）
**输入 YAML 规范数据 (`config.yaml`)：**
```yaml
env: "prod"
app_name: "go"         # 错误 1：字符长度为 2，违反了 minlen=3 的限制
api_url: ""            # 错误 2：条件触发必填 (env==prod) 却未传值
timeout: "500ms"       # 错误 3：违反了严格的 time.Duration 最小值限制 min=1s
server:
  logging:
    level: "debug"     # (有效)
  allowed_ips: []      # 错误 4：元素数量为 0，违反了 mincount=1 的限制
```
**系统抛出的运行时多错误累积聚合字符串文本 (`err.Error()`):**
```text
field app_name: string length 2 is less than minlen 3
field api_url: is required when field env==prod
field timeout: value 500ms < min 1s
field server.allowed_ips: collection size 0 is less than mincount 1
```

---

## 示例

`examples/` 目录包含覆盖所有验证规则和功能特性的可运行程序。请在仓库根目录下运行：

```bash
go run ./examples/01_choice
```

| 示例 | 演示内容 |
|------|----------|
| `01_choice` | `choice` 白名单/黑名单模式，含带逗号的引号值，有效与无效配置 |
| `02_help` | 通过 `yaml.Help()` 从 `description` 标签自动生成 CLI 帮助文档 |
| `03_min-max` | 数字与 `time.Duration` 的包含边界 `min`/`max`，有效与越界配置 |
| `04_mincount-maxcount` | 切片与映射的集合元素数量限制，有效与超限配置 |
| `05_required_if` | 跨字段 `required_if` 使用 `==`/`!=` 运算符、`~=`/`^=` 正则匹配器，结合 `endpoint` 与 `url` 规则 |
| `06_with-include` | 加载时解析 `!include` 指令，含缺失文件错误场景 |
| `07_with_env_overwrite` | 环境变量注入覆盖标签默认值，含白名单违规场景 |
| `08_write-to-includes` | 保留 `!include` 结构的原子化回写（`DumpWithInclude`） |
| `10_test` | 包含路径跟踪器（`FindIncludeFile`）及 `DumpWithInclude` 对标量输入的拒绝 |
| `11_not_empty-required` | `required` 规则及其旧版别名 `not_empty`（为向后兼容而保留） |
| `12_gt-lt` | int、float 与 `time.Duration` 的严格边界 `gt`/`lt` |
| `13_minlen-maxlen` | 以 Unicode 字符数计算的字符串长度限制 |
| `14_format` | `format=ip`、`format=ipv4`、`format=ipv6`、`format=uuid`、`format=hex` |
| `15_regexp` | `regexp` 规则：普通模式、内嵌逗号、切片元素 |
| `16_validation-errors` | 跨多种规则的多个违规的聚合报告 |
| `17_embed` | 共享嵌套类型上的实例级路径规则（`min_len.min=0` 与 `min_len.min=4`） |
| `18_required_if-operators` | `required_if` 规范运算符：`==`、`!=`、`~=` 与 `^=` |
| `19_priority` | 通过 `priority` 标签在 YAML/default 值与环境变量之间进行来源仲裁：双来源、单来源、无标签的默认行为及零值 `default` 边界 |
| `20_required_if-logic` | `required_if` 中的布尔表达式：`&&`、`\|\|`、括号与优先级（`&&` 优先于 `\|\|`） |
