package ast

// NodeType identifies the kind of AST node.
type NodeType string

const (
	NodeComponent  NodeType = "Component"
	NodeJSXElement NodeType = "JSXElement"
	NodeJSXText    NodeType = "JSXText"
	NodeUseState   NodeType = "UseState"
)

// StateVar represents a useState hook call.
type StateVar struct {
	// Name is the state variable name (e.g. "count").
	Name string
	// Setter is the setter function name (e.g. "setCount").
	Setter string
	// Initial is the initial value literal (e.g. "0", `""`, "false").
	Initial string
}

// Node is a generic AST node used throughout the transpiler.
type Node struct {
	Type NodeType

	// Component fields
	Name      string
	StateVars []StateVar
	Body      *Node // the root JSX element returned by the component

	// JSXElement fields
	Tag      string            // e.g. "div", "button", "Icon"
	Props    map[string]string // attribute name → raw value string
	Children []*Node

	// JSXText fields
	Text string
}
