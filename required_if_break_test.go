package yaml

import (
	"strings"
	"testing"
)

// The break modifier of the validate tag: a standalone break token splits the
// tag into the unconditional head and the conditional tail governed by the
// required_if parsed before break. The tests below exercise the whole break
// contract through the public API only (Load / Validate / HelpStr): the tail
// value rules run only when the condition is met, their configuration
// mistakes surface regardless of the condition, and a tag without break keeps
// its exact pre-break behavior.

// breakTelegram is the shared condition holder: the delivery mode.
type breakTelegram struct {
	Mode string `yaml:"mode" validate:"choice=webhook,pooling"`
}

// breakWebhook carries the canonical break tag of the contract example: the
// tail url/minlen rules apply only in webhook mode.
type breakWebhook struct {
	URL string `yaml:"url" validate:"required_if=telegram.mode==webhook,break,url,minlen=10"`
}

// breakConfig is the Telegram/webhook TLS-adjacent pair.
type breakConfig struct {
	Telegram breakTelegram `yaml:"telegram"`
	Webhook  breakWebhook  `yaml:"webhook"`
}

// TestRequiredIfBreak_MetEmptyFieldRequiresTail pins the met branch: the
// condition fires, the empty field reports the required error and the tail
// value rules (url, minlen) run against the zero value.
func TestRequiredIfBreak_MetEmptyFieldRequiresTail(t *testing.T) {
	cfg := breakConfig{}
	err := Load([]byte(`
telegram:
  mode: webhook
`), &cfg)
	if err == nil {
		t.Fatal("expected webhook mode with an empty url to fail, got nil")
	}

	for _, want := range []string{
		"field webhook.url: is required when field telegram.mode==webhook",
		"field webhook.url: value \"\" is missing a URL scheme separator (e.g., scheme://host)",
		"field webhook.url: string length 0 is less than minlen 10",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected met-condition error to contain %q, got:\n%s", want, err)
		}
	}
}

// TestRequiredIfBreak_NotMetEmptyFieldIsSilent is the main break case: with
// the condition false the empty field carries no tail rules at all and the
// configuration validates cleanly.
func TestRequiredIfBreak_NotMetEmptyFieldIsSilent(t *testing.T) {
	cfg := breakConfig{}
	err := Load([]byte(`
telegram:
  mode: pooling
`), &cfg)
	if err != nil {
		t.Fatalf("expected pooling mode with an empty url to pass, got:\n%s", err)
	}
}

// TestRequiredIfBreak_NotMetNonEmptyFieldIsSilent pins the accepted trade-off
// of the break contract (pooling mode + garbage value is silently valid): the
// tail value rules are skipped entirely when the condition is false, even for
// a non-empty field value.
func TestRequiredIfBreak_NotMetNonEmptyFieldIsSilent(t *testing.T) {
	cfg := breakConfig{}
	err := Load([]byte(`
telegram:
  mode: pooling
webhook:
  url: "not-a-url"
`), &cfg)
	if err != nil {
		t.Fatalf("expected pooling mode with a garbage url to pass (the accepted break trade-off), got:\n%s", err)
	}
}

// TestRequiredIfBreak_TailConfigErrorReportedWhenNotMet pins the closed hole
// of the break contract: configuration mistakes of the tail rules
// (minlen=abc) are reported regardless of the condition.
func TestRequiredIfBreak_TailConfigErrorReportedWhenNotMet(t *testing.T) {
	type cfg struct {
		Telegram breakTelegram `yaml:"telegram"`
		URL      string        `yaml:"url" validate:"required_if=telegram.mode==webhook,break,minlen=abc"`
	}

	for _, url := range []string{"", "whatever"} {
		c := cfg{}
		err := Load([]byte(`
telegram:
  mode: pooling
url: "`+url+`"
`), &c)
		if err == nil {
			t.Fatalf("expected the tail minlen=abc configuration mistake to be reported with url=%q, got nil", url)
		}
		if !strings.Contains(err.Error(), `field url: invalid validator configuration: minlen "abc"`) {
			t.Errorf("expected the minlen=abc configuration mistake with url=%q, got:\n%s", url, err)
		}
	}
}

// TestRequiredIfBreak_BreakWithoutRequiredIf: break without a preceding
// required_if has no condition to govern the tail — a configuration mistake.
func TestRequiredIfBreak_BreakWithoutRequiredIf(t *testing.T) {
	type cfg struct {
		URL string `yaml:"url" validate:"break,url"`
	}

	c := cfg{}
	err := Validate(&c)
	if err == nil {
		t.Fatal("expected break without a preceding required_if to fail, got nil")
	}
	if !strings.Contains(err.Error(), "field url: invalid validator configuration: break rule must be preceded by a required_if rule") {
		t.Errorf("expected the break-without-required_if mistake, got:\n%s", err)
	}
}

// TestRequiredIfBreak_SecondBreak: a tag may carry a single break token.
func TestRequiredIfBreak_SecondBreak(t *testing.T) {
	type cfg struct {
		Telegram breakTelegram `yaml:"telegram"`
		URL      string        `yaml:"url" validate:"required_if=telegram.mode==webhook,break,url,break,minlen=3"`
	}

	c := cfg{}
	err := Load([]byte(`
telegram:
  mode: pooling
`), &c)
	if err == nil {
		t.Fatal("expected a second break token to fail, got nil")
	}
	if !strings.Contains(err.Error(), "field url: invalid validator configuration: duplicate break rule") {
		t.Errorf("expected the duplicate break mistake, got:\n%s", err)
	}
}

// TestRequiredIfBreak_RequiredIfAfterBreak: the tail condition is defined by
// the required_if preceding break, so a required_if after break is a
// configuration mistake.
func TestRequiredIfBreak_RequiredIfAfterBreak(t *testing.T) {
	type cfg struct {
		Telegram breakTelegram `yaml:"telegram"`
		URL      string        `yaml:"url" validate:"required_if=telegram.mode==webhook,break,url,required_if=telegram.mode==pooling"`
	}

	c := cfg{}
	err := Load([]byte(`
telegram:
  mode: pooling
`), &c)
	if err == nil {
		t.Fatal("expected a required_if after break to fail, got nil")
	}
	if !strings.Contains(err.Error(), "field url: invalid validator configuration: required_if rule must be placed before break") {
		t.Errorf("expected the required_if-after-break mistake, got:\n%s", err)
	}
}

// TestRequiredIfBreak_BreakInsidePathEntry: the service token on a nested
// path target ("item.break") is rejected — the tail condition governs the
// owner field's value rules only.
func TestRequiredIfBreak_BreakInsidePathEntry(t *testing.T) {
	type cfg struct {
		URL string `yaml:"url" validate:"item.break"`
	}

	c := cfg{}
	err := Validate(&c)
	if err == nil {
		t.Fatal("expected break inside a path entry to fail, got nil")
	}
	if !strings.Contains(err.Error(), "field url: invalid validator configuration: break rule inside a path entry is not supported") {
		t.Errorf("expected the break-inside-path-entry mistake, got:\n%s", err)
	}
}

// TestRequiredIfBreak_CollectionElements exercises the collection rules of
// the break contract: the tail value rules apply to the collection itself
// (mincount), but never inherit to its elements (choice stays off them, v1).
func TestRequiredIfBreak_CollectionElements(t *testing.T) {
	type cfg struct {
		Mode  string   `yaml:"mode" validate:"choice=secure,dev"`
		Peers []string `yaml:"peers" validate:"required_if=mode==secure,break,mincount=2,choice=alpha,beta"`
	}

	// Not met: the elements stay clean — no tail rule applies to them.
	notMet := cfg{}
	if err := Load([]byte(`
mode: dev
peers: ["gamma"]
`), &notMet); err != nil {
		t.Fatalf("expected a not-met collection to pass, got:\n%s", err)
	}

	// Met: the tail mincount applies to the collection, but the conditional
	// choice does not inherit to the elements — "gamma" stays silent.
	met := cfg{}
	if err := Load([]byte(`
mode: secure
peers: ["alpha", "gamma"]
`), &met); err != nil {
		t.Fatalf("expected conditional tail rules not to inherit to elements, got:\n%s", err)
	}

	// Met: the collection's own value check fires.
	metShort := cfg{}
	err := Load([]byte(`
mode: secure
peers: ["alpha"]
`), &metShort)
	if err == nil {
		t.Fatal("expected the met tail mincount to fire on the collection, got nil")
	}
	if !strings.Contains(err.Error(), "field peers: collection size 1 is less than mincount 2") {
		t.Errorf("expected the tail mincount to apply to the collection, got:\n%s", err)
	}
}

// TestRequiredIfBreak_CollectionTailConfigErrorOnCollection pins the other
// half of the collection contract: tail configuration mistakes are reported
// on the collection itself, regardless of the condition and of the elements.
func TestRequiredIfBreak_CollectionTailConfigErrorOnCollection(t *testing.T) {
	type cfg struct {
		Mode  string   `yaml:"mode" validate:"choice=secure,dev"`
		Peers []string `yaml:"peers" validate:"required_if=mode==secure,break,choice="`
	}

	for _, mode := range []string{"dev", "secure"} {
		c := cfg{}
		err := Load([]byte(`
mode: `+mode+`
peers: ["anything"]
`), &c)
		if err == nil {
			t.Fatalf("expected the empty tail choice list to be reported on the collection (mode=%s), got nil", mode)
		}
		if !strings.Contains(err.Error(), "field peers: invalid validator configuration: empty choice list") {
			t.Errorf("expected the empty tail choice mistake on the collection (mode=%s), got:\n%s", mode, err)
		}
	}
}

// TestRequiredIfBreak_LegacyCondition covers the deprecated colon form as the
// tail condition: met applies the tail, not met keeps the field silent.
func TestRequiredIfBreak_LegacyCondition(t *testing.T) {
	type cfg struct {
		Mode string `yaml:"mode" validate:"choice=webhook,pooling"`
		URL  string `yaml:"url" validate:"required_if=mode:webhook,break,url"`
	}

	met := cfg{}
	err := Load([]byte(`
mode: webhook
url: "not-a-url"
`), &met)
	if err == nil {
		t.Fatal("expected the legacy met condition to fire, got nil")
	}
	if strings.Contains(err.Error(), "is required when") {
		t.Errorf("expected no required error on a non-empty field, got:\n%s", err)
	}
	if !strings.Contains(err.Error(), `field url: value "not-a-url" is missing a URL scheme separator`) {
		t.Errorf("expected the tail url rule to apply, got:\n%s", err)
	}

	// Met on a zero field: the legacy required error keeps its layout.
	metEmpty := cfg{}
	err = Load([]byte(`
mode: webhook
`), &metEmpty)
	if err == nil {
		t.Fatal("expected the legacy met condition to require the empty field, got nil")
	}
	if !strings.Contains(err.Error(), "field url: is required when field mode=webhook") {
		t.Errorf("expected the legacy required error, got:\n%s", err)
	}

	// Not met: the accepted trade-off again — garbage url stays silent.
	notMet := cfg{}
	if err := Load([]byte(`
mode: pooling
url: "not-a-url"
`), &notMet); err != nil {
		t.Fatalf("expected the legacy not-met condition to keep the field silent, got:\n%s", err)
	}
}

// TestRequiredIfBreak_EqualityOperandBreak pins the tokenizer guarantee: the
// word break inside a parameter value is data, not the service token — in the
// choice list and in the required_if atom alike.
func TestRequiredIfBreak_EqualityOperandBreak(t *testing.T) {
	type cfg struct {
		Mode string `yaml:"mode" validate:"choice=break,webhook"`
		URL  string `yaml:"url" validate:"required_if=mode==break,break,minlen=5"`
	}

	// mode=break: the choice accepts the literal value, the atom "mode==break"
	// is met and the tail minlen applies.
	met := cfg{}
	err := Load([]byte(`
mode: break
url: "abc"
`), &met)
	if err == nil {
		t.Fatal("expected the met mode==break condition to apply the tail minlen, got nil")
	}
	if !strings.Contains(err.Error(), "field url: string length 3 is less than minlen 5") {
		t.Errorf("expected the tail minlen to apply for mode=break, got:\n%s", err)
	}

	// mode=webhook: the atom is not met, the field stays silent.
	notMet := cfg{}
	if err := Load([]byte(`
mode: webhook
url: "abc"
`), &notMet); err != nil {
		t.Fatalf("expected mode=webhook to keep the field silent, got:\n%s", err)
	}
}

// TestRequiredIfBreak_BooleanExpression covers the boolean-expression form of
// required_if as the tail condition.
func TestRequiredIfBreak_BooleanExpression(t *testing.T) {
	type cfg struct {
		Telegram breakTelegram `yaml:"telegram"`
		URL      string        `yaml:"url" validate:"required_if=(telegram.mode==webhook||telegram.mode==fallback),break,minlen=10"`
	}

	met := cfg{}
	err := Load([]byte(`
telegram:
  mode: webhook
url: "short"
`), &met)
	if err == nil {
		t.Fatal("expected the met expression condition to apply the tail minlen, got nil")
	}
	if !strings.Contains(err.Error(), "field url: string length 5 is less than minlen 10") {
		t.Errorf("expected the tail minlen to apply, got:\n%s", err)
	}

	// Met on a zero field: the expression required error keeps its layout.
	metEmpty := cfg{}
	err = Load([]byte(`
telegram:
  mode: webhook
`), &metEmpty)
	if err == nil {
		t.Fatal("expected the met expression condition to require the empty field, got nil")
	}
	if !strings.Contains(err.Error(), "field url: is required when (telegram.mode==webhook||telegram.mode==fallback)") {
		t.Errorf("expected the expression required error, got:\n%s", err)
	}

	notMet := cfg{}
	if err := Load([]byte(`
telegram:
  mode: pooling
url: "short"
`), &notMet); err != nil {
		t.Fatalf("expected the not-met expression to keep the field silent, got:\n%s", err)
	}
}

// TestRequiredIfBreak_TagsWithoutBreakUnchanged is the regression guard: a
// tag without break behaves exactly as before — the same rules run
// unconditionally, even when the condition is false.
func TestRequiredIfBreak_TagsWithoutBreakUnchanged(t *testing.T) {
	type cfg struct {
		Telegram breakTelegram `yaml:"telegram"`
		URL      string        `yaml:"url" validate:"required_if=telegram.mode==webhook,url"`
		Host     string        `yaml:"host" validate:"required,minlen=3"`
	}

	// Not met, garbage url: the url rule still applies without break — the
	// unconditional pre-break behavior the break modifier opts out of.
	notMet := cfg{}
	err := Load([]byte(`
telegram:
  mode: pooling
host: "h1"
url: "not-a-url"
`), &notMet)
	if err == nil {
		t.Fatal("expected the unconditional url rule to fire without break, got nil")
	}
	if !strings.Contains(err.Error(), `field url: value "not-a-url" is missing a URL scheme separator`) {
		t.Errorf("expected the unconditional url rule to apply, got:\n%s", err)
	}

	// Zero-value required host: unchanged.
	required := cfg{}
	err = Load([]byte(`
telegram:
  mode: pooling
url: "https://ok.example.com"
`), &required)
	if err == nil {
		t.Fatal("expected the empty required host to fail, got nil")
	}
	if !strings.Contains(err.Error(), "field host: is empty, but required") {
		t.Errorf("expected the required rule to apply, got:\n%s", err)
	}

	// Met with garbage: unchanged.
	met := cfg{}
	err = Load([]byte(`
telegram:
  mode: webhook
host: "abc"
url: "not-a-url"
`), &met)
	if err == nil {
		t.Fatal("expected the garbage url to fail without break, got nil")
	}
	if !strings.Contains(err.Error(), `field url: value "not-a-url" is missing a URL scheme separator`) {
		t.Errorf("expected the unconditional url rule to apply, got:\n%s", err)
	}
}

// TestRequiredIfBreak_HelpHidesBreak pins the help rendering: the service
// break token stays out of the documented rule list while the remaining
// segments keep their original formatting; a tag without break renders
// bit-for-bit.
func TestRequiredIfBreak_HelpHidesBreak(t *testing.T) {
	out := HelpStr(breakConfig{})
	if strings.Contains(out, "break") {
		t.Errorf("expected the service break token to stay undocumented, got:\n%s", out)
	}
	if !strings.Contains(out, "validate: [required_if=telegram.mode==webhook,url,minlen=10]") {
		t.Errorf("expected the break-free tag rendering, got:\n%s", out)
	}

	// A tag without break renders exactly as before.
	plain := HelpStr(breakTelegram{})
	if !strings.Contains(plain, "validate: [choice=webhook,pooling]") {
		t.Errorf("expected the plain tag to render bit-for-bit, got:\n%s", plain)
	}

	// A tag reduced to nothing by the strip carries no constraint at all and
	// is omitted from the metadata entirely.
	bare := HelpStr(struct {
		URL string `yaml:"url" validate:"break" description:"webhook endpoint"`
	}{})
	if strings.Contains(bare, "validate:") || strings.Contains(bare, "break") {
		t.Errorf("expected a break-only tag to carry no validate metadata, got:\n%s", bare)
	}
}

// TestRequiredIfBreak_TailRegexpConfigErrorReportedWhenNotMet pins the
// declaration audit of the tail regexp: a pattern that cannot compile is
// reported regardless of the condition with the value-phase message; the
// audit never matches the field value, and a quoted pattern is stripped
// before the compile exactly like the value phase does.
func TestRequiredIfBreak_TailRegexpConfigErrorReportedWhenNotMet(t *testing.T) {
	type cfg struct {
		Telegram breakTelegram `yaml:"telegram"`
		URL      string        `yaml:"url" validate:"required_if=telegram.mode==webhook,break,regexp=(unclosed"`
	}

	for _, url := range []string{"", "whatever"} {
		c := cfg{}
		err := Load([]byte(`
telegram:
  mode: pooling
url: "`+url+`"
`), &c)
		if err == nil {
			t.Fatalf("expected the tail regexp compile mistake to be reported with url=%q, got nil", url)
		}
		if !strings.Contains(err.Error(), `field url: invalid regular expression syntax "(unclosed"`) {
			t.Errorf("expected the regexp compile mistake with url=%q, got:\n%s", url, err)
		}
	}

	// Met: the value phase names the same mistake exactly once — the audit
	// never runs on a met condition.
	met := cfg{}
	err := Load([]byte(`
telegram:
  mode: webhook
url: "whatever"
`), &met)
	if err == nil {
		t.Fatal("expected the met value phase to report the regexp compile mistake, got nil")
	}
	if strings.Count(err.Error(), "invalid regular expression syntax") != 1 {
		t.Errorf("expected a single regexp compile report, got:\n%s", err)
	}

	// A quoted comma-carrying pattern survives the scanner and compiles
	// after the quote strip: no audit report on a valid declaration.
	quoted := struct {
		Telegram breakTelegram `yaml:"telegram"`
		URL      string        `yaml:"url" validate:"required_if=telegram.mode==webhook,break,regexp='a{2,3}'"`
	}{}
	if err := Load([]byte(`
telegram:
  mode: pooling
url: "anything"
`), &quoted); err != nil {
		t.Fatalf("expected the quoted pattern to compile in the audit, got:\n%s", err)
	}
}

// TestRequiredIfBreak_TailFormatConfigErrorReportedWhenNotMet pins the
// declaration audit of the tail format token: an unknown token is reported
// regardless of the condition with the value-phase message, and the
// string-kind filter of the value phase is mirrored — a format rule on a
// kind the format switch never reaches stays silent.
func TestRequiredIfBreak_TailFormatConfigErrorReportedWhenNotMet(t *testing.T) {
	type cfg struct {
		Telegram breakTelegram `yaml:"telegram"`
		URL      string        `yaml:"url" validate:"required_if=telegram.mode==webhook,break,format=bogus"`
	}

	for _, url := range []string{"", "whatever"} {
		c := cfg{}
		err := Load([]byte(`
telegram:
  mode: pooling
url: "`+url+`"
`), &c)
		if err == nil {
			t.Fatalf("expected the tail format=bogus mistake to be reported with url=%q, got nil", url)
		}
		if !strings.Contains(err.Error(), `field url: unknown format "bogus" (expected ip, ipv4, ipv6, uuid or hex)`) {
			t.Errorf("expected the unknown format mistake with url=%q, got:\n%s", url, err)
		}
	}

	// Met: the value phase names the same mistake exactly once.
	met := cfg{}
	err := Load([]byte(`
telegram:
  mode: webhook
url: "whatever"
`), &met)
	if err == nil {
		t.Fatal("expected the met value phase to report the unknown format, got nil")
	}
	if strings.Count(err.Error(), "unknown format") != 1 {
		t.Errorf("expected a single unknown-format report, got:\n%s", err)
	}

	// The audit mirrors the value-phase kind filter: format on a non-string
	// field never reaches the format switch, so the audit stays silent too.
	intFmt := struct {
		Telegram breakTelegram `yaml:"telegram"`
		Port     int           `yaml:"port" validate:"required_if=telegram.mode==webhook,break,format=bogus"`
	}{}
	if err := Load([]byte(`
telegram:
  mode: pooling
port: 8080
`), &intFmt); err != nil {
		t.Fatalf("expected format=bogus on a non-string tail to stay silent, got:\n%s", err)
	}
}

// TestRequiredIfBreak_CrossSegmentBoundaryConflict pins the head×tail half
// of the conflicting-boundaries audit: min in the head with gt in the tail
// is the same declaration conflict as a same-segment pair — reported
// regardless of the condition, exactly once per state. Head-only pairs stay
// validateValue's business and must not double-report.
func TestRequiredIfBreak_CrossSegmentBoundaryConflict(t *testing.T) {
	type cfg struct {
		Telegram breakTelegram `yaml:"telegram"`
		Port     int           `yaml:"port" validate:"required_if=telegram.mode==webhook,min=100,break,gt=200"`
	}

	// Not met: the tail never joins the value phase — the audit names the
	// cross-segment pair.
	notMet := cfg{}
	err := Load([]byte(`
telegram:
  mode: pooling
port: 150
`), &notMet)
	if err == nil {
		t.Fatal("expected the cross-segment boundary conflict to be reported when not met, got nil")
	}
	if !strings.Contains(err.Error(), "field port: invalid validator configuration: conflicting boundaries metrics") {
		t.Errorf("expected the cross-segment conflict, got:\n%s", err)
	}

	// Met: the merged rule set names the same pair exactly once.
	met := cfg{}
	err = Load([]byte(`
telegram:
  mode: webhook
port: 150
`), &met)
	if err == nil {
		t.Fatal("expected the merged boundaries to conflict when met, got nil")
	}
	if strings.Count(err.Error(), "conflicting boundaries metrics") != 1 {
		t.Errorf("expected a single conflicting-boundaries report when met, got:\n%s", err)
	}

	// A head-only pair (min and gt both before break) reports through the
	// value phase alone: exactly one report in the not-met state.
	headOnly := struct {
		Telegram breakTelegram `yaml:"telegram"`
		Port     int           `yaml:"port" validate:"required_if=telegram.mode==webhook,min=100,gt=200,break,maxlen=3"`
	}{}
	err = Load([]byte(`
telegram:
  mode: pooling
port: 150
`), &headOnly)
	if err == nil {
		t.Fatal("expected the head-only boundary conflict to be reported, got nil")
	}
	if strings.Count(err.Error(), "conflicting boundaries metrics") != 1 {
		t.Errorf("expected a single conflicting-boundaries report for head-only pairs, got:\n%s", err)
	}
}

// TestRequiredIfBreak_BreakValueFormRejected pins the distorted token form:
// the flag-rule scan accepts '=' right after break, so "break=x" splits the
// tag while the '='-tail is silently skipped — the mistake is reported and
// the tail stays conditional.
func TestRequiredIfBreak_BreakValueFormRejected(t *testing.T) {
	type cfg struct {
		Telegram breakTelegram `yaml:"telegram"`
		URL      string        `yaml:"url" validate:"required_if=telegram.mode==webhook,break=x,minlen=20"`
	}

	// Not met: only the form mistake — the tail minlen stays suppressed.
	c := cfg{}
	err := Load([]byte(`
telegram:
  mode: pooling
`), &c)
	if err == nil {
		t.Fatal("expected the break=x form to fail, got nil")
	}
	if !strings.Contains(err.Error(), "field url: invalid validator configuration: break rule takes no value") {
		t.Errorf("expected the break-takes-no-value mistake, got:\n%s", err)
	}
	if strings.Contains(err.Error(), "string length") {
		t.Errorf("expected the tail to stay conditional on the distorted break form, got:\n%s", err)
	}
}
