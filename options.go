package yaml

import (
	yaml4 "go.yaml.in/yaml/v4"
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

// Re-exported composite configuration option constructors.
var (
	// OptsYAML evaluates a raw configuration properties string block to parse and extract operational configuration options.
	OptsYAML = yaml4.OptsYAML

	// Options aggregates multiple individual configuration parameters into a unified compound [Option].
	Options = yaml4.Options
)

// Re-exported version-preset and plugin wiring option constructors.
var (
	// WithV2Defaults returns V2-compatible default options. It uses the
	// current v4 error formatter; register the v3 error-format plugin
	// afterwards to select legacy-style error strings.
	WithV2Defaults = yaml4.WithV2Defaults

	// WithV3Defaults returns default options for go-yaml v3 compatibility.
	// Unmarshal uses these defaults; Marshal uses them with the line width
	// overridden to -1 (unlimited).
	WithV3Defaults = yaml4.WithV3Defaults

	// WithV4Defaults returns the current V4 default options. These are the
	// default settings when dumping or loading operations run without any options.
	WithV4Defaults = yaml4.WithV4Defaults

	// WithPlugin registers one or more plugins for YAML processing. Each
	// plugin implements one or more of the plugin interfaces re-exported below.
	WithPlugin = yaml4.WithPlugin

	// WithNamedPlugin selects the plugin implementation whose name matches the API.
	WithNamedPlugin = yaml4.WithNamedPlugin
)

// RegisterPlugin registers a named plugin implementation for [OptsYAML] and
// [WithNamedPlugin]. Call it during application startup, before parsing
// configuration; duplicate API and implementation name pairs are errors.
var RegisterPlugin = yaml4.RegisterPlugin

// Re-exported error construction helpers with preserved underlying causes.
var (
	// NewLoadError creates a yaml4 loading error with an underlying cause.
	// The cause stays accessible via Unwrap for use with errors.Is and errors.As.
	NewLoadError = yaml4.NewLoadError

	// NewDumpError creates a yaml4 dumping error with an underlying cause.
	// The cause stays accessible via Unwrap for use with errors.Is and errors.As.
	NewDumpError = yaml4.NewDumpError
)

// Re-exported core document node and interface model types.
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
type Unmarshaler = yaml4.Unmarshaler

// Re-exported document directive and encoding descriptor types.
type (
	// VersionDirective describes explicit version specifications contained inside structural document directives blocks.
	VersionDirective = yaml4.VersionDirective

	// TagDirective maps shorthand macro configurations used within structural tag declarations inside document blocks.
	TagDirective = yaml4.TagDirective

	// Encoding represents text streaming character layout types.
	Encoding = yaml4.Encoding
)

// Re-exported character data streaming stream encodings configuration constants.
const (
	EncodingAny     = yaml4.EncodingAny
	EncodingUTF8    = yaml4.EncodingUTF8
	EncodingUTF16LE = yaml4.EncodingUTF16LE
	EncodingUTF16BE = yaml4.EncodingUTF16BE
)

// Re-exported document loading error model types.
type (
	// LoadError represents an execution failure encountered during document unpacking operations,
	// containing file coordinates and detailed diagnostics about the incident location.
	LoadError = yaml4.LoadError

	// LoadErrors represents a composite validation type capturing multiple field processing failures
	// collected during complex multi-property decoding tasks.
	LoadErrors = yaml4.LoadErrors

	// TypeError represents a legacy semantic decoding tracking model preserved exclusively for backwards compatibility.
	//
	// Deprecated: Migrate to the modernized [LoadErrors] API model.
	//
	//nolint:staticcheck // Reason: preserving legacy struct for backward compatibility layouts API contracts.
	TypeError = yaml4.TypeError
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

// Re-exported error position and stream metadata model types.
type (
	// DumpError represents an error that occurred while dumping a YAML
	// document. It identifies the processing stage where the error occurred
	// and provides an optional underlying cause via Unwrap.
	DumpError = yaml4.DumpError

	// Mark represents a position in the YAML document.
	Mark = yaml4.Mark

	// Stream holds stream-level metadata for [StreamNode]: encoding, version
	// directive, and tag directives.
	Stream = yaml4.Stream

	// Stage identifies the processing stage where an error occurred during
	// YAML loading or dumping.
	Stage = yaml4.Stage
)

// Re-exported document parsing depth accounting model types.
type (
	// DepthKind represents the type of nesting (flow or block).
	DepthKind = yaml4.DepthKind

	// DepthContext holds context about a nesting depth check.
	DepthContext = yaml4.DepthContext
)

// Re-exported document element depth nesting category constant values.
const (
	// DepthKindFlow represents flow-style nesting (inside [] or {}).
	DepthKindFlow = yaml4.DepthKindFlow

	// DepthKindBlock represents block-style nesting (inside indentation).
	DepthKindBlock = yaml4.DepthKindBlock
)

// Re-exported indentation layout configuration model types.
type (
	// IndentConfig configures tab-indentation loading and dumping.
	IndentConfig = yaml4.IndentConfig

	// IndentMode supplies loading and dumping defaults.
	IndentMode = yaml4.IndentMode

	// IndentScope controls how long an auto-detected style remains active.
	IndentScope = yaml4.IndentScope

	// IndentStyle identifies the characters used for structural indentation.
	IndentStyle = yaml4.IndentStyle
)

// Re-exported indentation mode selection constant values.
const (
	// IndentModeAuto selects automatic indentation defaults.
	IndentModeAuto = yaml4.IndentModeAuto

	// IndentModeTabs selects tab-aware indentation defaults.
	IndentModeTabs = yaml4.IndentModeTabs
)

// Re-exported structural indentation character selection constant values.
const (
	// IndentStyleAuto selects the automatically detected indentation characters.
	IndentStyleAuto = yaml4.IndentStyleAuto

	// IndentStyleSpaces selects space characters for structural indentation.
	IndentStyleSpaces = yaml4.IndentStyleSpaces

	// IndentStyleTabs selects tab characters for structural indentation.
	IndentStyleTabs = yaml4.IndentStyleTabs
)

// Re-exported indentation style activation scope constant values.
const (
	// IndentScopeDocument keeps an auto-detected style active for the document.
	IndentScopeDocument = yaml4.IndentScopeDocument

	// IndentScopeStream keeps an auto-detected style active for the whole stream.
	IndentScopeStream = yaml4.IndentScopeStream
)

// Re-exported merge-key processing plugin model types.
type (
	// MergeKeyPlugin chooses the effective mapping before value construction.
	MergeKeyPlugin = yaml4.MergeKeyPlugin

	// MergeKeyConfig controls merge recognition, repeated directives, and conflicts.
	MergeKeyConfig = yaml4.MergeKeyConfig

	// MergeKeyInput supplies read-only target and source mappings to an algorithm.
	MergeKeyInput = yaml4.MergeKeyInput
)

// Re-exported anchor and alias data handling plugin model types.
type (
	// AliasDataPlugin creates per-stream anchor and alias policies.
	AliasDataPlugin = yaml4.AliasDataPlugin

	// AliasDataContext owns node-based anchor storage and alias lookup.
	AliasDataContext = yaml4.AliasDataContext

	// AliasDataServices supplies source snapshots and node conversions.
	AliasDataServices = yaml4.AliasDataServices
)

// Re-exported YAML processing plugin extension model types.
type (
	// PluginEvent carries source-independent YAML syntax information.
	PluginEvent = yaml4.PluginEvent

	// PluginFactory constructs a plugin from its plugin-specific configuration.
	PluginFactory = yaml4.PluginFactory

	// PluginRegistration describes one named implementation of a plugin API.
	// Version is empty for an unversioned implementation; Default selects the
	// implementation used by boolean configuration.
	PluginRegistration = yaml4.PluginRegistration

	// ParserPlugin supplies a complete YAML event stream in place of native parsing.
	ParserPlugin = yaml4.ParserPlugin

	// TabIndentPlugin enables tab-aware loading and tab-indented dumping.
	TabIndentPlugin = yaml4.TabIndentPlugin

	// LoaderLimitsPlugin configures safety limits for YAML loading: CheckDepth
	// runs on each nesting depth increase, CheckAlias on each alias expansion.
	LoaderLimitsPlugin = yaml4.LoaderLimitsPlugin

	// ErrorFormatPlugin customizes how YAML load and dump errors are rendered:
	// FormatLoadError and FormatDumpError produce the error message strings.
	ErrorFormatPlugin = yaml4.ErrorFormatPlugin
)

// Re-exported YAML loading processing stage constant values.
const (
	// ReaderStage is the input reading and encoding stage.
	ReaderStage = yaml4.ReaderStage

	// ScannerStage is the tokenization stage.
	ScannerStage = yaml4.ScannerStage

	// ParserStage is the event stream parsing stage.
	ParserStage = yaml4.ParserStage

	// ComposerStage is the node tree construction stage.
	ComposerStage = yaml4.ComposerStage

	// ResolverStage is the tag resolution stage.
	ResolverStage = yaml4.ResolverStage

	// ConstructorStage is the Go value construction stage.
	ConstructorStage = yaml4.ConstructorStage
)

// Re-exported YAML dumping processing stage constant values.
const (
	// RepresenterStage is the Go value to Node tree stage.
	RepresenterStage = yaml4.RepresenterStage

	// SerializerStage is the Node tree to event stage.
	SerializerStage = yaml4.SerializerStage

	// EmitterStage is the events to YAML bytes stage.
	EmitterStage = yaml4.EmitterStage

	// WriterStage is the output writing stage.
	WriterStage = yaml4.WriterStage
)
