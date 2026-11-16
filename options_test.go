package yaml

import (
	"errors"
	"io"
	"testing"

	yaml4 "go.yaml.in/yaml/v4"
)

// TestReExportSurface pins the drop-in re-export surface: every
// newly re-exported yaml4 symbol must be addressable from this package, and
// the error constructors must keep their underlying yaml4 semantics (Unwrap).
//
// Before the fix, this file does not compile — the re-exported identifiers
// are undefined, which is exactly the drop-in breakage this surface
// addresses.
func TestReExportSurface(t *testing.T) {
	// Compile-time references to re-exported option constructors and error builders.
	var _ = WithV2Defaults
	var _ = WithV3Defaults
	var _ = WithV4Defaults
	var _ = WithPlugin
	var _ = WithNamedPlugin
	var _ = RegisterPlugin
	var _ = NewLoadError
	var _ = NewDumpError

	// Compile-time references to re-exported types (zero values of each alias).
	var _ AliasDataContext
	var _ AliasDataPlugin
	var _ AliasDataServices
	var _ DepthContext
	var _ DepthKind
	var _ DumpError
	var _ ErrorFormatPlugin
	var _ IndentConfig
	var _ IndentMode
	var _ IndentScope
	var _ IndentStyle
	var _ LoaderLimitsPlugin
	var _ Mark
	var _ MergeKeyConfig
	var _ MergeKeyInput
	var _ MergeKeyPlugin
	var _ ParserPlugin
	var _ PluginEvent
	var _ PluginFactory
	var _ PluginRegistration
	var _ Stage
	var _ Stream
	var _ TabIndentPlugin

	// Compile-time references to re-exported constants.
	var _ = IndentModeAuto
	var _ = IndentModeTabs
	var _ = IndentStyleAuto
	var _ = IndentStyleSpaces
	var _ = IndentStyleTabs
	var _ = IndentScopeDocument
	var _ = IndentScopeStream
	var _ = DepthKindFlow
	var _ = DepthKindBlock
	var _ = ReaderStage
	var _ = ScannerStage
	var _ = ParserStage
	var _ = ComposerStage
	var _ = ResolverStage
	var _ = ConstructorStage
	var _ = RepresenterStage
	var _ = SerializerStage
	var _ = EmitterStage
	var _ = WriterStage

	// Minimal behavioral run: the re-exported version-default preset composes
	// with other options through the re-exported Options combinator.
	var decoded struct {
		Name string `yaml:"name"`
	}
	opts := Options(WithV4Defaults(), WithKnownFields(true))
	if err := Load([]byte("name: drop-in\n"), &decoded, opts); err != nil {
		t.Fatalf("Load with WithV4Defaults failed: %v", err)
	}
	if decoded.Name != "drop-in" {
		t.Fatalf("expected decoded name %q, got %q", "drop-in", decoded.Name)
	}

	// The re-exported error constructors build genuine yaml4 error values:
	// the underlying cause stays reachable via Unwrap (errors.Is).
	loadErr := NewLoadError(ReaderStage, "synthetic load failure", Mark{}, io.ErrUnexpectedEOF)
	if !errors.Is(loadErr, io.ErrUnexpectedEOF) {
		t.Fatalf("NewLoadError lost Unwrap semantics: %v", loadErr)
	}
	dumpErr := NewDumpError(EmitterStage, "synthetic dump failure", io.ErrUnexpectedEOF)
	if !errors.Is(dumpErr, io.ErrUnexpectedEOF) {
		t.Fatalf("NewDumpError lost Unwrap semantics: %v", dumpErr)
	}
}

// TestReExport_AliasedTypeIdentity pins the alias identity: the re-exported
// types (LoadError, LoadErrors, TypeError, Encoding, VersionDirective,
// TagDirective, Unmarshaler) must be true aliases (identical types) of their
// yaml4 counterparts, so drop-in code written against yaml4 keeps compiling
// and running unchanged.
//
// Before the fix, these are defined types — this test does not
// compile ("cannot use yaml.EncodingUTF8 ... as yaml.Encoding value",
// "impossible type assertion"), which is exactly the recorded drop-in
// breakage.
func TestReExport_AliasedTypeIdentity(t *testing.T) {
	// The re-exported error types implement error via the pointer receiver
	// inherited from the underlying yaml4 type.
	var _ error = &LoadError{}
	var _ error = &LoadErrors{}
	var _ error = &TypeError{} //nolint:staticcheck // Reason: legacy type kept for backward compatibility.

	// Re-exported Encoding accepts re-exported constants and interconverts
	// with yaml4.Encoding in both directions without conversion.
	var enc Encoding = EncodingUTF8
	var encY4 yaml4.Encoding = enc
	enc = encY4
	if enc != EncodingUTF8 {
		t.Fatalf("Encoding alias identity broken: %v", enc)
	}

	// Directive types interconvert with their yaml4 counterparts.
	var vd VersionDirective
	var vdY4 yaml4.VersionDirective = vd
	vd = vdY4

	var td TagDirective
	var tdY4 yaml4.TagDirective = td
	td = tdY4

	// Unmarshaler interconverts with yaml4.Unmarshaler in both directions.
	var u Unmarshaler
	var uY4 yaml4.Unmarshaler = u
	var uBack Unmarshaler = uY4

	_, _, _, _ = encY4, vdY4, tdY4, uY4
	_, _, _ = vd, td, uBack

	// A *yaml4.LoadError value is directly assertable as *LoadError.
	var y4LE yaml4.LoadError
	var asLE error = &y4LE
	le, ok := asLE.(*LoadError)
	if !ok {
		t.Fatalf("*yaml4.LoadError is not assertable as *LoadError: %T", asLE)
	}
	if le.Message != "" {
		t.Fatalf("LoadError fields lost after aliasing: %+v", le)
	}
}

// TestReExport_ErrorsAsLoadError verifies that errors.As works with the
// re-exported *LoadError type on a genuine load failure and returns false
// (instead of panicking) on an unrelated error — drop-in compatibility.
//
// Before the fix, errors.As panics with "errors: *target must be interface
// or implement error" (the defined *LoadError does not implement error).
func TestReExport_ErrorsAsLoadError(t *testing.T) {
	var broken struct {
		Name string `yaml:"name"`
	}

	err := Load([]byte("name: [unclosed\n"), &broken)
	if err == nil {
		t.Fatal("expected a load error for malformed YAML, got nil")
	}

	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("errors.As did not match *LoadError for: %v", err)
	}

	unrelated := errors.New("unrelated failure")
	var notLE *LoadError
	if errors.As(unrelated, &notLE) {
		t.Fatal("errors.As matched an unrelated error as *LoadError")
	}
}
