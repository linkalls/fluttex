// Package parser provides a simple regex/string-based TSX parser that converts
// a TSX source string into an internal AST representation.
package parser

import (
	"regexp"
	"strings"

	"github.com/linkalls/fluttex/internal/ast"
)

var (
	// componentRe matches: function ComponentName(...) { ... }
	componentRe = regexp.MustCompile(`(?s)(?:export\s+(?:default\s+)?)?function\s+([A-Z][A-Za-z0-9_]*)\s*\([^)]*\)\s*\{(.+)\}`)

	// useStateRe matches: const [name, setter] = useState(initial)
	useStateRe = regexp.MustCompile(`const\s+\[(\w+),\s*(\w+)\]\s*=\s*useState\(([^)]*)\)`)

	// returnJSXRe extracts the JSX block from a return statement
	returnJSXRe = regexp.MustCompile(`(?s)return\s*\(\s*(<.+>)\s*\)\s*;?\s*$`)

	// selfClosingRe matches <Tag .../>
	selfClosingRe = regexp.MustCompile(`(?s)^<([A-Za-z][A-Za-z0-9._-]*)((?:\s+[^>]*?)?)\s*/>$`)

	// openTagRe matches <Tag ...>
	openTagRe = regexp.MustCompile(`(?s)^<([A-Za-z][A-Za-z0-9._-]*)((?:\s+[^>]*)?)>`)

	// propRe matches key="value" or key={value} or key
	propRe = regexp.MustCompile(`([\w-]+)(?:=(?:"([^"]*)"|'([^']*)'|\{([^}]*)\}))?`)
)

// ParseFile parses a TSX source string and returns a slice of top-level Component nodes.
func ParseFile(src string) []*ast.Node {
	src = strings.TrimSpace(src)
	var components []*ast.Node

	matches := componentRe.FindAllStringSubmatchIndex(src, -1)
	for _, loc := range matches {
		name := src[loc[2]:loc[3]]
		body := src[loc[4]:loc[5]]
		comp := parseComponent(name, body)
		components = append(components, comp)
	}

	// If no function component found, try to parse raw JSX
	if len(components) == 0 {
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

	// Extract returned JSX
	m := returnJSXRe.FindStringSubmatch(body)
	if m != nil {
		comp.Body = parseJSX(strings.TrimSpace(m[1]))
	}

	return comp
}

// parseJSX parses a JSX string fragment into an AST node.
func parseJSX(s string) *ast.Node {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Self-closing tag: <Tag ... />
	if m := selfClosingRe.FindStringSubmatch(s); m != nil {
		return &ast.Node{
			Type:  ast.NodeJSXElement,
			Tag:   m[1],
			Props: parseProps(m[2]),
		}
	}

	// Opening tag: <Tag ...>children</Tag>
	if m := openTagRe.FindStringSubmatch(s); m != nil {
		tag := m[1]
		props := parseProps(m[2])
		afterOpen := s[len(m[0]):]
		inner, _ := extractInner(tag, afterOpen)
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

		if strings.HasPrefix(rest, "<") {
			// Find the tag name
			tagMatch := regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9._-]*)`)
			tm := tagMatch.FindStringSubmatch(rest)
			if tm == nil {
				break
			}
			tag := tm[1]

			// Self-closing?
			scm := selfClosingRe.FindStringSubmatch(rest)
			if scm != nil {
				children = append(children, &ast.Node{
					Type:  ast.NodeJSXElement,
					Tag:   scm[1],
					Props: parseProps(scm[2]),
				})
				rest = rest[len(scm[0]):]
				continue
			}

			// Opening tag
			om := openTagRe.FindStringSubmatch(rest)
			if om == nil {
				break
			}
			afterOpen := rest[len(om[0]):]
			inner2, remaining := extractInner(tag, afterOpen)
			childNode := &ast.Node{
				Type:     ast.NodeJSXElement,
				Tag:      tag,
				Props:    parseProps(om[2]),
				Children: parseChildren(inner2),
			}
			children = append(children, childNode)
			rest = remaining
		} else {
			// Text node — consume up to next '<'
			idx := strings.Index(rest, "<")
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

// parseProps parses an attribute string into a map.
func parseProps(attrs string) map[string]string {
	props := map[string]string{}
	attrs = strings.TrimSpace(attrs)
	if attrs == "" {
		return props
	}
	for _, m := range propRe.FindAllStringSubmatch(attrs, -1) {
		key := m[1]
		val := m[2] // "..."
		if val == "" {
			val = m[3] // '...'
		}
		if val == "" {
			val = m[4] // {...}
		}
		props[key] = val
	}
	return props
}
