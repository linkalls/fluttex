// Package generator converts the internal AST into Flutter/Dart source code.
package generator

import (
	"fmt"
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
	if len(n.StateVars) > 0 {
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
	sb.WriteString("\n  @override\n")
	sb.WriteString("  Widget build(BuildContext context) {\n")
	sb.WriteString("    return ")
	sb.WriteString(generateWidget(n.Body, 2))
	sb.WriteString(";\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	return sb.String()
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
// indent is the current indentation level (number of 2-space steps).
func generateWidget(n *ast.Node, indent int) string {
	if n == nil {
		return "SizedBox.shrink()"
	}
	switch n.Type {
	case ast.NodeJSXText:
		return fmt.Sprintf("Text('%s')", escapeSingleQuote(n.Text))
	case ast.NodeJSXElement:
		return generateElement(n, indent)
	}
	return "SizedBox.shrink()"
}

func generateElement(n *ast.Node, indent int) string {
	tag := strings.ToLower(n.Tag)

	switch tag {
	case "button":
		return generateButton(n, indent)
	case "input":
		return generateTextField(n, indent)
	case "icon":
		return generateIcon(n)
	case "span", "text", "p", "label", "h1", "h2", "h3", "h4", "h5", "h6":
		return generateText(n)
	case "div", "view", "section", "article", "main", "header", "footer", "nav":
		return generateContainer(n, indent)
	default:
		// Treat unknown as Container
		return generateContainer(n, indent)
	}
}

// ── Individual widget generators ─────────────────────────────────────────────

func generateButton(n *ast.Node, indent int) string {
	var parts []string

	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}

	onPressed := "() {}"
	if v, ok := n.Props["onClick"]; ok && v != "" {
		onPressed = v
	}
	parts = append(parts, fmt.Sprintf("onPressed: %s", onPressed))

	child := childWidget(n, indent)
	parts = append(parts, fmt.Sprintf("child: %s", child))

	return fmt.Sprintf("ElevatedButton(\n%s)", formatArgs(parts, indent+1))
}

func generateTextField(n *ast.Node, indent int) string {
	var parts []string

	if key, ok := keyProp(n); ok {
		parts = append(parts, key)
	}
	if v, ok := n.Props["onChange"]; ok && v != "" {
		parts = append(parts, fmt.Sprintf("onChanged: %s", v))
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

func generateText(n *ast.Node) string {
	// If there's a direct text child, use it
	for _, c := range n.Children {
		if c.Type == ast.NodeJSXText {
			return fmt.Sprintf("Text('%s')", escapeSingleQuote(c.Text))
		}
	}
	// If the element itself has text content (no children)
	return "Text('')"
}

func generateContainer(n *ast.Node, indent int) string {
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
		parts = append(parts, fmt.Sprintf("child: %s", generateWidget(realChildren[0], indent+1)))
		return fmt.Sprintf("Container(\n%s)", formatArgs(parts, indent+1))
	}

	// Multiple children → Column
	var childStrs []string
	for _, c := range realChildren {
		childStrs = append(childStrs, ind(indent+2)+generateWidget(c, indent+2))
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

// childWidget returns the single child widget string for a button etc.
func childWidget(n *ast.Node, indent int) string {
	real := filterChildren(n.Children)
	if len(real) == 0 {
		return "const SizedBox.shrink()"
	}
	if len(real) == 1 {
		return generateWidget(real[0], indent+1)
	}
	var parts []string
	for _, c := range real {
		parts = append(parts, ind(indent+2)+generateWidget(c, indent+2))
	}
	return fmt.Sprintf("Column(children: [\n%s\n%s])", strings.Join(parts, ",\n"), ind(indent+1))
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
