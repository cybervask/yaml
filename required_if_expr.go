package yaml

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Boolean expression support for the required_if rule. Besides the legacy
// single-condition form ("field==value") the rule value may combine several
// condition atoms with "&&" (logical AND) and "||" (logical OR), grouped
// with parentheses. The grammar follows C/Go precedence:
//
//	orExpr  := andExpr ('||' andExpr)*
//	andExpr := atom ('&&' atom)*
//	atom    := '(' orExpr ')' | condition
//
// The expression form is recognized only when the rule value contains "&&"
// or "||" outside quoted segments, or starts with an unquoted "(" — a
// grouping parenthesis in the atom-start position; a parenthesis anywhere
// else deliberately does not take part in the detection, so single
// conditions with patterns like "~=(a|b)" keep their legacy parsing. Every
// atom is evaluated — the combination never short-circuits — so
// configuration mistakes surface regardless of the combined result, and an
// atom with a configuration error contributes false.

// requiredIfTokenKind enumerates the lexeme classes of a required_if
// boolean expression.
type requiredIfTokenKind int

const (
	requiredIfTokenCondition requiredIfTokenKind = iota
	requiredIfTokenAnd
	requiredIfTokenOr
	requiredIfTokenLParen
	requiredIfTokenRParen
	requiredIfTokenEnd
)

// requiredIfToken is a single lexeme of a required_if boolean expression.
// Condition tokens keep their raw text (quotes included); atom evaluation
// strips the quotes later, exactly like the single-condition form.
type requiredIfToken struct {
	kind requiredIfTokenKind
	text string
}

// requiredIfExprNode is a parsed node of a required_if boolean expression.
type requiredIfExprNode interface {
	// eval computes the node value. Evaluation is exhaustive: every atom is
	// evaluated even when the combined result is already decided, so
	// configuration mistakes are reported regardless of the combination,
	// and a faulty atom contributes false.
	eval(root, scope reflect.Value, currentPath string, addRuleConfigErr func(format string, args ...any)) bool
}

// requiredIfOrNode combines its operands with logical OR.
type requiredIfOrNode struct {
	parts []requiredIfExprNode
}

// requiredIfAndNode combines its operands with logical AND.
type requiredIfAndNode struct {
	parts []requiredIfExprNode
}

// requiredIfAtomNode is a single condition atom in the legacy form.
type requiredIfAtomNode struct {
	cond string
}

// isRequiredIfExpression reports whether the required_if rule value uses the
// boolean expression form: it contains "&&" or "||" outside quoted segments,
// or starts with an unquoted "(" after optional leading whitespace — a
// grouping parenthesis in the atom-start position, so "(cond)" wraps a
// single condition and "(cond) other" is a missing-operator mistake instead
// of a malformed "target field "(mode"" lookup. A valid condition target
// never starts with an opening parenthesis, so the second trigger cannot
// change the outcome of a legacy single-condition value. Single '&' and '|'
// characters are not operators, and a parenthesis after a condition operator
// belongs to the operand, so patterns like "(a|b)" never trigger the
// expression form by themselves.
func isRequiredIfExpression(expr string) bool {
	inQuotes := false
	quoteChar := byte(0)
	started := false

	for i := 0; i < len(expr); i++ {
		ch := expr[i]

		if inQuotes {
			if ch == quoteChar {
				inQuotes = false
				quoteChar = 0
			}

			continue
		}

		switch {
		case ch == '\'' || ch == '"':
			inQuotes = true
			quoteChar = ch
			started = true
		case ch == '&' && i+1 < len(expr) && expr[i+1] == '&':
			return true
		case ch == '|' && i+1 < len(expr) && expr[i+1] == '|':
			return true
		case ch == '(' && !started:
			return true
		case ch == ' ' || ch == '\t':
			// Leading whitespace does not decide the form; the tokenizer
			// trims atoms anyway and a following "(" must still group.
		default:
			started = true
		}
	}

	return false
}

// lexRequiredIfExpression splits a required_if boolean expression into
// condition atoms, logical operators and grouping parentheses.
//
// Quoted segments ('...' or "...") are literal: logical operators and
// parentheses inside them never split or group. Outside quotes, "&&" and
// "||" always separate atoms — an operand carrying either must be quoted
// (e.g. "webhook.url~='(a|b)'"). A parenthesis opens a group only at an
// atom start (beginning of the expression, after a logical operator or an
// opening group); after a condition operator it belongs to the operand, so
// "x~=(a|b)&&y" lexes as the atom "x~=(a|b)" followed by "&&" and "y".
// Unbalanced parentheses — including an operand parenthesis left open
// across an operator boundary — are reported as an error.
func lexRequiredIfExpression(expr string) ([]requiredIfToken, error) {
	var tokens []requiredIfToken
	var cond strings.Builder

	// parenStack tracks every parenthesis opened outside quotes: true marks
	// a grouping paren opened at an atom start, false marks a literal paren
	// absorbed into the current condition operand.
	var parenStack []bool

	inQuotes := false
	quoteChar := byte(0)

	flushCondition := func() {
		text := strings.TrimSpace(cond.String())
		cond.Reset()
		if text != "" {
			tokens = append(tokens, requiredIfToken{kind: requiredIfTokenCondition, text: text})
		}
	}

	for i := 0; i < len(expr); i++ {
		ch := expr[i]

		if inQuotes {
			if ch == quoteChar {
				inQuotes = false
				quoteChar = 0
			}

			cond.WriteByte(ch)

			continue
		}

		switch {
		case ch == '\'' || ch == '"':
			inQuotes = true
			quoteChar = ch
			cond.WriteByte(ch)
		case ch == '&' && i+1 < len(expr) && expr[i+1] == '&':
			flushCondition()
			tokens = append(tokens, requiredIfToken{kind: requiredIfTokenAnd})
			i++
		case ch == '|' && i+1 < len(expr) && expr[i+1] == '|':
			flushCondition()
			tokens = append(tokens, requiredIfToken{kind: requiredIfTokenOr})
			i++
		case ch == '(':
			if cond.Len() == 0 {
				// Atom start: a grouping parenthesis.
				parenStack = append(parenStack, true)
				tokens = append(tokens, requiredIfToken{kind: requiredIfTokenLParen})

				break
			}

			// Inside a condition, past the condition operator: part of the
			// operand.
			parenStack = append(parenStack, false)
			cond.WriteByte(ch)
		case ch == ')':
			if len(parenStack) > 0 && !parenStack[len(parenStack)-1] {
				// Closes a literal operand parenthesis.
				parenStack = parenStack[:len(parenStack)-1]
				cond.WriteByte(ch)

				break
			}

			// Closes a grouping parenthesis, if any; a stray one stays a
			// token and is rejected by the parser below.
			if len(parenStack) > 0 {
				parenStack = parenStack[:len(parenStack)-1]
			}

			flushCondition()
			tokens = append(tokens, requiredIfToken{kind: requiredIfTokenRParen})
		default:
			if cond.Len() == 0 && (ch == ' ' || ch == '\t') {
				// Leading whitespace before an atom is insignificant: atoms
				// are trimmed anyway and a following '(' must still group.
				break
			}

			cond.WriteByte(ch)
		}
	}

	if len(parenStack) > 0 {
		return nil, fmt.Errorf("unbalanced parentheses in required_if expression %q", expr)
	}

	flushCondition()

	return tokens, nil
}

// requiredIfExprParser walks the token stream of a required_if boolean
// expression.
type requiredIfExprParser struct {
	tokens []requiredIfToken
	expr   string
	pos    int
}

// parseRequiredIfExpression parses the token stream into an expression tree
// with C/Go precedence: parentheses bind tighter than "&&", which binds
// tighter than "||".
func parseRequiredIfExpression(tokens []requiredIfToken, expr string) (requiredIfExprNode, error) {
	p := &requiredIfExprParser{tokens: tokens, expr: expr}
	node, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	// Only a stray ')' or a missing logical operator between atoms can be
	// left over here: "&&" chains are consumed by parseAnd, "||" chains by
	// parseOr.
	if p.pos < len(tokens) {
		tok := p.peek()
		if tok.kind == requiredIfTokenRParen {
			return nil, fmt.Errorf("unbalanced parentheses in required_if expression %q", expr)
		}

		quoted := tok.text
		if tok.kind == requiredIfTokenLParen {
			quoted = "("
		}

		return nil, fmt.Errorf("missing comparison operator before %q in required_if expression %q", quoted, expr)
	}

	return node, nil
}

// parseOr parses orExpr := andExpr ('||' andExpr)*.
func (p *requiredIfExprParser) parseOr() (requiredIfExprNode, error) {
	first, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	if p.peek().kind != requiredIfTokenOr {
		return first, nil
	}

	parts := []requiredIfExprNode{first}
	for p.peek().kind == requiredIfTokenOr {
		p.pos++

		next, err := p.parseAnd()
		if err != nil {
			return nil, err
		}

		parts = append(parts, next)
	}

	return &requiredIfOrNode{parts: parts}, nil
}

// parseAnd parses andExpr := atom ('&&' atom)*.
func (p *requiredIfExprParser) parseAnd() (requiredIfExprNode, error) {
	first, err := p.parseAtom()
	if err != nil {
		return nil, err
	}

	if p.peek().kind != requiredIfTokenAnd {
		return first, nil
	}

	parts := []requiredIfExprNode{first}
	for p.peek().kind == requiredIfTokenAnd {
		p.pos++

		next, err := p.parseAtom()
		if err != nil {
			return nil, err
		}

		parts = append(parts, next)
	}

	return &requiredIfAndNode{parts: parts}, nil
}

// parseAtom parses atom := '(' orExpr ')' | condition.
func (p *requiredIfExprParser) parseAtom() (requiredIfExprNode, error) {
	tok := p.peek()

	switch tok.kind {
	case requiredIfTokenLParen:
		p.pos++
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}

		if p.peek().kind != requiredIfTokenRParen {
			return nil, fmt.Errorf("unbalanced parentheses in required_if expression %q", p.expr)
		}

		p.pos++

		return inner, nil
	case requiredIfTokenCondition:
		p.pos++

		return &requiredIfAtomNode{cond: tok.text}, nil
	case requiredIfTokenRParen:
		return nil, fmt.Errorf("empty condition inside parentheses in required_if expression %q", p.expr)
	case requiredIfTokenAnd, requiredIfTokenOr:
		return nil, fmt.Errorf("empty condition around %q in required_if expression %q", requiredIfOperatorText(tok.kind), p.expr)
	default: // requiredIfTokenEnd
		return nil, fmt.Errorf("empty condition at the end of required_if expression %q", p.expr)
	}
}

// peek returns the upcoming token, or an end-of-input sentinel.
func (p *requiredIfExprParser) peek() requiredIfToken {
	if p.pos >= len(p.tokens) {
		return requiredIfToken{kind: requiredIfTokenEnd}
	}

	return p.tokens[p.pos]
}

// requiredIfOperatorText renders a logical token for error messages.
func requiredIfOperatorText(kind requiredIfTokenKind) string {
	switch kind {
	case requiredIfTokenAnd:
		return "&&"
	case requiredIfTokenOr:
		return "||"
	default:
		panic(fmt.Sprintf("yaml: requiredIfOperatorText: unexpected token kind %d", kind))
	}
}

// eval returns the logical OR of its operands; every operand is evaluated.
func (n *requiredIfOrNode) eval(root, scope reflect.Value, currentPath string, addRuleConfigErr func(format string, args ...any)) bool {
	result := false
	for _, part := range n.parts {
		if part.eval(root, scope, currentPath, addRuleConfigErr) {
			result = true
		}
	}

	return result
}

// eval returns the logical AND of its operands; every operand is evaluated.
func (n *requiredIfAndNode) eval(root, scope reflect.Value, currentPath string, addRuleConfigErr func(format string, args ...any)) bool {
	result := true
	for _, part := range n.parts {
		if !part.eval(root, scope, currentPath, addRuleConfigErr) {
			result = false
		}
	}

	return result
}

// eval parses and evaluates a single condition atom. A condition without a
// comparison operator is the same configuration-mistake class as in the
// single-condition form and contributes false.
func (n *requiredIfAtomNode) eval(root, scope reflect.Value, currentPath string, addRuleConfigErr func(format string, args ...any)) bool {
	cond, ok := parseRequiredIfCondition(n.cond)
	if !ok {
		addRuleConfigErr("field %s: invalid required_if configuration: missing comparison operator in %q (expected ==, !=, ~=, ^= or deprecated key:value)", currentPath, n.cond)

		return false
	}

	return evalRequiredIfCondition(cond, root, scope, currentPath, addRuleConfigErr).met
}

// requiredIfEval carries the evaluation result of one required_if condition
// together with the message fragments used to report a violated requirement:
// the resolved target name, the display operator ("=" for the deprecated
// colon form) and the reported operand (the actual target value for the
// equality operators, the pattern for the regexp ones).
type requiredIfEval struct {
	met       bool
	yamlName  string
	displayOp string
	operand   string
}

// evalRequiredIfCondition evaluates a single required_if condition against
// the configuration tree. Configuration mistakes (unknown target, unexported
// target, invalid regexp) are reported through addRuleConfigErr and evaluate
// to false, so a faulty atom takes part in a boolean combination as false;
// configuration errors never depend on the validated field value.
func evalRequiredIfCondition(cond requiredIfCondition, root, scope reflect.Value, currentPath string, addRuleConfigErr func(format string, args ...any)) requiredIfEval {
	targetFieldName := cond.target

	// Quote stripping applies to canonical operands only: the
	// deprecated colon form keeps its raw comparison payload.
	expectedValue := cond.value
	if cond.op != ":" {
		expectedValue = stripRuleQuotes(expectedValue)
	}

	// Resolve the condition target: dot paths address the root
	// struct, bare names match the enclosing struct first and
	// fall back to the root so that top-level targets keep
	// working for nested owners.
	var targetField reflect.Value
	yamlTargetName := targetFieldName

	if strings.Contains(targetFieldName, ".") {
		if resolved, found := resolvePathTarget(root, targetFieldName); found {
			targetField = resolved
			yamlTargetName = targetFieldName
		}
	} else if resolved, resolvedName := findStructTarget(scope, targetFieldName); resolved.IsValid() {
		targetField = resolved
		yamlTargetName = resolvedName
	} else if !sameStructValue(scope, root) {
		if resolved, resolvedName := findStructTarget(root, targetFieldName); resolved.IsValid() {
			targetField = resolved
			yamlTargetName = resolvedName
		}
	}

	switch {
	case targetField.IsValid() && !targetField.CanInterface():
		// The name matched an unexported field: its value cannot
		// be read back through reflect (Interface would panic), so
		// the condition is unresolvable by configuration — report
		// it like the other required_if configuration mistakes.
		addRuleConfigErr("field %s: invalid required_if configuration: target field %q is not exported", currentPath, targetFieldName)
	case targetField.IsValid():
		// Pointer targets are unwrapped before formatting: a non-nil
		// pointer compares by its element value, a nil one formats as
		// the empty string (Go's "<nil>" placeholder could never match
		// a rule token). The empty/not_empty macros below stay on the
		// original targetField IsZero semantics.
		formatTarget := targetField
		if formatTarget.Kind() == reflect.Pointer {
			if formatTarget.IsNil() {
				formatTarget = reflect.ValueOf("")
			} else {
				formatTarget = formatTarget.Elem()
			}
		}

		actualValueStr := fmt.Sprintf("%v", formatTarget.Interface())
		if formatTarget.Kind() == reflect.String {
			actualValueStr = formatTarget.String()
		}

		conditionMet := false

		switch cond.op {
		case "==", "!=":
			// Case-insensitive comparison: YAML values arrive in any
			// casing ("Client" must satisfy a role==client rule).
			matched := strings.EqualFold(actualValueStr, expectedValue)
			conditionMet = matched
			if cond.op == "!=" {
				conditionMet = !matched
			}
		case "~=", "^=":
			re, reErr := regexp.Compile(expectedValue)
			if reErr != nil {
				// Configuration errors must not depend on the field
				// value, same as the unknown-target error below.
				addRuleConfigErr("field %s: invalid required_if configuration: invalid regular expression %q: %w", currentPath, expectedValue, reErr)

				break
			}

			matched := re.MatchString(actualValueStr)
			conditionMet = matched
			if cond.op == "^=" {
				conditionMet = !matched
			}
		case ":":
			// Deprecated colon form: kept as-is for backward
			// compatibility, including the empty/not_empty macros
			// and the "!" negation prefix.
			isNegation := strings.HasPrefix(expectedValue, "!")
			cleanExpectedValue := strings.TrimPrefix(expectedValue, "!")

			switch cleanExpectedValue {
			case "empty":
				conditionMet = targetField.IsZero()
			case "not_empty":
				conditionMet = !targetField.IsZero()
			default:
				// Case-insensitive comparison: YAML values arrive
				// in any casing ("Client" must satisfy a
				// role:client rule).
				conditionMet = strings.EqualFold(actualValueStr, cleanExpectedValue)
			}

			if isNegation {
				conditionMet = !conditionMet
			}
		}

		// The reported operand is the actual target value for
		// equality operators and the pattern for regex ones.
		operand := actualValueStr
		displayOp := cond.op
		if cond.op == ":" {
			displayOp = "="
		} else if cond.op == "~=" || cond.op == "^=" {
			operand = expectedValue
		}

		return requiredIfEval{met: conditionMet, yamlName: yamlTargetName, displayOp: displayOp, operand: operand}
	default:
		addRuleConfigErr("field %s: invalid required_if configuration: target field %q not found", currentPath, targetFieldName)
	}

	return requiredIfEval{}
}

// evaluateRequiredIf runs the required_if rule for the validated field v.
// The legacy single-condition form keeps its exact pre-expression behavior;
// a rule value carrying "&&" or "||" outside quotes, or starting with an
// unquoted "(", is evaluated as a boolean expression instead. A met
// condition on a zero field appends the required error to errs;
// configuration mistakes go through addRuleConfigErr.
func evaluateRequiredIf(reqIfStr string, v reflect.Value, currentPath string, root, scope reflect.Value, addRuleConfigErr func(format string, args ...any), errs *[]error) {
	if !isRequiredIfExpression(reqIfStr) {
		evaluateRequiredIfSingleCondition(reqIfStr, v, currentPath, root, scope, addRuleConfigErr, errs)

		return
	}

	evaluateRequiredIfExpression(reqIfStr, v, currentPath, root, scope, addRuleConfigErr, errs)
}

// evaluateRequiredIfSingleCondition evaluates the legacy single-condition
// required_if form. It reproduces the pre-expression behavior bit-for-bit,
// including the "is required when field ..." message layout.
func evaluateRequiredIfSingleCondition(reqIfStr string, v reflect.Value, currentPath string, root, scope reflect.Value, addRuleConfigErr func(format string, args ...any), errs *[]error) {
	cond, ok := parseRequiredIfCondition(reqIfStr)
	if !ok {
		// Malformed required_if: no operator at all. Report a
		// configuration error regardless of the field value, same as
		// the unknown-target and invalid-regexp siblings above; an
		// empty value is the same malformed class.
		addRuleConfigErr("field %s: invalid required_if configuration: missing comparison operator in %q (expected ==, !=, ~=, ^= or deprecated key:value)", currentPath, reqIfStr)

		return
	}

	res := evalRequiredIfCondition(cond, root, scope, currentPath, addRuleConfigErr)
	if res.met && v.IsZero() {
		*errs = append(*errs, fmt.Errorf("field %s: is required when field %s%s%s", currentPath, res.yamlName, res.displayOp, res.operand))
	}
}

// evaluateRequiredIfExpression lexes, parses and evaluates the boolean
// expression form of required_if. Form errors (unbalanced parentheses,
// empty atoms) are configuration mistakes reported regardless of the field
// value; atom configuration mistakes are reported during evaluation and
// contribute false to the combination. Because evaluation is exhaustive, a
// repeated faulty atom (e.g. "nosuch==1&&nosuch==2") would surface the same
// mistake twice — identical reports (same format and arguments) are emitted
// once per expression, with the original formatting untouched.
func evaluateRequiredIfExpression(reqIfStr string, v reflect.Value, currentPath string, root, scope reflect.Value, addRuleConfigErr func(format string, args ...any), errs *[]error) {
	tokens, lexErr := lexRequiredIfExpression(reqIfStr)
	if lexErr != nil {
		addRuleConfigErr("field %s: invalid required_if configuration: %v", currentPath, lexErr)

		return
	}

	node, parseErr := parseRequiredIfExpression(tokens, reqIfStr)
	if parseErr != nil {
		addRuleConfigErr("field %s: invalid required_if configuration: %v", currentPath, parseErr)

		return
	}

	reported := make(map[string]struct{})
	reportOnce := func(format string, args ...any) {
		key := format + "\x00" + fmt.Sprint(args...)
		if _, dup := reported[key]; dup {
			return
		}

		reported[key] = struct{}{}
		addRuleConfigErr(format, args...)
	}

	if node.eval(root, scope, currentPath, reportOnce) && v.IsZero() {
		*errs = append(*errs, fmt.Errorf("field %s: is required when %s", currentPath, reqIfStr))
	}
}
