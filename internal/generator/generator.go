// Package generator converts the internal AST into Flutter/Dart source code.
package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/linkalls/fluttex/internal/ast"
)

// Generate converts a slice of top-level Component nodes into a complete Dart source file.
func Generate(nodes []*ast.Node) string {
	var sb strings.Builder
	sb.WriteString("import 'package:flutter/material.dart';\n")

	for _, n := range nodes {
		if n.Type == ast.NodeComponent {
			sb.WriteString("\n")
			sb.WriteString(generateComponent(n))
		}
	}

	return sb.String()
}

func generateComponent(n *ast.Node) string {
	if len(n.StateVars) > 0 || len(n.Effects) > 0 {
		return generateStatefulWidget(n)
	}
	return generateStatelessWidget(n)
}

// ── StatelessWidget ──────────────────────────────────────────────────────────

func generateStatelessWidget(n *ast.Node) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "class %s extends StatelessWidget {\n", n.Name)
	fmt.Fprintf(&sb, "  const %s({super.key});\n\n", n.Name)
	sb.WriteString("  @override\n")
	sb.WriteString("  Widget build(BuildContext context) {\n")
	sb.WriteString("    return ")
	sb.WriteString(generateWidget(n.Body, 2))
	sb.WriteString(";\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")
	return sb.String()
}

// ── StatefulWidget ───────────────────────────────────────────────────────────

func generateStatefulWidget(n *ast.Node) string {
	stateName := "_" + n.Name + "State"
	var sb strings.Builder

	// StatefulWidget class
	fmt.Fprintf(&sb, "class %s extends StatefulWidget {\n", n.Name)
	fmt.Fprintf(&sb, "  const %s({super.key});\n\n", n.Name)
	sb.WriteString("  @override\n")
	fmt.Fprintf(&sb, "  State<%s> createState() => %s();\n", n.Name, stateName)
	sb.WriteString("}\n\n")

	// State class
	fmt.Fprintf(&sb, "class %s extends State<%s> {\n", stateName, n.Name)
	for _, sv := range n.StateVars {
		dartType := inferDartType(sv.Initial)
		fmt.Fprintf(&sb, "  %s %s = %s;\n", dartType, sv.Name, sv.Initial)
	}

	// initState for mount-only effects (empty deps array)
	mountEffects := effectsWithDeps(n.Effects, true, true)
	// dispose effects (those that have a cleanup)
	cleanupEffects := effectsWithCleanup(n.Effects)
	// effects that run on every update (no deps array)
	everyEffects := effectsWithDeps(n.Effects, false, false)

	if len(mountEffects) > 0 || len(everyEffects) > 0 {
		sb.WriteString("\n  @override\n")
		sb.WriteString("  void initState() {\n")
		sb.WriteString("    super.initState();\n")
		for _, e := range mountEffects {
			for _, line := range splitLines(e.Body) {
				if line != "" {
					fmt.Fprintf(&sb, "    %s\n", line)
				}
			}
		}
		for _, e := range everyEffects {
			for _, line := range splitLines(e.Body) {
				if line != "" {
					fmt.Fprintf(&sb, "    %s\n", line)
				}
			}
		}
		sb.WriteString("  }\n")
	}

	if len(cleanupEffects) > 0 {
		sb.WriteString("\n  @override\n")
		sb.WriteString("  void dispose() {\n")
		for _, e := range cleanupEffects {
			for _, line := range splitLines(e.Cleanup) {
				if line != "" {
					fmt.Fprintf(&sb, "    %s\n", line)
				}
			}
		}
		sb.WriteString("    super.dispose();\n")
		sb.WriteString("  }\n")
	}

	sb.WriteString("\n  @override\n")
	sb.WriteString("  Widget build(BuildContext context) {\n")
	sb.WriteString("    return ")
	// Pass state vars for setState wrapping
	sb.WriteString(generateWidgetWithState(n.Body, 2, n.StateVars))
	sb.WriteString(";\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// effectsWithDeps returns effects that have (or don't have) a deps array, optionally empty.
func effectsWithDeps(effects []ast.Effect, hasDeps bool, emptyOnly bool) []ast.Effect {
	var out []ast.Effect
	for _, e := range effects {
		if e.HasDeps != hasDeps {
			continue
		}
		if emptyOnly && len(e.Deps) > 0 {
			continue
		}
		out = append(out, e)
	}
	return out
}

func effectsWithCleanup(effects []ast.Effect) []ast.Effect {
	var out []ast.Effect
	for _, e := range effects {
		if e.Cleanup != "" {
			out = append(out, e)
		}
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// inferDartType returns a Dart type string for a literal initial value.
func inferDartType(initial string) string {
	initial = strings.TrimSpace(initial)
	if initial == "true" || initial == "false" {
		return "bool"
	}
	if _, err := fmt.Sscanf(initial, "%f", new(float64)); err == nil {
		if strings.Contains(initial, ".") {
			return "double"
		}
		return "int"
	}
	if strings.HasPrefix(initial, `"`) || strings.HasPrefix(initial, `'`) {
		return "String"
	}
	return "var"
}

// ── Widget generation ────────────────────────────────────────────────────────

// generateWidget converts a single JSX Node to Dart widget code.
func generateWidget(n *ast.Node, indent int) string {
	return generateWidgetWithState(n, indent, nil)
}

// generateWidgetWithState converts a single JSX Node to Dart widget code, aware of state setters.
func generateWidgetWithState(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	if n == nil {
		return "SizedBox.shrink()"
	}
	switch n.Type {
	case ast.NodeJSXText:
		return fmt.Sprintf("Text('%s')", escapeSingleQuote(n.Text))
	case ast.NodeJSXElement:
		return generateElementWithState(n, indent, stateVars)
	case ast.NodeJSXExpression:
		return generateExpression(n, stateVars)
	case ast.NodeConditional:
		return generateConditional(n, indent, stateVars)
	case ast.NodeListRender:
		return generateListRender(n, indent, stateVars)
	}
	return "SizedBox.shrink()"
}

func generateExpression(n *ast.Node, stateVars []ast.StateVar) string {
	expr := n.Expression
	// If it looks like a simple variable that could be a string (e.g. title, name),
	// wrap in Text() for widget context.
	// Heuristic: if it's a simple identifier or basic expression, wrap as Text.
	if isSimpleExpr(expr) {
		return fmt.Sprintf("Text(%s.toString())", expr)
	}
	// String literal expression
	if strings.HasPrefix(expr, `"`) || strings.HasPrefix(expr, `'`) {
		return fmt.Sprintf("Text(%s)", expr)
	}
	// Default: wrap as Text
	return fmt.Sprintf("Text(%s.toString())", expr)
}

func generateConditional(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	cond := n.Condition
	consequent := generateWidgetWithState(n.Consequent, indent, stateVars)
	if n.Alternate == nil {
		// && shorthand: show or SizedBox.shrink()
		return fmt.Sprintf("%s ? %s : const SizedBox.shrink()", cond, consequent)
	}
	alternate := generateWidgetWithState(n.Alternate, indent, stateVars)
	return fmt.Sprintf("%s ? %s : %s", cond, consequent, alternate)
}

func generateListRender(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	itemWidget := generateWidgetWithState(n.ListBody, indent+1, stateVars)
	// Generates: ...listExpr.map((item) => itemWidget).toList()
	// Wraps in a Column for widget context
	return fmt.Sprintf("Column(\n%schildren: %s.map((%s) => %s).toList(),\n%s)",
		ind(indent+1), n.ListExpr, n.ListItem, itemWidget, ind(indent))
}

func generateElementWithState(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	tag := strings.ToLower(n.Tag)

	switch tag {
	case "button":
		return generateButtonWithState(n, indent, stateVars)
	case "input":
		return generateTextFieldWithState(n, indent, stateVars)
	case "icon":
		return generateIcon(n)
	case "image", "img":
		return generateImage(n)
	case "scaffold":
		return generateScaffold(n, indent, stateVars)
	case "appbar":
		return generateAppBar(n, indent, stateVars)
	case "listview":
		return generateListView(n, indent, stateVars)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return generateHeadingWithState(n, tag, stateVars)
	case "span", "text", "p", "label":
		return generateTextWithState(n, stateVars)
	case "div", "view", "section", "article", "main", "header", "footer", "nav":
		return generateContainerWithState(n, indent, stateVars)
	default:
		// Unknown capitalised tags are treated as custom Flutter widgets
		if n.Tag != "" && n.Tag[0] >= 'A' && n.Tag[0] <= 'Z' {
			return generateCustomWidget(n, indent, stateVars)
		}
		return generateContainerWithState(n, indent, stateVars)
	}
}

// ── Individual widget generators ─────────────────────────────────────────────

func generateButton(n *ast.Node, indent int) string {
	return generateButtonWithState(n, indent, nil)
}

func generateButtonWithState(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	var parts []string

	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}

	onPressed := "() {}"
	if v, ok := n.Props["onClick"]; ok && v != "" {
		onPressed = wrapSetterCalls(v, stateVars)
	}
	parts = append(parts, fmt.Sprintf("onPressed: %s", onPressed))

	child := childWidgetWithState(n, indent, stateVars)
	parts = append(parts, fmt.Sprintf("child: %s", child))

	return fmt.Sprintf("ElevatedButton(\n%s)", formatArgs(parts, indent+1))
}

func generateTextField(n *ast.Node, indent int) string {
	return generateTextFieldWithState(n, indent, nil)
}

// normalizeOnChange rewrites React event-handler patterns into Dart-friendly form.
// Specifically it transforms `(e) => setter(e.target.value)` → `(value) => setter(value)`
// so that Flutter's TextField.onChanged (which passes a String directly) is correct.
var eTargetValueRe = regexp.MustCompile(`(?s)^\((\w+)\)\s*=>(.+)$`)

func normalizeOnChange(handler string) string {
	m := eTargetValueRe.FindStringSubmatch(strings.TrimSpace(handler))
	if m == nil {
		return handler
	}
	param := m[1]
	body := strings.TrimSpace(m[2])
	if strings.Contains(body, param+".target.value") {
		body = strings.ReplaceAll(body, param+".target.value", "value")
		return "(value) => " + body
	}
	return handler
}

func generateTextFieldWithState(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	var parts []string

	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}
	if v, ok := n.Props["onChange"]; ok && v != "" {
		handler := normalizeOnChange(v)
		parts = append(parts, fmt.Sprintf("onChanged: %s", wrapSetterCalls(handler, stateVars)))
	}
	if v, ok := n.Props["placeholder"]; ok && v != "" {
		parts = append(parts, fmt.Sprintf("decoration: const InputDecoration(hintText: '%s')", escapeSingleQuote(v)))
	}

	if len(parts) == 0 {
		return "TextField()"
	}
	return fmt.Sprintf("TextField(\n%s)", formatArgs(parts, indent+1))
}

func generateIcon(n *ast.Node) string {
	iconName := "help"
	if v, ok := n.Props["name"]; ok && v != "" {
		iconName = camelToSnake(v)
	}
	var parts []string
	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Icon(Icons.%s)", iconName)
	}
	return fmt.Sprintf("Icon(Icons.%s, %s)", iconName, strings.Join(parts, ", "))
}

func generateImage(n *ast.Node) string {
	src := n.Props["src"]
	alt := n.Props["alt"]
	var parts []string
	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}
	if src != "" {
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			parts = append(parts, fmt.Sprintf("image: NetworkImage('%s')", escapeSingleQuote(src)))
		} else {
			parts = append(parts, fmt.Sprintf("image: AssetImage('%s')", escapeSingleQuote(src)))
		}
	}
	if alt != "" {
		parts = append(parts, fmt.Sprintf("semanticLabel: '%s'", escapeSingleQuote(alt)))
	}
	if len(parts) == 0 {
		return "Image(image: AssetImage(''))"
	}
	return fmt.Sprintf("Image(\n%s)", formatArgs(parts, 3))
}

func generateScaffold(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	var parts []string
	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}
	// Find AppBar child and body children
	var appBarNode *ast.Node
	var bodyChildren []*ast.Node
	for _, c := range n.Children {
		if c.Type == ast.NodeJSXElement && strings.EqualFold(c.Tag, "AppBar") {
			appBarNode = c
		} else {
			bodyChildren = append(bodyChildren, c)
		}
	}
	if appBarNode != nil {
		parts = append(parts, fmt.Sprintf("appBar: %s", generateAppBar(appBarNode, indent+1, stateVars)))
	}
	if len(bodyChildren) > 0 {
		bodyNode := &ast.Node{Type: ast.NodeJSXElement, Tag: "div", Children: bodyChildren}
		parts = append(parts, fmt.Sprintf("body: %s", generateContainerWithState(bodyNode, indent+1, stateVars)))
	}
	if len(parts) == 0 {
		return "Scaffold()"
	}
	return fmt.Sprintf("Scaffold(\n%s)", formatArgs(parts, indent+1))
}

func generateAppBar(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	var parts []string
	// title prop or first text child
	if v, ok := n.Props["title"]; ok && v != "" {
		parts = append(parts, fmt.Sprintf("title: Text('%s')", escapeSingleQuote(v)))
	} else if len(n.Children) > 0 {
		parts = append(parts, fmt.Sprintf("title: %s", generateWidgetWithState(n.Children[0], indent+1, stateVars)))
	}
	if len(parts) == 0 {
		return "AppBar()"
	}
	return fmt.Sprintf("AppBar(\n%s)", formatArgs(parts, indent+1))
}

func generateListView(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	real := filterChildren(n.Children)
	var parts []string
	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}
	var childStrs []string
	for _, c := range real {
		childStrs = append(childStrs, ind(indent+2)+generateWidgetWithState(c, indent+2, stateVars))
	}
	if len(childStrs) > 0 {
		children := fmt.Sprintf("children: [\n%s,\n%s]",
			strings.Join(childStrs, ",\n"), ind(indent+1))
		parts = append(parts, children)
	}
	if len(parts) == 0 {
		return "ListView(children: [])"
	}
	return fmt.Sprintf("ListView(\n%s)", formatArgs(parts, indent+1))
}

// headingStyles maps h1-h6 tags to Flutter TextTheme getter names.
var headingStyles = map[string]string{
	"h1": "headlineLarge",
	"h2": "headlineMedium",
	"h3": "headlineSmall",
	"h4": "titleLarge",
	"h5": "titleMedium",
	"h6": "titleSmall",
}

func generateHeadingWithState(n *ast.Node, tag string, stateVars []ast.StateVar) string {
	textWidget := generateTextWithState(n, stateVars)
	styleName, ok := headingStyles[tag]
	if !ok {
		return textWidget
	}
	// Wrap Text with a style from the theme: replace closing ')' to add style arg
	if strings.HasPrefix(textWidget, "Text(") && strings.HasSuffix(textWidget, ")") {
		inner := textWidget[len("Text(") : len(textWidget)-1]
		return fmt.Sprintf("Text(%s, style: Theme.of(context).textTheme.%s)", inner, styleName)
	}
	return textWidget
}

func generateTextWithState(n *ast.Node, stateVars []ast.StateVar) string {
	var parts []string
	for _, c := range n.Children {
		if c.Type == ast.NodeJSXText {
			text := c.Text
			if strings.TrimSpace(text) != "" {
				parts = append(parts, escapeSingleQuote(text))
			}
		} else if c.Type == ast.NodeJSXExpression {
			parts = append(parts, "${"+c.Expression+"}")
		}
	}
	if len(parts) > 0 {
		combined := strings.Join(parts, "")
		return fmt.Sprintf("Text('%s')", combined)
	}
	// If the element itself has text content (no children)
	return "Text('')"
}

func generateCustomWidget(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	var parts []string
	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}
	// Pass common props as named args
	for _, prop := range []string{"title", "label", "value", "text"} {
		if v, ok := n.Props[prop]; ok && v != "" {
			parts = append(parts, fmt.Sprintf("%s: '%s'", prop, escapeSingleQuote(v)))
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%s()", n.Tag)
	}
	return fmt.Sprintf("%s(\n%s)", n.Tag, formatArgs(parts, indent+1))
}

func generateContainerWithState(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	realChildren := filterChildren(n.Children)

	if len(realChildren) == 0 {
		if key, ok := keyProp(n); ok {
			return fmt.Sprintf("Container(%s)", key)
		}
		return "Container()"
	}

	var parts []string
	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}

	if len(realChildren) == 1 {
		parts = append(parts, fmt.Sprintf("child: %s", generateWidgetWithState(realChildren[0], indent+1, stateVars)))
		return fmt.Sprintf("Container(\n%s)", formatArgs(parts, indent+1))
	}

	// Multiple children → Column
	var childStrs []string
	for _, c := range realChildren {
		childStrs = append(childStrs, ind(indent+2)+generateWidgetWithState(c, indent+2, stateVars))
	}
	columnChildren := fmt.Sprintf("children: [\n%s,\n%s]",
		strings.Join(childStrs, ",\n"),
		ind(indent+1))

	parts = append(parts, columnChildren)
	return fmt.Sprintf("Column(\n%s)", formatArgs(parts, indent+1))
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// keyProp returns the Dart key parameter if data-testid is present.
func keyProp(n *ast.Node) (string, bool) {
	if v, ok := n.Props["data-testid"]; ok && v != "" {
		return fmt.Sprintf("key: const Key('%s')", escapeSingleQuote(v)), true
	}
	return "", false
}

// childWidgetWithState returns the single child widget string for a button etc.
func childWidgetWithState(n *ast.Node, indent int, stateVars []ast.StateVar) string {
	real := filterChildren(n.Children)
	if len(real) == 0 {
		return "const SizedBox.shrink()"
	}
	if len(real) == 1 {
		return generateWidgetWithState(real[0], indent+1, stateVars)
	}
	var parts []string
	for _, c := range real {
		parts = append(parts, ind(indent+2)+generateWidgetWithState(c, indent+2, stateVars))
	}
	return fmt.Sprintf("Column(children: [\n%s\n%s])", strings.Join(parts, ",\n"), ind(indent+1))
}

// childWidget returns the single child widget string (no state awareness).
func childWidget(n *ast.Node, indent int) string {
	return childWidgetWithState(n, indent, nil)
}

// filterChildren removes empty text nodes.
func filterChildren(nodes []*ast.Node) []*ast.Node {
	var out []*ast.Node
	for _, n := range nodes {
		if n.Type == ast.NodeJSXText && strings.TrimSpace(n.Text) == "" {
			continue
		}
		out = append(out, n)
	}
	return out
}

// formatArgs renders a list of named arguments indented under the widget call.
func formatArgs(parts []string, indent int) string {
	prefix := ind(indent)
	var lines []string
	for _, p := range parts {
		lines = append(lines, prefix+p)
	}
	// closing paren sits at parent indent
	return strings.Join(lines, ",\n") + ",\n" + ind(indent-1)
}

// ind returns n levels of 2-space indentation.
func ind(n int) string {
	return strings.Repeat("  ", n)
}

func escapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", `\'`)
}

// camelToSnake converts camelCase/PascalCase to snake_case for Icons.
func camelToSnake(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, []rune(strings.ToLower(string(r)))...)
	}
	return string(result)
}

// isSimpleExpr returns true for simple identifiers or member access expressions.
func isSimpleExpr(expr string) bool {
	// Simple identifier or dotted expression: foo, foo.bar, foo.bar.baz
	for _, c := range expr {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// wrapSetterCalls rewrites setter calls inside an event handler lambda so that
// state mutations are wrapped in setState(). For example:
//
//	() => setCount(count + 1)
//	→ () => setState(() { count = count + 1; })
func wrapSetterCalls(handler string, stateVars []ast.StateVar) string {
	if len(stateVars) == 0 {
		return handler
	}

	// Build a map of setter name → state var name
	setterMap := map[string]string{}
	for _, sv := range stateVars {
		setterMap[sv.Setter] = sv.Name
	}

	// Check if any setter is called in the handler
	hasSetter := false
	for setter := range setterMap {
		if strings.Contains(handler, setter+"(") {
			hasSetter = true
			break
		}
	}
	if !hasSetter {
		return handler
	}

	// Replace setter(value) → varName = value (inside setState body)
	body := handler
	for setter, varName := range setterMap {
		// Match setter(expr) — simple single-arg call
		setterCallRe := regexp.MustCompile(regexp.QuoteMeta(setter) + `\(([^)]*)\)`)
		body = setterCallRe.ReplaceAllStringFunc(body, func(match string) string {
			sub := setterCallRe.FindStringSubmatch(match)
			if sub == nil {
				return match
			}
			return varName + " = " + sub[1]
		})
	}

	// Now wrap the lambda body with setState
	// Handle: () => expr  and  (args) => expr
	arrowLambdaRe := regexp.MustCompile(`(?s)^(\([^)]*\)|[A-Za-z_]\w*)\s*=>(.+)$`)
	if m := arrowLambdaRe.FindStringSubmatch(strings.TrimSpace(body)); m != nil {
		params := strings.TrimSpace(m[1])
		innerBody := strings.TrimSpace(m[2])
		// Strip outer {} only if the opening brace matches the closing brace at the end
		if strings.HasPrefix(innerBody, "{") {
			closeIdx := findMatchingBraceInBody(innerBody, 0)
			if closeIdx == len(innerBody)-1 {
				innerBody = strings.TrimSpace(innerBody[1:closeIdx])
			}
		}
		return fmt.Sprintf("%s => setState(() { %s; })", params, innerBody)
	}

	return handler
}

// findMatchingBraceInBody finds the closing '}' matching the '{' at position start,
// handling string literals to avoid false matches.
func findMatchingBraceInBody(s string, start int) int {
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
