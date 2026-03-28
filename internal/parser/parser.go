// Package parser provides a simple regex/string-based TSX parser that converts
// a TSX source string into an internal AST representation.
package parser

import (
	"regexp"
	"strings"

	"github.com/linkalls/fluttex/internal/ast"
)

var (
	// funcDeclRe matches the start of a function component declaration.
	funcDeclRe = regexp.MustCompile(`(?:export\s+(?:default\s+)?)?function\s+([A-Z][A-Za-z0-9_]*)\s*\([^)]*\)\s*`)

	// arrowDeclRe matches the start of an arrow function component declaration.
	arrowDeclRe = regexp.MustCompile(`(?:export\s+(?:default\s+)?)?const\s+([A-Z][A-Za-z0-9_]*)\s*(?::[^=]+)?=\s*(?:\([^)]*\)|[A-Za-z_]\w*)(?:\s*:[^=>{]+)?\s*=>\s*`)

	// useStateRe matches: const [name, setter] = useState(initial)
	useStateRe = regexp.MustCompile(`const\s+\[(\w+),\s*(\w+)\]\s*=\s*useState\(([^)]*)\)`)

	// useEffectRe matches useEffect(() => { body }, [deps])
	useEffectRe = regexp.MustCompile(`(?s)useEffect\(\s*\(\s*\)\s*=>\s*\{(.*?)\}\s*(?:,\s*(\[[^\]]*\]))?\s*\)`)

	// returnJSXRe extracts the JSX block from a return statement
	returnJSXRe = regexp.MustCompile(`(?s)return\s*\(\s*(<.+?>.*?)\s*\)\s*;?\s*$`)

	// propRe matches key="value" or key='value' — complex {value} handled separately
	propSQRe = regexp.MustCompile(`([\w-]+)=(?:"([^"]*)"|'([^']*)')`)

	// tagNameRe matches a tag name at the start of a string
	tagNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]*`)

	// depsRe parses deps array word tokens
	depsRe = regexp.MustCompile(`\w+`)

	// cleanupRe extracts cleanup function from useEffect body
	cleanupRe = regexp.MustCompile(`(?s)return\s*\(\s*\)\s*=>\s*\{(.*?)\}`)
)

// ParseFile parses a TSX source string and returns a slice of top-level Component nodes.
func ParseFile(src string) []*ast.Node {
	src = strings.TrimSpace(src)
	var components []*ast.Node
	seen := map[string]bool{}

	// Find function components using brace-counting approach
	funcLocs := funcDeclRe.FindAllStringSubmatchIndex(src, -1)
	for _, loc := range funcLocs {
		name := src[loc[2]:loc[3]]
		if seen[name] {
			continue
		}
		// The function body starts at the '{' right after the match
		bodyOpenIdx := loc[1]
		if bodyOpenIdx >= len(src) || src[bodyOpenIdx] != '{' {
			// Try finding the next '{'
			for bodyOpenIdx < len(src) && src[bodyOpenIdx] != '{' {
				bodyOpenIdx++
			}
		}
		body, _ := extractBraceBody(src, bodyOpenIdx)
		comp := parseComponent(name, body)
		seen[name] = true
		components = append(components, comp)
	}

	// Find arrow function components
	arrowLocs := arrowDeclRe.FindAllStringSubmatchIndex(src, -1)
	for _, loc := range arrowLocs {
		name := src[loc[2]:loc[3]]
		if seen[name] {
			continue
		}
		afterArrow := strings.TrimSpace(src[loc[1]:])
		comp := parseArrowBody(name, afterArrow)
		if comp != nil {
			seen[name] = true
			components = append(components, comp)
		}
	}

	// Fallback: try whole source as function component body
	if len(components) == 0 {
		comp := parseComponent("App", src)
		if comp.Body != nil {
			return []*ast.Node{comp}
		}
		node := parseJSX(strings.TrimSpace(src))
		if node != nil {
			components = append(components, &ast.Node{
				Type: ast.NodeComponent,
				Name: "App",
				Body: node,
			})
		}
	}

	return components
}

// extractBraceBody extracts the content between matching '{' '}' starting at index start.
// Returns (body, indexAfterClosingBrace).
func extractBraceBody(src string, start int) (string, int) {
	if start >= len(src) || src[start] != '{' {
		return "", start
	}
	depth := 0
	inStr := false
	strChar := byte(0)
	for i := start; i < len(src); i++ {
		c := src[i]
		if inStr {
			if c == strChar && !(i > 0 && src[i-1] == '\\') {
				inStr = false
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			inStr = true
			strChar = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start+1 : i], i + 1
			}
		}
	}
	return src[start+1:], len(src)
}

// parseArrowBody parses an arrow function component body starting right after "=>".
func parseArrowBody(name, afterArrow string) *ast.Node {
	afterArrow = strings.TrimSpace(afterArrow)

	// Block body: { ... }
	if strings.HasPrefix(afterArrow, "{") {
		body, _ := extractBraceBody(afterArrow, 0)
		return parseComponent(name, body)
	}

	// Parenthesized JSX: (...)
	if strings.HasPrefix(afterArrow, "(") {
		// Find matching )
		inner, _ := extractParenBody(afterArrow, 0)
		inner = strings.TrimSpace(inner)
		node := parseJSX(inner)
		if node == nil {
			return nil
		}
		return &ast.Node{Type: ast.NodeComponent, Name: name, Body: node}
	}

	// Direct JSX: <Tag ...>...</Tag> or <Tag />
	if strings.HasPrefix(afterArrow, "<") {
		node := parseJSX(afterArrow)
		if node == nil {
			return nil
		}
		return &ast.Node{Type: ast.NodeComponent, Name: name, Body: node}
	}

	return nil
}

// extractParenBody extracts the content between matching '(' ')' starting at index start.
func extractParenBody(src string, start int) (string, int) {
	if start >= len(src) || src[start] != '(' {
		return "", start
	}
	depth := 0
	inStr := false
	strChar := byte(0)
	for i := start; i < len(src); i++ {
		c := src[i]
		if inStr {
			if c == strChar && !(i > 0 && src[i-1] == '\\') {
				inStr = false
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			inStr = true
			strChar = c
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return src[start+1 : i], i + 1
			}
		}
	}
	return src[start+1:], len(src)
}

func parseComponent(name, body string) *ast.Node {
	comp := &ast.Node{
		Type: ast.NodeComponent,
		Name: name,
	}

	// Extract useState calls
	for _, m := range useStateRe.FindAllStringSubmatch(body, -1) {
		comp.StateVars = append(comp.StateVars, ast.StateVar{
			Name:    m[1],
			Setter:  m[2],
			Initial: strings.TrimSpace(m[3]),
		})
	}

	// Extract useEffect calls
	for _, m := range useEffectRe.FindAllStringSubmatch(body, -1) {
		effectBody := strings.TrimSpace(m[1])
		effect := ast.Effect{Body: effectBody}

		if cm := cleanupRe.FindStringSubmatch(effectBody); cm != nil {
			effect.Cleanup = strings.TrimSpace(cm[1])
			effect.Body = strings.TrimSpace(cleanupRe.ReplaceAllString(effectBody, ""))
		}

		if m[2] != "" {
			effect.HasDeps = true
			depsStr := strings.Trim(m[2], "[] \t\n")
			if depsStr != "" {
				for _, d := range depsRe.FindAllString(depsStr, -1) {
					effect.Deps = append(effect.Deps, d)
				}
			}
		}
		comp.Effects = append(comp.Effects, effect)
	}

	// Extract returned JSX — find "return (" and extract the JSX inside
	jsxStr := extractReturnJSX(body)
	if jsxStr != "" {
		comp.Body = parseJSX(strings.TrimSpace(jsxStr))
	}

	return comp
}

// extractReturnJSX finds the return statement and extracts the JSX content.
func extractReturnJSX(body string) string {
	// Look for "return (" pattern
	returnParenRe := regexp.MustCompile(`(?s)return\s*\(`)
	loc := returnParenRe.FindStringIndex(body)
	if loc != nil {
		// Find the matching paren
		parenStart := loc[1] - 1 // index of '('
		inner, _ := extractParenBody(body, parenStart)
		return strings.TrimSpace(inner)
	}

	// Look for "return <" pattern (direct JSX return without parens)
	returnDirectRe := regexp.MustCompile(`(?s)return\s+(<[A-Za-z])`)
	loc2 := returnDirectRe.FindStringIndex(body)
	if loc2 != nil {
		// Find where JSX starts (at the '<')
		jsxStart := loc2[1] - 1
		// Extract to end of tag tree
		return strings.TrimSpace(body[jsxStart:])
	}

	// Fallback: use returnJSXRe
	if m := returnJSXRe.FindStringSubmatch(body); m != nil {
		return m[1]
	}

	return ""
}

// parseJSX parses a JSX string fragment into an AST node.
func parseJSX(s string) *ast.Node {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// JSX Expression: {expr}
	if strings.HasPrefix(s, "{") {
		end := findMatchingBrace(s, 0)
		if end == len(s)-1 {
			return parseJSXExpression(s[1 : len(s)-1])
		}
	}

	// Determine if self-closing or opening tag using character scan
	if strings.HasPrefix(s, "<") {
		tag, props, afterTag, selfClosing := scanOpeningTag(s)
		if tag == "" {
			return nil
		}
		if selfClosing {
			return &ast.Node{
				Type:  ast.NodeJSXElement,
				Tag:   tag,
				Props: props,
			}
		}
		// Opening tag
		inner, _ := extractInner(tag, afterTag)
		children := parseChildren(inner)
		return &ast.Node{
			Type:     ast.NodeJSXElement,
			Tag:      tag,
			Props:    props,
			Children: children,
		}
	}

	// Plain text
	if !strings.HasPrefix(s, "<") {
		return &ast.Node{Type: ast.NodeJSXText, Text: s}
	}

	return nil
}

// scanOpeningTag parses an opening (or self-closing) JSX tag.
// It handles {expr} attribute values that may contain '>'.
// Returns (tagName, props, textAfterTag, isSelfClosing).
func scanOpeningTag(s string) (string, map[string]string, string, bool) {
	if len(s) == 0 || s[0] != '<' {
		return "", nil, s, false
	}

	i := 1 // skip '<'

	// Tag name
	nameStart := i
	for i < len(s) && isTagNameByte(s[i]) {
		i++
	}
	if i == nameStart {
		return "", nil, s, false
	}
	tag := s[nameStart:i]

	// Scan attributes
	attrBuf := strings.Builder{}
	for i < len(s) {
		c := s[i]
		switch {
		case c == '{':
			// Read the entire {expr} including nested braces
			end := findMatchingBrace(s, i)
			if end < 0 {
				end = len(s) - 1
			}
			attrBuf.WriteString(s[i : end+1])
			i = end + 1
		case c == '"':
			// Quoted attribute value
			j := i + 1
			for j < len(s) && s[j] != '"' {
				j++
			}
			attrBuf.WriteString(s[i : j+1])
			i = j + 1
		case c == '\'':
			j := i + 1
			for j < len(s) && s[j] != '\'' {
				j++
			}
			attrBuf.WriteString(s[i : j+1])
			i = j + 1
		case c == '/' && i+1 < len(s) && s[i+1] == '>':
			// Self-closing
			afterTag := ""
			if i+2 < len(s) {
				afterTag = s[i+2:]
			}
			return tag, parsePropsStr(attrBuf.String()), afterTag, true
		case c == '>':
			// End of opening tag
			afterTag := ""
			if i+1 < len(s) {
				afterTag = s[i+1:]
			}
			return tag, parsePropsStr(attrBuf.String()), afterTag, false
		default:
			attrBuf.WriteByte(c)
			i++
		}
	}
	return tag, parsePropsStr(attrBuf.String()), "", false
}

func isTagNameByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.'
}

// parsePropsStr parses an attribute string into a map, handling {expr} values.
func parsePropsStr(attrs string) map[string]string {
	props := map[string]string{}
	attrs = strings.TrimSpace(attrs)
	if attrs == "" {
		return props
	}

	i := 0
	for i < len(attrs) {
		// Skip whitespace
		for i < len(attrs) && isWhitespace(attrs[i]) {
			i++
		}
		if i >= len(attrs) {
			break
		}

		// Read key
		keyStart := i
		for i < len(attrs) && attrs[i] != '=' && !isWhitespace(attrs[i]) {
			i++
		}
		if i == keyStart {
			i++
			continue
		}
		key := attrs[keyStart:i]

		// Skip whitespace
		for i < len(attrs) && isWhitespace(attrs[i]) {
			i++
		}

		if i >= len(attrs) || attrs[i] != '=' {
			// Boolean attribute
			props[key] = ""
			continue
		}
		i++ // skip '='

		// Skip whitespace
		for i < len(attrs) && isWhitespace(attrs[i]) {
			i++
		}

		if i >= len(attrs) {
			props[key] = ""
			break
		}

		var val string
		switch attrs[i] {
		case '"':
			// "value"
			j := i + 1
			for j < len(attrs) && attrs[j] != '"' {
				j++
			}
			val = attrs[i+1 : j]
			i = j + 1
		case '\'':
			// 'value'
			j := i + 1
			for j < len(attrs) && attrs[j] != '\'' {
				j++
			}
			val = attrs[i+1 : j]
			i = j + 1
		case '{':
			// {expr}
			end := findMatchingBrace(attrs, i)
			if end < 0 {
				end = len(attrs) - 1
			}
			val = attrs[i+1 : end]
			i = end + 1
		default:
			// Unquoted value (rare in JSX but handle gracefully)
			j := i
			for j < len(attrs) && !isWhitespace(attrs[j]) {
				j++
			}
			val = attrs[i:j]
			i = j
		}
		props[key] = val
	}
	return props
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// parseJSXExpression parses the inner content of a {expr} node.
func parseJSXExpression(expr string) *ast.Node {
	expr = strings.TrimSpace(expr)

	if node := tryParseConditional(expr); node != nil {
		return node
	}
	if node := tryParseAnd(expr); node != nil {
		return node
	}
	if node := tryParseMap(expr); node != nil {
		return node
	}

	return &ast.Node{
		Type:       ast.NodeJSXExpression,
		Expression: expr,
	}
}

// tryParseConditional attempts to parse "cond ? consequent : alternate".
func tryParseConditional(expr string) *ast.Node {
	qIdx := findTopLevel(expr, '?')
	if qIdx < 0 {
		return nil
	}
	cond := strings.TrimSpace(expr[:qIdx])
	rest := expr[qIdx+1:]

	cIdx := findTopLevel(rest, ':')
	if cIdx < 0 {
		return nil
	}
	consequentStr := strings.TrimSpace(rest[:cIdx])
	alternateStr := strings.TrimSpace(rest[cIdx+1:])

	consequent := parseJSX(consequentStr)
	alternate := parseJSX(alternateStr)

	return &ast.Node{
		Type:       ast.NodeConditional,
		Condition:  cond,
		Consequent: consequent,
		Alternate:  alternate,
	}
}

// tryParseAnd attempts to parse "cond && <jsx>".
func tryParseAnd(expr string) *ast.Node {
	idx := findTopLevelStr(expr, "&&")
	if idx < 0 {
		return nil
	}
	cond := strings.TrimSpace(expr[:idx])
	jsxStr := strings.TrimSpace(expr[idx+2:])
	consequent := parseJSX(jsxStr)
	if consequent == nil {
		return nil
	}
	return &ast.Node{
		Type:       ast.NodeConditional,
		Condition:  cond,
		Consequent: consequent,
		Alternate:  nil,
	}
}

// mapCallRe matches: expr.map((item[, idx]) => <jsx/>) or expr.map(item => <jsx/>)
var mapCallRe = regexp.MustCompile(`(?s)^(.+?)\.map\(\s*\(?(\w+)(?:\s*,\s*\w+)?\)?\s*=>\s*(.+)\s*\)$`)

// tryParseMap attempts to parse "items.map((item) => <jsx/>)".
func tryParseMap(expr string) *ast.Node {
	m := mapCallRe.FindStringSubmatch(strings.TrimSpace(expr))
	if m == nil {
		return nil
	}
	listExpr := strings.TrimSpace(m[1])
	itemVar := m[2]
	jsxStr := strings.TrimSpace(m[3])

	bodyNode := parseJSX(jsxStr)
	if bodyNode == nil {
		return nil
	}

	return &ast.Node{
		Type:     ast.NodeListRender,
		ListExpr: listExpr,
		ListItem: itemVar,
		ListBody: bodyNode,
	}
}

// findTopLevel finds the index of byte b at the top level (not inside brackets/parens/strings).
func findTopLevel(s string, b byte) int {
	depth := 0
	inStr := false
	strChar := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if c == strChar && !(i > 0 && s[i-1] == '\\') {
				inStr = false
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			inStr = true
			strChar = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		default:
			if depth == 0 && c == b {
				return i
			}
		}
	}
	return -1
}

// findTopLevelStr finds the index of substr at the top level.
func findTopLevelStr(s, substr string) int {
	depth := 0
	inStr := false
	strChar := byte(0)
	for i := 0; i <= len(s)-len(substr); i++ {
		c := s[i]
		if inStr {
			if c == strChar && !(i > 0 && s[i-1] == '\\') {
				inStr = false
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			inStr = true
			strChar = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		default:
			if depth == 0 && s[i:i+len(substr)] == substr {
				return i
			}
		}
	}
	return -1
}

// extractInner returns the inner content between <tag> and </tag>,
// handling nested tags of the same name.
func extractInner(tag, s string) (string, string) {
	depth := 1
	i := 0
	openPat := "<" + tag
	closePat := "</" + tag + ">"

	for i < len(s) {
		ci := strings.Index(s[i:], closePat)
		oi := strings.Index(s[i:], openPat)

		if ci == -1 {
			break
		}

		if oi != -1 && oi < ci {
			depth++
			i += oi + len(openPat)
		} else {
			depth--
			if depth == 0 {
				return s[:i+ci], s[i+ci+len(closePat):]
			}
			i += ci + len(closePat)
		}
	}
	return s, ""
}

// parseChildren splits an inner JSX string into child nodes.
func parseChildren(inner string) []*ast.Node {
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return nil
	}

	var children []*ast.Node
	rest := inner

	for rest != "" {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			break
		}

		if strings.HasPrefix(rest, "{") {
			end := findMatchingBrace(rest, 0)
			if end < 0 {
				break
			}
			exprStr := rest[1:end]
			exprStr = strings.TrimSpace(exprStr)
			if exprStr != "" {
				child := parseJSXExpression(exprStr)
				if child != nil {
					children = append(children, child)
				}
			}
			rest = rest[end+1:]
			continue
		}

		if strings.HasPrefix(rest, "<") {
			// Check for closing tag — we shouldn't see it here but handle gracefully
			if strings.HasPrefix(rest, "</") {
				break
			}

			tag, props, afterTag, selfClosing := scanOpeningTag(rest)
			if tag == "" {
				break
			}

			if selfClosing {
				children = append(children, &ast.Node{
					Type:  ast.NodeJSXElement,
					Tag:   tag,
					Props: props,
				})
				rest = afterTag
				continue
			}

			// Opening tag — extract inner content
			inner2, remaining := extractInner(tag, afterTag)
			childNode := &ast.Node{
				Type:     ast.NodeJSXElement,
				Tag:      tag,
				Props:    props,
				Children: parseChildren(inner2),
			}
			children = append(children, childNode)
			rest = remaining
		} else {
			// Text node — consume up to next '<' or '{'
			idx := strings.IndexAny(rest, "<{")
			var text string
			if idx == -1 {
				text = rest
				rest = ""
			} else {
				text = rest[:idx]
				rest = rest[idx:]
			}
			text = strings.TrimSpace(text)
			if text != "" {
				children = append(children, &ast.Node{Type: ast.NodeJSXText, Text: text})
			}
		}
	}
	return children
}

// findMatchingBrace finds the index of the closing '}' that matches the '{' at start.
// It properly handles string literals so braces inside strings don't affect depth.
func findMatchingBrace(s string, start int) int {
	if start >= len(s) || s[start] != '{' {
		return -1
	}
	depth := 0
	inStr := false
	strChar := byte(0)
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if c == strChar && !(i > 0 && s[i-1] == '\\') {
				inStr = false
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			inStr = true
			strChar = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
