package compiler

import (
	"testing"
	"strings"
)

func stripWhitespace(s string) string {
    return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "\n", ""), "\t", "")
}

func TestTranspileTable(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		contains []string
	}{
		{
			name: "Basic View and Text",
			source: `
				export function App() {
					return (
						<View>
							<Text>Hello</Text>
						</View>
					);
				}
			`,
			contains: []string{"classAppextendsStatelessWidget", "Container(child:constText('Hello'))"},
		},
		{
			name: "Button with TestID",
			source: `
				export function App() {
					return (
						<button data-testid="my-btn" onClick={() => {}}>Click</button>
					);
				}
			`,
			contains: []string{"ElevatedButton(key:constKey('my-btn')", "child:constText('Click')"},
		},
		{
			name: "Stateful Counter",
			source: `
				export function App() {
					const [count, setCount] = useState(0);

					return (
						<View>
							<button data-testid="my-btn" onClick={() => setCount(count + 1)}>
								Click
							</button>
						</View>
					);
				}
			`,
			contains: []string{
				"classAppextendsStatefulWidget",
				"intcount=0;",
				"setState((){count=count+1;})",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTranspiler()
			out, err := tr.Transpile(tt.source)
			if err != nil {
				t.Fatalf("Transpile failed: %v", err)
			}

            strippedOut := stripWhitespace(out)
			for _, check := range tt.contains {
				if !strings.Contains(strippedOut, stripWhitespace(check)) {
					t.Errorf("Expected output to contain '%s'\nOutput: \n%s", check, out)
				}
			}
		})
	}
}
