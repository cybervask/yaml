# KNOWN_PROBLEMS.md — problems inherited from the upstream library

> Scope: behaviors inherited byte-for-byte from `go.yaml.in/yaml/v4` v4.0.0.
> This library is a drop-in replacement: it does not patch the upstream
> parser, so upstream quirks are documented here instead of being "fixed".

## KP-1 · Unsupported yaml struct flags panic on decode, error on encode

- **Symptom**: a struct tag carrying a flag the upstream decoder does not
  know — e.g. `yaml:"a,include"` — makes every decode path panic:
  `panic: unsupported flag "include" in tag "a,include" of type <T>`, while
  `Marshal` of the same struct returns a regular error with the same wording.
  Reproduced: `yaml4.Unmarshal([]byte("a: x\n"), &v)` panics; our `Unmarshal`
  panics identically (byte-for-byte drop-in parity); `yaml4.Marshal` returns
  `yaml: unsupported flag "include" in tag "a,include" of type <T>`.
- **Root cause (upstream)**: `go.yaml.in/yaml/v4` v4.0.0 rejects unknown
  struct-tag flags with a panic in its decode paths and an error in its
  encode path — the asymmetry is upstream's.
- **Status**: not patched in this library; patching it would break drop-in
  compatibility, so the behavior is documented instead.
- **Workaround**: do not use flags unknown to the upstream decoder; pin
  struct tags in tests (go vet does not catch unknown flags).

## KP-2 · Multi-document input: one-shot paths reject, streaming paths truncate

- **Symptom**: input with more than one YAML document (a `---` separator,
  whether or not the second document is non-empty) is rejected by the
  one-shot paths (`Unmarshal`, `Load`, `LoadFile`) with
  `failed to parse YAML to AST: yaml: construct errors: <unknown position>:
  yaml: expected single document, found multiple`, while the streaming paths
  (`Loader.Load`, `Decoder.Decode`) return the first document silently
  (`err=nil`, remainder discarded). Verified against all five paths with
  both `a: 1\n---\nb: 2\n` and `a: 1\n---\n` on 2026-11-15.
- **Root cause (upstream)**: the upstream Node-based one-shot loader errors
  on extra documents; the upstream streaming loader reads the first document
  and stops — the asymmetry lives in the upstream parser/loader code, and
  this library merely routes both behaviors through its own API (including
  the error wrapping).
- **Status**: not patched for the top-level document (drop-in parity).
  Include targets are guarded by this library on every path: an include
  target holding more than one document fails with
  `included file ... contains multiple documents` regardless of the load
  path.
- **Workaround**: keep exactly one document per file; when streaming, split
  the documents yourself and call `Decoder.Decode` once per document.

## Close, but ours — not upstream

- `Decoder.Decode` now resolves `!include` directives the same way the load
  paths do: each `Decode` call resets the include tracker, parses the next
  document into a node, and resolves `!include` recursively against the
  current working directory before defaults are applied and the node is
  decoded into the target. The former documented limitation is lifted. No
  include session is bound to the target: a later `DumpWithInclude` of a
  stream-decoded object runs in the inline safe mode. A session previously
  bound to the same pointer by `Load` or `LoadFile` is dropped by a
  successful stream load (`Decoder.Decode` or `Loader.Load`) — the stream
  replaced the object's content, so the earlier binding no longer reflects
  it; a failed stream load keeps the session.
- `HelpStr` no longer emits trailing whitespace: every rendered line is
  trimmed with `strings.TrimRight` before the newline — our rendering,
  deliberately unlike the padded upstream-style output; column alignment
  is preserved, and the path column width is measured in terminal display
  columns (East Asian wide runes reserve two); the documentation follows:
  the README.md example is updated to the new rendering, while the
  localized ru/zh examples keep their translated descriptions and are
  trimmed of trailing whitespace on the same lines.
