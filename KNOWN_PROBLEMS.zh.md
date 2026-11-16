# KNOWN_PROBLEMS.zh.md — 继承自上游库的已知问题

> 范围：从 `go.yaml.in/yaml/v4` v4.0.0 逐字节继承的行为。本库是 drop-in
> replacement（直接替换实现）：不修补上游解析器，因此上游的怪癖在此记录，
> 而非“修复”。

## KP-1 · 未知的 yaml 结构体标签标志：解码时 panic，编码时报错

- **症状**：结构体标签携带上游解码器不认识的标志——例如
  `yaml:"a,include"`——会使所有解码路径 panic：
  `panic: unsupported flag "include" in tag "a,include" of type <T>`，而同一
  结构体的 `Marshal` 返回一条措辞相同的普通错误。实证复现：
  `yaml4.Unmarshal([]byte("a: x\n"), &v)` panic；本库的 `Unmarshal` 以完全
  相同的方式 panic（逐字节 drop-in 一致性）；`yaml4.Marshal` 返回
  `yaml: unsupported flag "include" in tag "a,include" of type <T>`。
- **根因（上游）**：`go.yaml.in/yaml/v4` v4.0.0 在解码路径中以 panic 拒绝
  未知的结构体标签标志，在编码路径中以错误拒绝——该不对称性属于上游。
- **状态**：本库不修补：修补会破坏 drop-in 兼容性，故仅作文档记录。
- **规避**：不要使用上游解码器不认识的标志；用测试固定结构体标签
  （go vet 不会捕获未知标志）。

## KP-2 · 多文档输入：一次性路径拒绝，流式路径截断

- **症状**：包含多个 YAML 文档的输入（`---` 分隔符，无论第二个文档是否
  非空）会被一次性路径（`Unmarshal`、`Load`、`LoadFile`）拒绝，错误为
  `failed to parse YAML to AST: yaml: construct errors: <unknown position>:
  yaml: expected single document, found multiple`；而流式路径
  （`Loader.Load`、`Decoder.Decode`）静默返回第一个文档（`err=nil`，其余
  被丢弃）。已于 2026-11-15 对全部五条路径、以
  `a: 1\n---\nb: 2\n` 与 `a: 1\n---\n` 两种输入验证。
- **根因（上游）**：上游基于 Node 的一次性加载器对多余文档报错；上游
  流式加载器读取第一个文档后停止——该不对称性存在于上游解析器/加载器
  代码中，本库只是通过自己的 API 转发这两种行为（包括错误包装）。
- **状态**：对顶层文档不修补（drop-in 一致性）。`!include` 的目标文件在
  本库的每条路径上都有保护：包含多于一个文档的 include 目标会报错
  `included file ... contains multiple documents`，与加载路径无关。
- **规避**：每个文件严格保留一个文档；流式读取时自行拆分文档，对每个
  文档调用一次 `Decoder.Decode`。

## 相近但属于本库——并非上游

- `Decoder.Decode` 现在以与其余加载路径相同的方式解析 `!include` 指令：每次
  调用 `Decode` 都会重置 include 跟踪器，把下一个文档解析为节点，并在应用
  默认值、把节点解码到目标之前，相对于当前工作目录递归解析 `!include`。
  此前记录的限制已解除。目标对象不绑定 include 会话：对通过流式解码加载的
  对象随后调用 `DumpWithInclude` 时，将以 inline safe mode 运行。先前通过
  `Load` 或 `LoadFile` 绑定到同一指针的 include 会话会被成功的流式加载
  （`Decoder.Decode` 或 `Loader.Load`）移除——流替换了对象的内容，先前的
  绑定不再反映该对象；失败的流式加载会保留会话。
- `HelpStr` 不再输出尾随空格：每一行渲染结果在换行前都会通过
  `strings.TrimRight` 截去尾部空格——这是本库自己的渲染，刻意不同于上游的
  填充式输出；列对齐保持不变，路径列宽度按终端显示列宽计算（East Asian
  宽字符占两列），文档随之更新：README.md 示例已更新为新渲染，本地化的
  ru/zh 示例保留其翻译描述，并在相同行上截去尾随空格。
