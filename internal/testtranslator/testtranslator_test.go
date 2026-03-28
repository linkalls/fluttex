package testtranslator_test

import (
	"strings"
	"testing"

	"github.com/linkalls/fluttex/internal/testtranslator"
)

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain:\n  %q\ngot:\n%s", want, got)
	}
}

func TestBasicTestTranslation(t *testing.T) {
	src := `
import { render, screen, fireEvent } from '@testing-library/react';
import { App } from './App';

test('renders the app', async () => {
  render(<App />);
  expect(screen.getByTestId('main-container')).toBeInTheDocument();
});
`
	out := testtranslator.TranslateFile(src)
	assertContains(t, out, "import 'package:flutter_test/flutter_test.dart';")
	assertContains(t, out, "import 'app.dart';")
	assertContains(t, out, "testWidgets('renders the app'")
	assertContains(t, out, "await tester.pumpWidget(const MaterialApp(home: App()));")
	assertContains(t, out, "find.byKey(const Key('main-container'))")
	assertContains(t, out, "findsOneWidget")
}

func TestFireEventClickTranslation(t *testing.T) {
	src := `
import { render, screen, fireEvent } from '@testing-library/react';
import { Counter } from './Counter';

test('increments count on click', async () => {
  render(<Counter />);
  const btn = screen.getByTestId('increment-btn');
  fireEvent.click(btn);
  expect(screen.getByText('1')).toBeInTheDocument();
});
`
	out := testtranslator.TranslateFile(src)
	assertContains(t, out, "import 'counter.dart';")
	assertContains(t, out, "await tester.pumpWidget(const MaterialApp(home: Counter()));")
	assertContains(t, out, "find.byKey(const Key('increment-btn'))")
	assertContains(t, out, "await tester.tap(")
	assertContains(t, out, "await tester.pump();")
	assertContains(t, out, "find.text('1')")
}

func TestFireEventInputTranslation(t *testing.T) {
	src := `
import { render, screen, fireEvent } from '@testing-library/react';
import { LoginForm } from './LoginForm';

test('submits the form', async () => {
  render(<LoginForm />);
  const input = screen.getByTestId('email-input');
  fireEvent.change(input, { target: { value: 'user@example.com' } });
  expect(screen.getByTestId('email-input')).toBeInTheDocument();
});
`
	out := testtranslator.TranslateFile(src)
	assertContains(t, out, "await tester.enterText(")
	assertContains(t, out, "'user@example.com'")
	assertContains(t, out, "await tester.pump();")
}

func TestDescribeGroupTranslation(t *testing.T) {
	src := `
import { render, screen } from '@testing-library/react';
import { Button } from './Button';

describe('Button component', () => {
  test('renders with label', async () => {
    render(<Button />);
    expect(screen.getByTestId('btn')).toBeInTheDocument();
  });
});
`
	out := testtranslator.TranslateFile(src)
	assertContains(t, out, "group('Button component'")
	assertContains(t, out, "testWidgets('renders with label'")
	assertContains(t, out, "find.byKey(const Key('btn'))")
}

func TestWaitForTranslation(t *testing.T) {
	src := `
import { render, screen, waitFor } from '@testing-library/react';
import { AsyncComp } from './AsyncComp';

test('waits for async update', async () => {
  render(<AsyncComp />);
  await waitFor(() => {
    expect(screen.getByTestId('result')).toBeInTheDocument();
  });
});
`
	out := testtranslator.TranslateFile(src)
	assertContains(t, out, "await tester.pumpAndSettle();")
}

func TestGetByRoleButtonTranslation(t *testing.T) {
	src := `
import { render, screen } from '@testing-library/react';
import { MyWidget } from './MyWidget';

test('finds button by role', async () => {
  render(<MyWidget />);
  expect(screen.getByRole('button')).toBeInTheDocument();
});
`
	out := testtranslator.TranslateFile(src)
	assertContains(t, out, "find.byType(ElevatedButton)")
	assertContains(t, out, "findsOneWidget")
}

func TestVoidMainWrapper(t *testing.T) {
	src := `
test('simple', async () => {
  render(<App />);
});
`
	out := testtranslator.TranslateFile(src)
	assertContains(t, out, "void main() {")
	assertContains(t, out, "}")
}
