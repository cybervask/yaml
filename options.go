package yaml

import (
	yaml4 "go.yaml.in/yaml/v4"
)

// Re-exported version configuration presets.
var (
	V2 = yaml4.V2
	V3 = yaml4.V3
	V4 = yaml4.V4
)

// Option defines a functional configuration capability that can customize
// and control YAML loading and dumping pipeline operations.
type Option = yaml4.Option

// Re-exported functional configuration option parameters.
var (
	// WithIndent configures the preferred indentation spacing layout (bounds ranging from 2 to 9).
	WithIndent = yaml4.WithIndent

	// WithCompactSeqIndent configures the encoder to treat the sequence indicator ('- ')
	// as part of the indentation alignment block.
	WithCompactSeqIndent = yaml4.WithCompactSeqIndent

	// WithKnownFields enforces strict matching between incoming YAML keys and
	// exported structure fields during decoding.
	WithKnownFields = yaml4.WithKnownFields

	// WithSingleDocument ensures that only the initial document inside an incoming stream is processed.
	WithSingleDocument = yaml4.WithSingleDocument

	// WithStreamNodes enables stream boundary node tracking parameters when decoding documents.
	WithStreamNodes = yaml4.WithStreamNodes

	// WithAllDocuments enables multi-document processing semantics for both [Load] and [Dump] operations.
	WithAllDocuments = yaml4.WithAllDocuments

	// WithLineWidth defines the preferred maximum column width threshold for wrapped text layout formatting.
	WithLineWidth = yaml4.WithLineWidth

	// WithUnicode allows or suppresses escaping routines over non-ASCII characters inside output streams.
	WithUnicode = yaml4.WithUnicode

	// WithUniqueKeys activates strict checks to prevent duplicate keys within mapping elements.
	WithUniqueKeys = yaml4.WithUniqueKeys

	// WithCanonical instructs the engine to enforce canonical YAML representation layouts on output streams.
	WithCanonical = yaml4.WithCanonical

	// WithLineBreak configures the specific platform-dependent line termination layout format.
	WithLineBreak = yaml4.WithLineBreak

	// WithExplicitStart forces document boundary start markers (---) onto emitted documents.
	WithExplicitStart = yaml4.WithExplicitStart

	// WithExplicitEnd forces document boundary end markers (...) onto emitted documents.
	WithExplicitEnd = yaml4.WithExplicitEnd

	// WithFlowSimpleCollections applies block or flow line styles selectively over basic collection types.
	WithFlowSimpleCollections = yaml4.WithFlowSimpleCollections

	// WithQuotePreference configures preferred character escapes and quoting options when text quoting is required.
	WithQuotePreference = yaml4.WithQuotePreference
)

var (
	// OptsYAML evaluates a raw configuration properties string block to parse and extract operational configuration options.
	OptsYAML = yaml4.OptsYAML

	// Options aggregates multiple individual configuration parameters into a unified compound [Option].
	Options = yaml4.Options
)

type (
	// Node models an abstract structural syntax element inside the overall YAML document parsing hierarchy tree.
	Node = yaml4.Node

	// Kind identifies the structural type semantics associated with a specific document [Node].
	Kind = yaml4.Kind

	// Style controls visual syntax representation and string formatting configurations on a output document [Node].
	Style = yaml4.Style

	// Marshaler defines the interface protocol implemented by data types that require customized YAML serialization behavior.
	Marshaler = yaml4.Marshaler

	// IsZeroer defines the interface protocol implemented by types capable of reporting whether their inner state reflects a zero value.
	IsZeroer = yaml4.IsZeroer
)

// Unmarshaler defines the interface protocol implemented by data types that require customized YAML deserialization capabilities.
type Unmarshaler yaml4.Unmarshaler

type (
	// VersionDirective describes explicit version specifications contained inside structural document directives blocks.
	VersionDirective yaml4.VersionDirective

	// TagDirective maps shorthand macro configurations used within structural tag declarations inside document blocks.
	TagDirective yaml4.TagDirective

	// Encoding represents text streaming character layout types.
	Encoding yaml4.Encoding
)

// Re-exported character data streaming stream encodings configuration constants.
const (
	EncodingAny     = yaml4.EncodingAny
	EncodingUTF8    = yaml4.EncodingUTF8
	EncodingUTF16LE = yaml4.EncodingUTF16LE
	EncodingUTF16BE = yaml4.EncodingUTF16BE
)

type (
	// LoadError represents an execution failure encountered during document unpacking operations,
	// containing file coordinates and detailed diagnostics about the incident location.
	LoadError yaml4.LoadError

	// LoadErrors represents a composite validation type capturing multiple field processing failures
	// collected during complex multi-property decoding tasks.
	LoadErrors yaml4.LoadErrors

	// TypeError represents a legacy semantic decoding tracking model preserved exclusively for backwards compatibility.
	//
	// Deprecated: Migrate to the modernized [LoadErrors] API model.
	//
	//nolint:staticcheck // Reason: preserving legacy struct for backward compatibility layouts API contracts.
	TypeError yaml4.TypeError
)

// Re-exported node identification category tracking constant values.
const (
	DocumentNode = yaml4.DocumentNode
	SequenceNode = yaml4.SequenceNode
	MappingNode  = yaml4.MappingNode
	ScalarNode   = yaml4.ScalarNode
	AliasNode    = yaml4.AliasNode
	StreamNode   = yaml4.StreamNode
)

// Re-exported document element stylistic structural visibility presentation constants.
const (
	TaggedStyle       = yaml4.TaggedStyle
	DoubleQuotedStyle = yaml4.DoubleQuotedStyle
	SingleQuotedStyle = yaml4.SingleQuotedStyle
	LiteralStyle      = yaml4.LiteralStyle
	FoldedStyle       = yaml4.FoldedStyle
	FlowStyle         = yaml4.FlowStyle
)

// LineBreak represents line feed configuration layout markers applied over output files.
type LineBreak = yaml4.LineBreak

// Re-exported environment line ending platform termination literal constants.
const (
	LineBreakLN   = yaml4.LineBreakLN   // Unix-style line feeding markers (\n)
	LineBreakCR   = yaml4.LineBreakCR   // Classic legacy Macintosh line feed carriage markers (\r)
	LineBreakCRLN = yaml4.LineBreakCRLN // Windows-style paired carriage line feeding parameters (\r\n)
)

// QuoteStyle defines specific text wrapping quotes applied over raw data elements.
type QuoteStyle = yaml4.QuoteStyle

// Re-exported string quotation styling presentation constants.
const (
	QuoteSingle = yaml4.QuoteSingle // Prefer single quote escaping semantics (modern v4 baseline default)
	QuoteDouble = yaml4.QuoteDouble // Prefer double quote escaping configurations
	QuoteLegacy = yaml4.QuoteLegacy // Fallback to classic legacy v2/v3 escaping layouts
)
