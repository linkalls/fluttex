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

// ── Original Tests ────────────────────────────────────────────────────────────

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

// ── New Tests ─────────────────────────────────────────────────────────────────

func TestArrowFunctionComponent(t *testing.T) {
	tsx := `
const Greeting = () => (
  <div>
    <span>Hello Arrow</span>
  </div>
);`
	out := compile(tsx)
	assertContains(t, out, "class Greeting extends StatelessWidget")
	assertContains(t, out, "Text('Hello Arrow')")
}

func TestArrowFunctionComponentWithBlock(t *testing.T) {
	tsx := `
const Profile = () => {
  return (
    <div>
      <span>Profile Page</span>
    </div>
  );
};`
	out := compile(tsx)
	assertContains(t, out, "class Profile extends StatelessWidget")
	assertContains(t, out, "Text('Profile Page')")
}

func TestJSXExpressionChild(t *testing.T) {
	tsx := `
function Dynamic() {
  const [count, setCount] = useState(0);
  return (
    <div>
      <span>{count}</span>
    </div>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "class Dynamic extends StatefulWidget")
	assertContains(t, out, "int count = 0;")
	// The {count} expression should render as a Text widget
	assertContains(t, out, "count.toString()")
}

func TestConditionalRendering(t *testing.T) {
	tsx := `
function Visibility() {
  const [show, setShow] = useState(true);
  return (
    <div>
      {show ? <span>Visible</span> : <span>Hidden</span>}
    </div>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "show ?")
	assertContains(t, out, "Text('Visible')")
	assertContains(t, out, "Text('Hidden')")
}

func TestConditionalAndShorthand(t *testing.T) {
	tsx := `
function Toggle() {
  const [open, setOpen] = useState(false);
  return (
    <div>
      {open && <span>Open!</span>}
    </div>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "open ?")
	assertContains(t, out, "Text('Open!')")
	assertContains(t, out, "const SizedBox.shrink()")
}

func TestSetStateWrapping(t *testing.T) {
	tsx := `
function Counter() {
  const [count, setCount] = useState(0);
  return (
    <button data-testid="inc-btn" onClick={() => setCount(count + 1)}>
      <span>Increment</span>
    </button>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "setState(")
	assertContains(t, out, "count = count + 1")
	assertContains(t, out, "key: const Key('inc-btn')")
}

func TestUseEffect(t *testing.T) {
	tsx := `
function Timer() {
  const [tick, setTick] = useState(0);
  useEffect(() => {
    console.log('mounted');
  }, []);
  return (
    <span>{tick}</span>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "class Timer extends StatefulWidget")
	assertContains(t, out, "initState()")
	assertContains(t, out, "super.initState();")
}

func TestUseEffectWithCleanup(t *testing.T) {
	tsx := `
function Sub() {
  useEffect(() => {
    subscribe();
    return () => {
      unsubscribe();
    };
  }, []);
  return (<div />);
}`
	out := compile(tsx)
	assertContains(t, out, "initState()")
	assertContains(t, out, "dispose()")
	assertContains(t, out, "super.dispose();")
}

func TestListRendering(t *testing.T) {
	tsx := `
function ItemList() {
  return (
    <div>
      {items.map((item) => <span>{item}</span>)}
    </div>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "items.map((item) =>")
	assertContains(t, out, ".toList()")
}

func TestIconWithTestId(t *testing.T) {
	tsx := `
function StatusIcon() {
  return (
    <Icon name="check" data-testid="check-icon" />
  );
}`
	out := compile(tsx)
	assertContains(t, out, "Icon(Icons.check,")
	assertContains(t, out, "key: const Key('check-icon')")
}

func TestCamelCaseIconName(t *testing.T) {
	tsx := `
function ArrowIcon() {
  return (
    <Icon name="arrowBack" />
  );
}`
	out := compile(tsx)
	assertContains(t, out, "Icon(Icons.arrow_back)")
}

func TestImageWidget(t *testing.T) {
	tsx := `
function Photo() {
  return (
    <img src="https://example.com/img.png" alt="photo" />
  );
}`
	out := compile(tsx)
	assertContains(t, out, "NetworkImage('https://example.com/img.png')")
	assertContains(t, out, "semanticLabel: 'photo'")
}

func TestMultipleComponents(t *testing.T) {
	tsx := `
function Header() {
  return (<div><span>Header</span></div>);
}

function Footer() {
  return (<div><span>Footer</span></div>);
}
`
	out := compile(tsx)
	assertContains(t, out, "class Header extends StatelessWidget")
	assertContains(t, out, "class Footer extends StatelessWidget")
}

func TestTextFieldWithTestId(t *testing.T) {
	tsx := `
function SearchBox() {
  return (
    <input data-testid="search-input" onChange={handleSearch} placeholder="Search..." />
  );
}`
	out := compile(tsx)
	assertContains(t, out, "TextField(")
	assertContains(t, out, "key: const Key('search-input')")
	assertContains(t, out, "onChanged: handleSearch")
	assertContains(t, out, "hintText: 'Search...'")
}

func TestHeadingElements(t *testing.T) {
	tsx := `
function Title() {
  return (
    <h1>Welcome</h1>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "Text('Welcome')")
}

func TestBooleanStateVar(t *testing.T) {
	tsx := `
function Modal() {
  const [visible, setVisible] = useState(false);
  return (
    <div>{visible && <span>Modal content</span>}</div>
  );
}`
	out := compile(tsx)
	assertContains(t, out, "bool visible = false;")
}

func TestStringStateVar(t *testing.T) {
	tsx := `
function Greeter() {
  const [name, setName] = useState("World");
  return (
    <span>{name}</span>
  );
}`
	out := compile(tsx)
	assertContains(t, out, `String name = "World";`)
}
