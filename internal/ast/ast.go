package ast

// NodeType identifies the kind of AST node.
type NodeType string

const (
	NodeComponent      NodeType = "Component"
	NodeJSXElement     NodeType = "JSXElement"
	NodeJSXText        NodeType = "JSXText"
	NodeJSXExpression  NodeType = "JSXExpression"  // {expr} inside JSX
	NodeConditional    NodeType = "Conditional"     // {cond ? A : B} or {cond && A}
	NodeListRender     NodeType = "ListRender"      // {items.map(...)}
	NodeUseState       NodeType = "UseState"
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

// Effect represents a useEffect hook call.
type Effect struct {
	// Body is the raw effect callback body (without outer function wrapper).
	Body string
	// Deps lists the dependency variable names. Empty means run every render,
	// nil (vs []string{}) means no deps array was provided.
	Deps []string
	// HasDeps is true when a deps array was explicitly provided (even if empty).
	HasDeps bool
	// Cleanup is the raw return cleanup body, if any.
	Cleanup string
}

// Node is a generic AST node used throughout the transpiler.
type Node struct {
	Type NodeType

	// Component fields
	Name      string
	StateVars []StateVar
	Effects   []Effect
	Body      *Node // the root JSX element returned by the component

	// JSXElement fields
	Tag      string            // e.g. "div", "button", "Icon"
	Props    map[string]string // attribute name → raw value string
	Children []*Node

	// JSXText fields
	Text string

	// JSXExpression / Conditional / ListRender fields
	Expression  string // raw TS expression
	Condition   string // for Conditional: the condition expression
	Consequent  *Node  // for Conditional: true branch
	Alternate   *Node  // for Conditional: false branch (nil for && shorthand)
	ListExpr    string // for ListRender: the raw .map(...) expression
	ListItem    string // for ListRender: item variable name
	ListBody    *Node  // for ListRender: the JSX template per item
}
