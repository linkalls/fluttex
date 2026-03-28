package generator_test

import (
	"strings"
	"testing"

	"github.com/linkalls/fluttex/internal/generator"
	"github.com/linkalls/fluttex/internal/parser"
)

// compile is a helper that parses TSX then generates Dart.
func compile(tsx string) string {
	nodes := parser.ParseFile(strings.TrimSpace(tsx))
	return generator.Generate(nodes)
}

// assertContains fails the test if the generated output doesn't contain the expected substring.
func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain:\n  %q\ngot:\n%s", want, got)
	}
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestStatelessWidget(t *testing.T) {
	tsx := `
function Hello() {
  return (
    <div>
      <span>Hello World</span>
    </div>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "import 'package:flutter/material.dart';")
	assertContains(t, out, "class Hello extends StatelessWidget")
	assertContains(t, out, "const Hello({super.key});")
	assertContains(t, out, "Widget build(BuildContext context)")
	assertContains(t, out, "Text('Hello World')")
}

func TestStatefulWidget(t *testing.T) {
	tsx := `
function Counter() {
  const [count, setCount] = useState(0);
  return (
    <button onClick={() => setCount(count + 1)}>
      <span>{count}</span>
    </button>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "class Counter extends StatefulWidget")
	assertContains(t, out, "State<Counter> createState() => _CounterState();")
	assertContains(t, out, "class _CounterState extends State<Counter>")
	assertContains(t, out, "int count = 0;")
	assertContains(t, out, "ElevatedButton(")
}

func TestButtonTranslation(t *testing.T) {
	tsx := `
function MyBtn() {
  return (
    <button data-testid="submit-btn" onClick={handleSubmit}>Submit</button>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "ElevatedButton(")
	assertContains(t, out, "key: const Key('submit-btn')")
	assertContains(t, out, "onPressed: handleSubmit")
	assertContains(t, out, "Text('Submit')")
}

func TestTextFieldTranslation(t *testing.T) {
	tsx := `
function MyForm() {
  return (
    <input onChange={handleChange} placeholder="Enter text" />
  );
}`
	out := compile(tsx)
	assertContains(t, out, "TextField(")
	assertContains(t, out, "onChanged: handleChange")
	assertContains(t, out, "hintText: 'Enter text'")
}

func TestIconTranslation(t *testing.T) {
	tsx := `
function CheckIcon() {
  return (
    <Icon name="check" />
  );
}`
	out := compile(tsx)
	assertContains(t, out, "Icon(Icons.check)")
}

func TestNestedElements(t *testing.T) {
	tsx := `
function Layout() {
  return (
    <div>
      <span>Title</span>
      <span>Body</span>
    </div>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "Column(")
	assertContains(t, out, "Text('Title')")
	assertContains(t, out, "Text('Body')")
}

func TestDataTestId(t *testing.T) {
	tsx := `
function MyWidget() {
  return (
    <div data-testid="my-container">
      <span>content</span>
    </div>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "key: const Key('my-container')")
}
