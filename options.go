package yaml

import (
	yaml4 "go.yaml.in/yaml/v4"
)

// Version presets
var (
	V2 = yaml4.V2
	V3 = yaml4.V3
	V4 = yaml4.V4
)

// Option allows configuring YAML loading and dumping operations.
type Option = yaml4.Option

// With-options
var (
	// WithIndent sets indentation spaces (2-9).
	WithIndent = yaml4.WithIndent

	// WithCompactSeqIndent configures '- ' as part of indentation.
	WithCompactSeqIndent = yaml4.WithCompactSeqIndent

	// WithKnownFields enables strict field checking during loading.
	WithKnownFields = yaml4.WithKnownFields

	// WithSingleDocument only processes first document in stream.
	WithSingleDocument = yaml4.WithSingleDocument

	// WithStreamNodes enables stream boundary nodes when loading.
	WithStreamNodes = yaml4.WithStreamNodes

	// WithAllDocuments enables multi-document mode for Load and Dump.
	WithAllDocuments = yaml4.WithAllDocuments

	// WithLineWidth sets preferred line width for output.
	WithLineWidth = yaml4.WithLineWidth

	// WithUnicode controls non-ASCII characters in output.
	WithUnicode = yaml4.WithUnicode

	// WithUniqueKeys enables duplicate key detection.
	WithUniqueKeys = yaml4.WithUniqueKeys

	// WithCanonical forces canonical YAML output format.
	WithCanonical = yaml4.WithCanonical

	// WithLineBreak sets line ending style for output.
	WithLineBreak = yaml4.WithLineBreak

	// WithExplicitStart controls document start markers (---).
	WithExplicitStart = yaml4.WithExplicitStart

	// WithExplicitEnd controls document end markers (...).
	WithExplicitEnd = yaml4.WithExplicitEnd

	// WithFlowSimpleCollections controls flow style for simple collections.
	WithFlowSimpleCollections = yaml4.WithFlowSimpleCollections

	// WithQuotePreference sets preferred quote style when quoting is required.
	WithQuotePreference = yaml4.WithQuotePreference
)

var (
	// OptsYAML evaluates a raw properties config segment string, extracting structural Option configurations.
	OptsYAML = yaml4.OptsYAML

	// Options folds multiple distinct configuration parameter settings down into one unified single composite Option.
	Options = yaml4.Options
)

// Type and constant re-exports

type (
	// Node represents a YAML node in the document tree.
	Node = yaml4.Node

	// Kind identifies the type of a YAML node.
	Kind = yaml4.Kind

	// Style controls the presentation of a YAML node.
	Style = yaml4.Style

	// Marshaler is implemented by types with custom YAML marshaling.
	Marshaler = yaml4.Marshaler

	// IsZeroer is implemented by types that can report if they're zero.
	IsZeroer = yaml4.IsZeroer
)

type Unmarshaler yaml4.Unmarshaler

// Re-export stream-related types

type (
	VersionDirective yaml4.VersionDirective
	TagDirective     yaml4.TagDirective
	Encoding         yaml4.Encoding
)

// Re-export encoding constants

const (
	EncodingAny     = yaml4.EncodingAny
	EncodingUTF8    = yaml4.EncodingUTF8
	EncodingUTF16LE = yaml4.EncodingUTF16LE
	EncodingUTF16BE = yaml4.EncodingUTF16BE
)

// Re-export error types

type (
	// LoadError represents an error encountered while decoding a YAML document.
	//
	// It contains details about the location in the document where the error
	// occurred, as well as a descriptive message.
	LoadError yaml4.LoadError

	// LoadErrors is returned when one or more fields cannot be properly decoded.
	//
	// It contains multiple *[LoadError] instances with details about each error.
	LoadErrors yaml4.LoadErrors

	// TypeError is an obsolete error type retained for compatibility.
	//
	// Deprecated: Use [LoadErrors] instead.
	//
	//nolint:staticcheck // we are using deprecated TypeError for compatibility
	TypeError yaml4.TypeError
)

// Re-export Kind constants
const (
	DocumentNode = yaml4.DocumentNode
	SequenceNode = yaml4.SequenceNode
	MappingNode  = yaml4.MappingNode
	ScalarNode   = yaml4.ScalarNode
	AliasNode    = yaml4.AliasNode
	StreamNode   = yaml4.StreamNode
)

// Re-export Style constants
const (
	TaggedStyle       = yaml4.TaggedStyle
	DoubleQuotedStyle = yaml4.DoubleQuotedStyle
	SingleQuotedStyle = yaml4.SingleQuotedStyle
	LiteralStyle      = yaml4.LiteralStyle
	FoldedStyle       = yaml4.FoldedStyle
	FlowStyle         = yaml4.FlowStyle
)

// LineBreak represents the line ending style for YAML output.
type LineBreak = yaml4.LineBreak

// Line break constants for different platforms.
const (
	LineBreakLN   = yaml4.LineBreakLN   // Unix-style \n (default)
	LineBreakCR   = yaml4.LineBreakCR   // Old Mac-style \r
	LineBreakCRLN = yaml4.LineBreakCRLN // Windows-style \r\n
)

// QuoteStyle represents the quote style to use when quoting is required.
type QuoteStyle = yaml4.QuoteStyle

// Quote style constants for required quoting.
const (
	QuoteSingle = yaml4.QuoteSingle // Prefer single quotes (v4 default)
	QuoteDouble = yaml4.QuoteDouble // Prefer double quotes
	QuoteLegacy = yaml4.QuoteLegacy // Legacy v2/v3 behavior
)
