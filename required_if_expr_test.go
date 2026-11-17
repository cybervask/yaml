package yaml

import (
	"strings"
	"testing"
)

// The tests below pin the boolean expression form of required_if ("&&",
// "||", grouping parentheses) and exercise it strictly through the public
// API (Load / Validate). The file deliberately references no unexported
// package symbols, so it compiles against code without the feature, where
// every expression-form test fails (A/B pin) and the single-condition
// regression cases keep passing.

// TestRequiredIfExpr_ThreeAtomsAnd covers the conjunction of three atoms
// over dot-path targets: key_file becomes mandatory only when the mode is
// "webhook", the URL is TLS and the certificate file is provided.
func TestRequiredIfExpr_ThreeAtomsAnd(t *testing.T) {
	type tlsServer struct {
		Mode     string `yaml:"mode" validate:"choice=webhook,pooling"`
		URL      string `yaml:"url"`
		CertFile string `yaml:"cert_file"`
	}

	type tlsConfig struct {
		Server  tlsServer `yaml:"server"`
		KeyFile string    `yaml:"key_file" validate:"required_if=server.mode==webhook&&server.url~=^https:&&server.cert_file^=^$"`
	}

	// All three atoms hold and the guarded field is empty: one violation
	// reporting the whole original expression.
	var violation tlsConfig
	err := Load([]byte(`
server:
  mode: webhook
  url: "https://hooks.example.com"
  cert_file: "/etc/cert.pem"
`), &violation)
	if err == nil || !strings.Contains(err.Error(), "field key_file: is required when server.mode==webhook&&server.url~=^https:&&server.cert_file^=^$") {
		t.Fatalf("expected required_if expression violation, got: %v", err)
	}

	// The same condition with the guarded field filled: no error.
	var complete tlsConfig
	err = Load([]byte(`
server:
  mode: webhook
  url: "https://hooks.example.com"
  cert_file: "/etc/cert.pem"
key_file: "/etc/key.pem"
`), &complete)
	if err != nil {
		t.Fatalf("expected complete configuration to pass, got: %v", err)
	}

	// Each atom failing alone keeps the combination false.
	var nonTLS tlsConfig
	err = Load([]byte(`
server:
  mode: webhook
  url: "http://hooks.example.com"
  cert_file: "/etc/cert.pem"
`), &nonTLS)
	if err != nil {
		t.Fatalf("expected non-TLS URL to disable the requirement, got: %v", err)
	}

	var noCertificate tlsConfig
	err = Load([]byte(`
server:
  mode: webhook
  url: "https://hooks.example.com"
`), &noCertificate)
	if err != nil {
		t.Fatalf("expected missing certificate to disable the requirement, got: %v", err)
	}

	var pooling tlsConfig
	err = Load([]byte(`
server:
  mode: pooling
`), &pooling)
	if err != nil {
		t.Fatalf("expected pooling mode to disable the requirement, got: %v", err)
	}
}

// TestRequiredIfExpr_ParensOr covers A&&(B||C): parentheses group the OR so
// that the guarded field is required when the gate holds and either operand
// of the group does.
func TestRequiredIfExpr_ParensOr(t *testing.T) {
	type parensConfig struct {
		Mode    string `yaml:"mode"`
		URL     string `yaml:"url"`
		Cert    string `yaml:"cert"`
		KeyFile string `yaml:"key_file" validate:"required_if=mode==webhook&&(url~=^https:||cert^=^$)"`
	}

	viaCert := parensConfig{Mode: "webhook", URL: "http://x", Cert: "/c.pem"}
	err := Validate(&viaCert)
	if err == nil || !strings.Contains(err.Error(), "field key_file: is required when mode==webhook&&(url~=^https:||cert^=^$)") {
		t.Fatalf("expected violation for the true OR operand, got: %v", err)
	}

	viaURL := parensConfig{Mode: "webhook", URL: "https://x"}
	err = Validate(&viaURL)
	if err == nil || !strings.Contains(err.Error(), "field key_file: is required when mode==webhook&&(url~=^https:||cert^=^$)") {
		t.Fatalf("expected violation for the other true OR operand, got: %v", err)
	}

	bothFalse := parensConfig{Mode: "webhook", URL: "http://x"}
	if err := Validate(&bothFalse); err != nil {
		t.Fatalf("expected both OR operands false to disable the requirement, got: %v", err)
	}

	gateFalse := parensConfig{Mode: "pooling", URL: "https://x"}
	if err := Validate(&gateFalse); err != nil {
		t.Fatalf("expected the false gate to disable the requirement, got: %v", err)
	}
}

// TestRequiredIfExpr_AndBindsTighterThanOr pins the C/Go precedence: the
// rule "A||B&&C" must equal "A||(B&&C)", i.e. "(A&&B)||C". TokenA and
// TokenB carry the bare and the explicitly parenthesized form of the same
// expression and must produce identical requirements for every input
// combination.
func TestRequiredIfExpr_AndBindsTighterThanOr(t *testing.T) {
	type precedenceConfig struct {
		Role   string `yaml:"role"`
		DB     string `yaml:"db"`
		Extra  string `yaml:"extra"`
		TokenA string `yaml:"token_a" validate:"required_if=role!=prod&&db==on||extra==on"`
		TokenB string `yaml:"token_b" validate:"required_if=(role!=prod&&db==on)||extra==on"`
	}

	// Distinguishing combinations: with the wrong precedence
	// ((A||B)&&C) the first would not fire and the second two would.
	distinguishing := []precedenceConfig{
		{Role: "admin", DB: "on", Extra: "off"}, // conjunction true, disjunction false
		{Role: "prod", DB: "off", Extra: "on"},  // conjunction false, disjunction true
		{Role: "prod", DB: "on", Extra: "off"},  // both conjunction and disjunction false
	}

	for _, cfg := range distinguishing {
		err := Validate(&cfg)
		aRequired := err != nil && strings.Contains(err.Error(), "field token_a: is required when role!=prod&&db==on||extra==on")
		bRequired := err != nil && strings.Contains(err.Error(), "field token_b: is required when (role!=prod&&db==on)||extra==on")

		wantRequired := (cfg.Role != "prod" && cfg.DB == "on") || cfg.Extra == "on"
		if aRequired != wantRequired || bRequired != wantRequired {
			t.Fatalf("precedence mismatch for role=%q db=%q extra=%q: token_a=%v token_b=%v, want %v (err: %v)",
				cfg.Role, cfg.DB, cfg.Extra, aRequired, bRequired, wantRequired, err)
		}
	}

	// Full truth table: the bare and the parenthesized forms always agree.
	for _, role := range []string{"admin", "prod"} {
		for _, db := range []string{"on", "off"} {
			for _, extra := range []string{"on", "off"} {
				cfg := precedenceConfig{Role: role, DB: db, Extra: extra}
				err := Validate(&cfg)
				errText := ""
				if err != nil {
					errText = err.Error()
				}

				aRequired := strings.Contains(errText, "field token_a: is required when role!=prod&&db==on||extra==on")
				bRequired := strings.Contains(errText, "field token_b: is required when (role!=prod&&db==on)||extra==on")
				if aRequired != bRequired {
					t.Fatalf("bare and parenthesized forms disagree for role=%q db=%q extra=%q: token_a=%v token_b=%v (err: %s)",
						role, db, extra, aRequired, bRequired, errText)
				}
			}
		}
	}
}

// TestRequiredIfExpr_NestedParens covers ((A||B)&&C): a group nested inside
// another group.
func TestRequiredIfExpr_NestedParens(t *testing.T) {
	type nestedConfig struct {
		Role  string `yaml:"role"`
		DB    string `yaml:"db"`
		Token string `yaml:"token" validate:"required_if=((role==admin)||role==root)&&db==on"`
	}

	adminOn := nestedConfig{Role: "admin", DB: "on"}
	err := Validate(&adminOn)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when ((role==admin)||role==root)&&db==on") {
		t.Fatalf("expected violation for the nested group condition, got: %v", err)
	}

	rootOn := nestedConfig{Role: "root", DB: "on"}
	err = Validate(&rootOn)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when ((role==admin)||role==root)&&db==on") {
		t.Fatalf("expected violation for the second OR operand, got: %v", err)
	}

	adminOff := nestedConfig{Role: "admin", DB: "off"}
	if err := Validate(&adminOff); err != nil {
		t.Fatalf("expected the false AND operand to disable the requirement, got: %v", err)
	}

	userOn := nestedConfig{Role: "user", DB: "on"}
	if err := Validate(&userOn); err != nil {
		t.Fatalf("expected both OR operands false to disable the requirement, got: %v", err)
	}
}

// TestRequiredIfExpr_QuotedOperands covers quoted operands inside an
// expression: the pattern operand '(a|b)' keeps its parentheses and the
// equality operand 'on&&off' keeps its logical operators literal.
func TestRequiredIfExpr_QuotedOperands(t *testing.T) {
	type quotedConfig struct {
		Mode  string `yaml:"mode"`
		Tag   string `yaml:"tag"`
		Token string `yaml:"token" validate:"required_if=tag~='(a|b)'&&mode=='on&&off'"`
	}

	hit := quotedConfig{Mode: "on&&off", Tag: "xay"}
	err := Validate(&hit)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when tag~='(a|b)'&&mode=='on&&off'") {
		t.Fatalf("expected violation with quoted operands, got: %v", err)
	}

	plainMode := quotedConfig{Mode: "on", Tag: "xay"}
	if err := Validate(&plainMode); err != nil {
		t.Fatalf("expected the quoted && operand to stay literal, got: %v", err)
	}

	noMatchTag := quotedConfig{Mode: "on&&off", Tag: "xzy"}
	if err := Validate(&noMatchTag); err != nil {
		t.Fatalf("expected the quoted pattern operand to stay literal, got: %v", err)
	}

	// A quoted segment is literal text: a comma inside it never splits the
	// validate tag.
	type commaConfig struct {
		Mode  string `yaml:"mode"`
		Tag   string `yaml:"tag"`
		Token string `yaml:"token" validate:"required_if=tag=='a,b'&&mode==on"`
	}

	commaHit := commaConfig{Mode: "on", Tag: "a,b"}
	err = Validate(&commaHit)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when tag=='a,b'&&mode==on") {
		t.Fatalf("expected violation with a comma inside the quoted operand, got: %v", err)
	}

	commaMiss := commaConfig{Mode: "on", Tag: "a"}
	if err := Validate(&commaMiss); err != nil {
		t.Fatalf("expected the quoted comma operand to compare literally, got: %v", err)
	}
}

// TestRequiredIfExpr_UnquotedOperandParens pins the deterministic parse of
// an unquoted parenthesized operand inside an expression: the parenthesis
// after the condition operator belongs to the operand, so
// "tag~=(a|b)&&mode==on" lexes as the atom "tag~=(a|b)" followed by "&&".
func TestRequiredIfExpr_UnquotedOperandParens(t *testing.T) {
	type operandParensConfig struct {
		Mode  string `yaml:"mode"`
		Tag   string `yaml:"tag"`
		Token string `yaml:"token" validate:"required_if=tag~=(a|b)&&mode==on"`
	}

	hit := operandParensConfig{Mode: "on", Tag: "xay"}
	err := Validate(&hit)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when tag~=(a|b)&&mode==on") {
		t.Fatalf("expected violation for the unquoted parenthesized operand, got: %v", err)
	}

	noMatch := operandParensConfig{Mode: "on", Tag: "xzy"}
	if err := Validate(&noMatch); err != nil {
		t.Fatalf("expected the operand parentheses to stay inside the pattern, got: %v", err)
	}

	modeOff := operandParensConfig{Mode: "off", Tag: "xay"}
	if err := Validate(&modeOff); err != nil {
		t.Fatalf("expected the second atom to gate the requirement, got: %v", err)
	}
}

// TestRequiredIfExpr_SpacesAroundOperators covers whitespace tolerance:
// spaces around "&&", "||" and the parentheses are insignificant and atoms
// are trimmed, while the reported expression keeps its original spacing.
func TestRequiredIfExpr_SpacesAroundOperators(t *testing.T) {
	type spacesConfig struct {
		Mode    string `yaml:"mode"`
		URL     string `yaml:"url"`
		Cert    string `yaml:"cert"`
		KeyFile string `yaml:"key_file" validate:"required_if=mode==webhook && ( url~=^https: || cert^=^$ )"`
	}

	hit := spacesConfig{Mode: "webhook", URL: "https://x"}
	err := Validate(&hit)
	if err == nil || !strings.Contains(err.Error(), "field key_file: is required when mode==webhook && ( url~=^https: || cert^=^$ )") {
		t.Fatalf("expected violation despite spaces around operators, got: %v", err)
	}

	gateFalse := spacesConfig{Mode: "pooling", URL: "https://x"}
	if err := Validate(&gateFalse); err != nil {
		t.Fatalf("expected the trimmed gate atom to compare cleanly, got: %v", err)
	}
}

// TestRequiredIfExpr_LegacyAtomsInsideExpression covers mixing deprecated
// colon atoms with canonical ones, including the "!" negation prefix and
// the empty/not_empty macros.
func TestRequiredIfExpr_LegacyAtomsInsideExpression(t *testing.T) {
	type legacyConfig struct {
		Mode   string `yaml:"mode"`
		Role   string `yaml:"role"`
		Token  string `yaml:"token"`
		Secret string `yaml:"secret" validate:"required_if=mode==webhook&&role:!observer"`
		Addr   string `yaml:"addr" validate:"required_if=mode:webhook&&token:not_empty"`
	}

	hit := legacyConfig{Mode: "webhook", Role: "admin"}
	err := Validate(&hit)
	if err == nil || !strings.Contains(err.Error(), "field secret: is required when mode==webhook&&role:!observer") {
		t.Fatalf("expected violation for the mixed canonical and legacy expression, got: %v", err)
	}

	negated := legacyConfig{Mode: "webhook", Role: "observer"}
	if err := Validate(&negated); err != nil {
		t.Fatalf("expected the legacy negation atom to be false, got: %v", err)
	}

	gateOff := legacyConfig{Mode: "pooling", Role: "admin"}
	if err := Validate(&gateOff); err != nil {
		t.Fatalf("expected the canonical gate atom to be false, got: %v", err)
	}

	macroHit := legacyConfig{Mode: "webhook", Token: "t"}
	err = Validate(&macroHit)
	if err == nil || !strings.Contains(err.Error(), "field addr: is required when mode:webhook&&token:not_empty") {
		t.Fatalf("expected violation for the legacy not_empty macro atom, got: %v", err)
	}

	// The empty token disables the not_empty atom and the observer role
	// disables the negation atom, so neither field is required.
	macroMiss := legacyConfig{Mode: "webhook", Role: "observer"}
	if err := Validate(&macroMiss); err != nil {
		t.Fatalf("expected the empty token to disable the not_empty atom, got: %v", err)
	}
}

// TestRequiredIfExpr_FormErrors covers expression shape mistakes: they are
// configuration errors reported regardless of the field value and never
// produce a required error.
func TestRequiredIfExpr_FormErrors(t *testing.T) {
	type unbalancedOpenConfig struct {
		Token string `yaml:"token" validate:"required_if=(mode==webhook&&url~=^https:"`
		Mode  string `yaml:"mode"`
		URL   string `yaml:"url"`
	}

	type unbalancedCloseConfig struct {
		Token string `yaml:"token" validate:"required_if=mode==webhook&&url~=^https:)"`
		Mode  string `yaml:"mode"`
		URL   string `yaml:"url"`
	}

	type trailingAndConfig struct {
		Token string `yaml:"token" validate:"required_if=mode==webhook&&"`
		Mode  string `yaml:"mode"`
	}

	type leadingAndConfig struct {
		Token string `yaml:"token" validate:"required_if=&&mode==webhook"`
		Mode  string `yaml:"mode"`
	}

	type doubleAndConfig struct {
		Token string `yaml:"token" validate:"required_if=mode==webhook&&&&url~=^https:"`
		Mode  string `yaml:"mode"`
		URL   string `yaml:"url"`
	}

	type doubleOrConfig struct {
		Token string `yaml:"token" validate:"required_if=mode==webhook||||url~=^https:"`
		Mode  string `yaml:"mode"`
		URL   string `yaml:"url"`
	}

	type emptyParensConfig struct {
		Token string `yaml:"token" validate:"required_if=mode==webhook&&()"`
		Mode  string `yaml:"mode"`
	}

	type emptyParensBeforeCloseConfig struct {
		Token string `yaml:"token" validate:"required_if=(mode==webhook||)"`
		Mode  string `yaml:"mode"`
	}

	// Token values drive the expression, so both a zero and a filled field
	// prove the independence of form errors from the field value.
	for _, token := range []string{"", "filled"} {
		open := unbalancedOpenConfig{Token: token, Mode: "webhook", URL: "https://x"}
		err := Validate(&open)
		if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: unbalanced parentheses") {
			t.Fatalf("expected unbalanced opening parenthesis error, got: %v", err)
		}

		closing := unbalancedCloseConfig{Token: token, Mode: "webhook", URL: "https://x"}
		err = Validate(&closing)
		if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: unbalanced parentheses") {
			t.Fatalf("expected unbalanced closing parenthesis error, got: %v", err)
		}

		trailing := trailingAndConfig{Token: token, Mode: "webhook"}
		err = Validate(&trailing)
		if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: empty condition at the end") {
			t.Fatalf("expected empty trailing atom error, got: %v", err)
		}

		if strings.Contains(err.Error(), "is required when") {
			t.Fatalf("form error must not produce a required error, got: %v", err)
		}
	}

	leading := leadingAndConfig{Mode: "webhook"}
	err := Validate(&leading)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: empty condition around "&&"`) {
		t.Fatalf("expected empty leading atom error, got: %v", err)
	}

	doubleAnd := doubleAndConfig{Mode: "webhook", URL: "https://x"}
	err = Validate(&doubleAnd)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: empty condition around "&&"`) {
		t.Fatalf("expected empty atom between the operators error, got: %v", err)
	}

	doubleOr := doubleOrConfig{Mode: "webhook", URL: "https://x"}
	err = Validate(&doubleOr)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: empty condition around "||"`) {
		t.Fatalf("expected empty atom between the OR operators error, got: %v", err)
	}

	emptyParens := emptyParensConfig{Mode: "webhook"}
	err = Validate(&emptyParens)
	if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: empty condition inside parentheses") {
		t.Fatalf("expected empty parentheses error, got: %v", err)
	}

	emptyBeforeClose := emptyParensBeforeCloseConfig{Mode: "webhook"}
	err = Validate(&emptyBeforeClose)
	if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: empty condition inside parentheses") {
		t.Fatalf("expected empty atom before the closing parenthesis error, got: %v", err)
	}
}

// TestRequiredIfExpr_ParenOnlySingleForm pins the paren-start detection: a
// value beginning with an unquoted "(" parses as an expression even without
// any "&&"/"||", so "(cond)" wraps a single condition instead of failing the
// legacy lookup for a target named "(cond", and a missing logical operator
// after the group reports the precise atom it precedes.
func TestRequiredIfExpr_ParenOnlySingleForm(t *testing.T) {
	type config struct {
		Token string `yaml:"token" validate:"required_if=(mode==on)"`
		Mode  string `yaml:"mode"`
	}

	pinned := config{Mode: "on"}
	err := Validate(&pinned)
	if err == nil || !strings.Contains(err.Error(), "is required when (mode==on)") {
		t.Fatalf("expected the parenthesized single condition to fire, got: %v", err)
	}

	filled := config{Token: "filled", Mode: "on"}
	if err := Validate(&filled); err != nil {
		t.Fatalf("expected no error for the filled field, got: %v", err)
	}

	unset := config{Mode: "off"}
	if err := Validate(&unset); err != nil {
		t.Fatalf("expected no error when the condition is false, got: %v", err)
	}

	type unbalancedConfig struct {
		Token string `yaml:"token" validate:"required_if=(mode==on"`
		Mode  string `yaml:"mode"`
	}

	unbalanced := unbalancedConfig{Mode: "on"}
	err = Validate(&unbalanced)
	if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: unbalanced parentheses") {
		t.Fatalf("expected unbalanced parentheses error for the paren-start form, got: %v", err)
	}

	type missingOperatorConfig struct {
		Token string `yaml:"token" validate:"required_if=(mode==on) flag==yes"`
		Mode  string `yaml:"mode"`
		Flag  string `yaml:"flag"`
	}

	missing := missingOperatorConfig{Mode: "on", Flag: "yes"}
	err = Validate(&missing)
	if err == nil || !strings.Contains(err.Error(), `missing comparison operator before "flag==yes"`) {
		t.Fatalf("expected the precise missing-operator error, got: %v", err)
	}
}

// TestValidate_DuplicateRequiredIf pins the duplicate-rule guard: a repeated
// required_if on one field used to overwrite the earlier value silently —
// the classic quiet-loss class. Now it is a configuration error that names
// both values with the merge hint, reported regardless of the field value;
// the same guard fires inside a path rule.
func TestValidate_DuplicateRequiredIf(t *testing.T) {
	type config struct {
		Token string `yaml:"token" validate:"required_if=mode==on&&flag==yes,required_if=role==admin"`
		Mode  string `yaml:"mode"`
		Flag  string `yaml:"flag"`
		Role  string `yaml:"role"`
	}

	for _, token := range []string{"", "filled"} {
		dup := config{Token: token}
		err := Validate(&dup)
		if err == nil || !strings.Contains(err.Error(), "invalid validator configuration: duplicate required_if rule") {
			t.Fatalf("expected duplicate required_if configuration error, got: %v", err)
		}

		if !strings.Contains(err.Error(), `merge the conditions with "&&" or "||"`) {
			t.Fatalf("expected the merge hint, got: %v", err)
		}

		if !strings.Contains(err.Error(), `"role==admin"`) || !strings.Contains(err.Error(), `"mode==on&&flag==yes"`) {
			t.Fatalf("expected both rule values to be named, got: %v", err)
		}
	}

	type innerConfig struct {
		Item string `yaml:"item"`
		Mode string `yaml:"mode"`
		Role string `yaml:"role"`
	}

	type pathConfig struct {
		Inner innerConfig `yaml:"inner" validate:"item.required_if=mode==on,item.required_if=role==admin"`
	}

	pathDup := pathConfig{}
	err := Validate(&pathDup)
	if err == nil || !strings.Contains(err.Error(), "invalid validator configuration: duplicate required_if rule") {
		t.Fatalf("expected duplicate required_if error inside a path rule, got: %v", err)
	}
}

// TestRequiredIfExpr_AtomConfigErrorsAlwaysReported covers exhaustive
// evaluation: an atom with a configuration mistake (unknown target, invalid
// regexp, unexported target) is reported no matter the value of the guarded
// field and the truth of the other atoms, and contributes false to the
// combination.
func TestRequiredIfExpr_AtomConfigErrorsAlwaysReported(t *testing.T) {
	type unknownTargetConfig struct {
		Token string `yaml:"token" validate:"required_if=mode==webhook&&nosuch==on"`
		Mode  string `yaml:"mode"`
	}

	// The first atom is true, the second is unresolvable: reported, and the
	// combination stays false.
	half := unknownTargetConfig{Mode: "webhook"}
	err := Validate(&half)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: target field "nosuch" not found`) {
		t.Fatalf("expected unknown target error, got: %v", err)
	}

	if strings.Contains(err.Error(), "is required when") {
		t.Fatalf("faulty atom must contribute false to the combination, got: %v", err)
	}

	// The first atom is false: the same configuration error is reported.
	other := unknownTargetConfig{Mode: "pooling"}
	err = Validate(&other)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: target field "nosuch" not found`) {
		t.Fatalf("expected unknown target error despite the false first atom, got: %v", err)
	}

	// The guarded field is filled: the configuration error is independent
	// of the field value.
	filled := unknownTargetConfig{Mode: "webhook", Token: "t"}
	err = Validate(&filled)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: target field "nosuch" not found`) {
		t.Fatalf("expected unknown target error despite the filled field, got: %v", err)
	}

	type badRegexpConfig struct {
		Token string `yaml:"token" validate:"required_if=mode==webhook&&tag~=["`
		Mode  string `yaml:"mode"`
		Tag   string `yaml:"tag"`
	}

	reHit := badRegexpConfig{Mode: "webhook", Tag: "x"}
	err = Validate(&reHit)
	if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: invalid regular expression") {
		t.Fatalf("expected invalid regexp error, got: %v", err)
	}

	if strings.Contains(err.Error(), "is required when") {
		t.Fatalf("faulty atom must contribute false to the combination, got: %v", err)
	}

	reMiss := badRegexpConfig{Mode: "pooling", Tag: "x"}
	err = Validate(&reMiss)
	if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: invalid regular expression") {
		t.Fatalf("expected invalid regexp error despite the false first atom, got: %v", err)
	}

	// An OR combination makes the faulty atom contribute false while the
	// true atom fires the requirement; the configuration error is reported
	// alongside the required error.
	type orWithFaultyAtomConfig struct {
		Token string `yaml:"token" validate:"required_if=nosuch==on||mode==webhook"`
		Mode  string `yaml:"mode"`
	}

	orHit := orWithFaultyAtomConfig{Mode: "webhook"}
	err = Validate(&orHit)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: target field "nosuch" not found`) {
		t.Fatalf("expected unknown target error alongside the fired requirement, got: %v", err)
	}

	if !strings.Contains(err.Error(), "field token: is required when nosuch==on||mode==webhook") {
		t.Fatalf("expected the true OR operand to fire the requirement, got: %v", err)
	}

	type unexportedTargetConfig struct {
		Token  string `yaml:"token" validate:"required_if=mode==webhook&&hidden==on"`
		Mode   string `yaml:"mode"`
		hidden string
	}

	hiddenHit := unexportedTargetConfig{Mode: "webhook", hidden: "on"}
	err = Validate(&hiddenHit)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: target field "hidden" is not exported`) {
		t.Fatalf("expected unexported target error, got: %v", err)
	}

	if strings.Contains(err.Error(), "is required when") {
		t.Fatalf("faulty atom must contribute false to the combination, got: %v", err)
	}
}

// TestRequiredIfExpr_SingleFormRegression pins the pre-expression behavior
// of every single-condition form bit-for-bit, including the "is required
// when field ..." message layout and the detection rules: parentheses and
// quoted "&&" never switch a single condition to the expression form.
func TestRequiredIfExpr_SingleFormRegression(t *testing.T) {
	type eqConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role==prod"`
	}

	eq := eqConfig{Role: "PROD"}
	err := Validate(&eq)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field role==PROD") {
		t.Fatalf("single == regression, got: %v", err)
	}

	type quotedEqConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role=='prod'"`
	}

	quotedEq := quotedEqConfig{Role: "prod"}
	err = Validate(&quotedEq)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field role==prod") {
		t.Fatalf("quoted == regression, got: %v", err)
	}

	type neqConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role!=observer"`
	}

	neq := neqConfig{Role: "admin"}
	err = Validate(&neq)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field role!=admin") {
		t.Fatalf("!= regression, got: %v", err)
	}

	type matchConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role~='^(prod|dev)$'"`
	}

	match := matchConfig{Role: "prod"}
	err = Validate(&match)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field role~=^(prod|dev)$") {
		t.Fatalf("~= regression, got: %v", err)
	}

	type antiConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role^=^prod"`
	}

	anti := antiConfig{Role: "stage"}
	err = Validate(&anti)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field role^=^prod") {
		t.Fatalf("^= regression, got: %v", err)
	}

	type legacyConfig struct {
		Mode  string `yaml:"mode"`
		Token string `yaml:"token" validate:"required_if=Mode:prod"`
	}

	legacy := legacyConfig{Mode: "prod"}
	err = Validate(&legacy)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field mode=prod") {
		t.Fatalf("legacy colon regression, got: %v", err)
	}

	type legacyNegConfig struct {
		Role   string `yaml:"role"`
		Secret string `yaml:"secret" validate:"required_if=Role:!observer"`
	}

	legacyNeg := legacyNegConfig{Role: "admin"}
	err = Validate(&legacyNeg)
	if err == nil || !strings.Contains(err.Error(), "field secret: is required when field role=admin") {
		t.Fatalf("legacy negation regression, got: %v", err)
	}

	type legacyMixedConfig struct {
		Mode  string `yaml:"mode"`
		Token string `yaml:"token" validate:"required_if=mode:secret==x"`
	}

	legacyMixed := legacyMixedConfig{Mode: "secret==x"}
	err = Validate(&legacyMixed)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field mode=secret==x") {
		t.Fatalf("legacy operand with embedded == regression, got: %v", err)
	}

	type dotPathConfig struct {
		Server struct {
			Mode string `yaml:"mode"`
		} `yaml:"server"`
		Token string `yaml:"token" validate:"required_if=server.mode==prod"`
	}

	dotPath := dotPathConfig{}
	dotPath.Server.Mode = "prod"
	err = Validate(&dotPath)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field server.mode==prod") {
		t.Fatalf("dot path regression, got: %v", err)
	}

	// Missing operator: the exact configuration error text is preserved.
	type noOperatorConfig struct {
		Token string `yaml:"token" validate:"required_if=nooperator"`
	}

	noOperator := noOperatorConfig{}
	err = Validate(&noOperator)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: missing comparison operator in "nooperator" (expected ==, !=, ~=, ^= or deprecated key:value)`) {
		t.Fatalf("missing operator regression, got: %v", err)
	}

	type emptyValueConfig struct {
		Token string `yaml:"token" validate:"required_if="`
	}

	emptyValue := emptyValueConfig{}
	err = Validate(&emptyValue)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: missing comparison operator in ""`) {
		t.Fatalf("empty required_if regression, got: %v", err)
	}

	type unknownTargetConfig struct {
		Token string `yaml:"token" validate:"required_if=nonexistent:prod"`
	}

	unknown := unknownTargetConfig{}
	err = Validate(&unknown)
	if err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: target field "nonexistent" not found`) {
		t.Fatalf("unknown target regression, got: %v", err)
	}

	// Invalid regexp is reported for a zero and a filled field alike.
	type badRegexpConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role~=["`
	}

	badZero := badRegexpConfig{}
	err = Validate(&badZero)
	if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: invalid regular expression") {
		t.Fatalf("invalid regexp regression, got: %v", err)
	}

	badFilled := badRegexpConfig{Token: "t"}
	err = Validate(&badFilled)
	if err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: invalid regular expression") {
		t.Fatalf("invalid regexp regression on a filled field, got: %v", err)
	}

	// Parentheses in a pattern never switch a single condition to the
	// expression form: "role~=(a|b)" keeps the legacy message.
	type parensPatternConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role~=(a|b)"`
	}

	parensHit := parensPatternConfig{Role: "xa"}
	err = Validate(&parensHit)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field role~=(a|b)") {
		t.Fatalf("parenthesized single pattern regression, got: %v", err)
	}

	parensMiss := parensPatternConfig{Role: "xz"}
	if err := Validate(&parensMiss); err != nil {
		t.Fatalf("parenthesized single pattern must not fire without a match, got: %v", err)
	}

	// "&&" inside quotes never switches a single condition to the
	// expression form.
	type quotedAndConfig struct {
		Mode  string `yaml:"mode"`
		Token string `yaml:"token" validate:"required_if=mode=='a&&b'"`
	}

	quotedAnd := quotedAndConfig{Mode: "a&&b"}
	err = Validate(&quotedAnd)
	if err == nil || !strings.Contains(err.Error(), "field token: is required when field mode==a&&b") {
		t.Fatalf("quoted && single condition regression, got: %v", err)
	}
}
