package yaml

import (
	"fmt"
	"io"
	"os"

	yaml4 "go.yaml.in/yaml/v4"
)

// Decoder wraps an underlying YAML stream decoder to provide additional
// initialization and validation layers during decoding.
type Decoder struct {
	dec         *yaml4.Decoder
	knownFields bool
}

// NewDecoder returns a new [Decoder] that reads from the provided [io.Reader].
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{dec: yaml4.NewDecoder(r)}
}

// KnownFields configures the decoder behavior regarding unknown keys.
// If enabled, the decoding process will return an error if a key in the YAML
// mappings cannot be matched to an exported field in the target structure.
func (dec *Decoder) KnownFields(enable bool) {
	dec.knownFields = enable
	dec.dec.KnownFields(enable)
}

// Decode reads the next YAML-encoded value from its input stream and stores
// the result in the value pointed to by v.
//
// The document is first decoded into an intermediate AST node; !include
// directives are then resolved recursively against the current working
// directory (the include tracker is reset per Decode call, so after a
// multi-document stream the global tracker holds only the last document's
// paths; per-object sessions recorded by [Load] and [LoadFile] are not
// affected). Only after the
// document has been parsed and its !include directives resolved successfully
// are default tag values applied, the resolved node decoded into v,
// non-empty environment variable overrides re-applied over the decoded
// content, and the final structural state validated. A stream parse error —
// including io.EOF on an exhausted stream — or a failed include resolution
// returns as-is and leaves the target untouched.
//
// Unlike [Load] and [LoadFile], no include session is bound to v: a later
// DumpWithInclude of a stream-decoded object runs in the inline safe mode.
// A session previously bound to the same pointer by [Load] or [LoadFile] is
// dropped by a successful Decode — the stream replaced the object's content,
// so the earlier binding no longer reflects it (a failed Decode keeps it).
//
// The multi-document stream semantics of the underlying decoder are
// preserved: each Decode call reads the next document. Unlike
// [Loader.Load], the parse error — including io.EOF on an exhausted stream —
// is returned unwrapped, matching the upstream [Decoder].
func (dec *Decoder) Decode(v any) error {
	ResetIncludeTracker()

	var node yaml4.Node
	if err := dec.dec.Decode(&node); err != nil {
		return err
	}

	// Relative include paths resolve against the real working directory so
	// that the tracker records absolute paths for every load entry point,
	// as documented by FindIncludeFileAbs.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	if err := resolveIncludes(&node, cwd, "", newIncludeChain(), nil); err != nil {
		return err
	}

	// Defaults are applied only after the document has been read and its
	// !include directives resolved successfully: a stream parse error or a
	// failed include resolution must leave the target untouched, matching
	// the contract of Load and LoadFile.
	if err := SetDefaults(v); err != nil {
		return fmt.Errorf("failed to set default values: %w", err)
	}

	var opts []Option
	if dec.knownFields {
		opts = append(opts, WithKnownFields())
	}

	if err := node.Load(v, opts...); err != nil {
		return fmt.Errorf("failed to decode YAML AST into target structure: %w", err)
	}

	if err := applyEnvOverrides(v); err != nil {
		return fmt.Errorf("failed to apply environment variable overrides: %w", err)
	}

	if err := Validate(v); err != nil {
		return err
	}

	// The stream replaced the object's content without binding a new session:
	// a session an earlier [Load]/[LoadFile] recorded for this pointer no
	// longer reflects the object, so it must not drive a later write-back.
	dropIncludeSession(v)

	return nil
}
