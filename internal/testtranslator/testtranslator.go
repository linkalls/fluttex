// Package testtranslator converts Jest/Vitest TSX test files into Dart widget test files.
// It maps common testing-library patterns to their flutter_test equivalents.
package testtranslator

import (
"fmt"
"regexp"
"strings"
)

// TranslateFile translates a Jest/Vitest test source string to a Dart widget test file.
func TranslateFile(src string) string {
src = strings.TrimSpace(src)

var sb strings.Builder
sb.WriteString("import 'package:flutter/material.dart';\n")
sb.WriteString("import 'package:flutter_test/flutter_test.dart';\n")

importedComponents := extractImportedComponents(src)
for _, comp := range importedComponents {
snakeName := camelToSnake(comp)
fmt.Fprintf(&sb, "import '%s.dart';\n", snakeName)
}

sb.WriteString("\n")
sb.WriteString("void main() {\n")

// Translate describe blocks first
remaining := translateDescribeBlocks(src, &sb)

// Translate top-level test/it blocks
for _, blk := range findTestBlocks(remaining) {
emitTestBlock(blk.desc, blk.body, &sb, 1)
}

sb.WriteString("}\n")

return sb.String()
}

// ── Pattern regexes ──────────────────────────────────────────────────────────

var (
importRe = regexp.MustCompile(`import\s+\{([^}]+)\}\s+from\s+['"]([^'"]+)['"]`)

describeSingleStartRe = regexp.MustCompile(`describe\s*\(\s*'([^']*)'\s*,\s*\(\s*\)\s*=>\s*\{`)
describeDoubleStartRe = regexp.MustCompile(`describe\s*\(\s*"([^"]*)"\s*,\s*\(\s*\)\s*=>\s*\{`)

testSingleStartRe = regexp.MustCompile(`(?:test|it)\s*\(\s*'([^']*)'\s*,\s*async\s*\(\s*\)\s*=>\s*\{`)
testDoubleStartRe = regexp.MustCompile(`(?:test|it)\s*\(\s*"([^"]*)"\s*,\s*async\s*\(\s*\)\s*=>\s*\{`)

renderRe = regexp.MustCompile(`(?:const\s+\{[^}]*\}\s*=\s*)?render\(\s*(<[^>]+>(?:.*?</[A-Za-z]+>)?)\s*\)`)

getByTestIdRe   = regexp.MustCompile(`screen\.getByTestId\(['"]([^'"]+)['"]\)`)
queryByTestIdRe = regexp.MustCompile(`screen\.queryByTestId\(['"]([^'"]+)['"]\)`)
getByTextRe     = regexp.MustCompile(`screen\.getByText\(['"]([^'"]+)['"]\)`)
getByRoleRe     = regexp.MustCompile(`screen\.getByRole\(['"]([^'"]+)['"]\)`)
findByTestIdRe  = regexp.MustCompile(`await\s+screen\.findByTestId\(['"]([^'"]+)['"]\)`)

fireEventClickRe = regexp.MustCompile(`fireEvent\.click\((.+?)\)`)
fireEventInputRe = regexp.MustCompile(`fireEvent\.change\((.+?),\s*\{\s*target:\s*\{\s*value:\s*['"]([^'"]*)['"]\s*\}\s*\}\s*\)`)

// Use lazy (.+?) to allow nested parens in the argument
expectTextRe    = regexp.MustCompile(`expect\((.+?)\)\.toBeInTheDocument\(\)`)
expectNotTextRe = regexp.MustCompile(`expect\((.+?)\)\.not\.toBeInTheDocument\(\)`)
expectTextValRe = regexp.MustCompile(`expect\((.+?)\)\.toHaveTextContent\(['"]([^'"]+)['"]\)`)
expectEqualRe   = regexp.MustCompile(`expect\((.+?)\)\.toBe\((.+?)\)`)
expectTrueRe    = regexp.MustCompile(`expect\((.+?)\)\.toBeTruthy\(\)`)
expectFalseRe   = regexp.MustCompile(`expect\((.+?)\)\.toBeFalsy\(\)`)

waitForRe = regexp.MustCompile(`await\s+waitFor\(`)

finderVarRe = regexp.MustCompile(`const\s+(\w+)\s*=\s*(screen\.\w+\([^)]*\))`)
)

// testBlock holds a parsed test description and body.
type testBlock struct {
desc string
body string
}

// extractImportedComponents returns PascalCase identifiers imported from relative paths.
func extractImportedComponents(src string) []string {
var components []string
seen := map[string]bool{}
for _, m := range importRe.FindAllStringSubmatch(src, -1) {
from := m[2]
if strings.HasPrefix(from, ".") {
for _, name := range strings.Split(m[1], ",") {
name = strings.TrimSpace(name)
if name != "" && name[0] >= 'A' && name[0] <= 'Z' && !seen[name] {
components = append(components, name)
seen[name] = true
}
}
}
}
return components
}

// findTestBlocks finds all test/it blocks in src using brace-counting body extraction.
func findTestBlocks(src string) []testBlock {
var blocks []testBlock
for _, re := range []*regexp.Regexp{testSingleStartRe, testDoubleStartRe} {
locs := re.FindAllStringSubmatchIndex(src, -1)
for _, loc := range locs {
desc := src[loc[2]:loc[3]]
// The '{' is the last character of the full match
braceStart := loc[1] - 1
body, _ := extractBraceBody(src, braceStart)
blocks = append(blocks, testBlock{desc: desc, body: body})
}
}
return blocks
}

// findDescribeBlocks finds all describe blocks in src using brace-counting body extraction.
func findDescribeBlocks(src string) []testBlock {
var blocks []testBlock
for _, re := range []*regexp.Regexp{describeSingleStartRe, describeDoubleStartRe} {
locs := re.FindAllStringSubmatchIndex(src, -1)
for _, loc := range locs {
desc := src[loc[2]:loc[3]]
braceStart := loc[1] - 1
body, _ := extractBraceBody(src, braceStart)
blocks = append(blocks, testBlock{desc: desc, body: body})
}
}
return blocks
}

// extractBraceBody extracts the content between matching '{' '}' starting at index start.
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

// translateDescribeBlocks converts describe blocks to group blocks.
func translateDescribeBlocks(src string, sb *strings.Builder) string {
remaining := src
for _, blk := range findDescribeBlocks(src) {
fmt.Fprintf(sb, "  group('%s', () {\n", escapeSingleQuote(blk.desc))
for _, testBlk := range findTestBlocks(blk.body) {
emitTestBlock(testBlk.desc, testBlk.body, sb, 2)
}
sb.WriteString("  });\n\n")
remaining = strings.Replace(remaining, blk.desc, "", 1)
}
return remaining
}

// emitTestBlock writes a testWidgets block to sb.
func emitTestBlock(desc, body string, sb *strings.Builder, indentLevel int) {
prefix := strings.Repeat("  ", indentLevel)
fmt.Fprintf(sb, "%stestWidgets('%s', (WidgetTester tester) async {\n", prefix, escapeSingleQuote(desc))
translateTestBody(body, sb, indentLevel+1)
fmt.Fprintf(sb, "%s});\n\n", prefix)
}

// translateTestBody translates the body of a single test block.
func translateTestBody(body string, sb *strings.Builder, indentLevel int) {
prefix := strings.Repeat("  ", indentLevel)

finderVars := map[string]string{}
for _, m := range finderVarRe.FindAllStringSubmatch(body, -1) {
finderVars[m[1]] = translateFinder(m[2])
}

lines := strings.Split(body, "\n")
for _, line := range lines {
trimmed := strings.TrimSpace(line)
if trimmed == "" {
continue
}
translated := translateStatement(trimmed, finderVars)
if translated == "" {
continue
}
for _, tLine := range strings.Split(translated, "\n") {
tLine = strings.TrimSpace(tLine)
if tLine != "" {
fmt.Fprintf(sb, "%s%s\n", prefix, tLine)
}
}
}
}

// translateStatement translates a single TSX test statement to Dart.
func translateStatement(line string, finderVars map[string]string) string {
if strings.HasPrefix(line, "import ") ||
strings.HasPrefix(line, "//") ||
strings.HasPrefix(line, "describe(") ||
strings.HasPrefix(line, "test(") ||
strings.HasPrefix(line, "it(") {
return ""
}

if m := renderRe.FindStringSubmatch(line); m != nil {
jsx := strings.TrimSpace(m[1])
compName := extractRootTagName(jsx)
return fmt.Sprintf("await tester.pumpWidget(const MaterialApp(home: %s()));", compName)
}

if m := finderVarRe.FindStringSubmatch(line); m != nil {
varName := m[1]
finderExpr := translateFinder(m[2])
return fmt.Sprintf("final %s = %s;", varName, finderExpr)
}

if m := fireEventClickRe.FindStringSubmatch(line); m != nil {
target := strings.TrimSpace(m[1])
finder := resolveFinderOrVar(target, finderVars)
return fmt.Sprintf("await tester.tap(%s);\nawait tester.pump();", finder)
}

if m := fireEventInputRe.FindStringSubmatch(line); m != nil {
target := strings.TrimSpace(m[1])
value := m[2]
finder := resolveFinderOrVar(target, finderVars)
return fmt.Sprintf("await tester.enterText(%s, '%s');\nawait tester.pump();", finder, escapeSingleQuote(value))
}

if waitForRe.MatchString(line) {
return "await tester.pumpAndSettle();"
}

// Check not.toBeInTheDocument before toBeInTheDocument
if m := expectNotTextRe.FindStringSubmatch(line); m != nil {
finderExpr := resolveFinderOrVar(strings.TrimSpace(m[1]), finderVars)
return fmt.Sprintf("expect(%s, findsNothing);", finderExpr)
}

if m := expectTextRe.FindStringSubmatch(line); m != nil {
finderExpr := resolveFinderOrVar(strings.TrimSpace(m[1]), finderVars)
return fmt.Sprintf("expect(%s, findsOneWidget);", finderExpr)
}

if m := expectTextValRe.FindStringSubmatch(line); m != nil {
text := m[2]
return fmt.Sprintf("expect(find.text('%s'), findsOneWidget);", escapeSingleQuote(text))
}

if m := expectEqualRe.FindStringSubmatch(line); m != nil {
actual := resolveFinderOrVar(strings.TrimSpace(m[1]), finderVars)
expected := strings.TrimSpace(m[2])
return fmt.Sprintf("expect(%s, %s);", actual, expected)
}

if m := expectTrueRe.FindStringSubmatch(line); m != nil {
actual := resolveFinderOrVar(strings.TrimSpace(m[1]), finderVars)
return fmt.Sprintf("expect(%s, isTrue);", actual)
}

if m := expectFalseRe.FindStringSubmatch(line); m != nil {
actual := resolveFinderOrVar(strings.TrimSpace(m[1]), finderVars)
return fmt.Sprintf("expect(%s, isFalse);", actual)
}

return ""
}

// translateFinder converts a screen.getByXxx call to a Dart find expression.
func translateFinder(expr string) string {
expr = strings.TrimSpace(expr)

if m := getByTestIdRe.FindStringSubmatch(expr); m != nil {
return fmt.Sprintf("find.byKey(const Key('%s'))", escapeSingleQuote(m[1]))
}
if m := queryByTestIdRe.FindStringSubmatch(expr); m != nil {
return fmt.Sprintf("find.byKey(const Key('%s'))", escapeSingleQuote(m[1]))
}
if m := findByTestIdRe.FindStringSubmatch(expr); m != nil {
return fmt.Sprintf("find.byKey(const Key('%s'))", escapeSingleQuote(m[1]))
}
if m := getByTextRe.FindStringSubmatch(expr); m != nil {
return fmt.Sprintf("find.text('%s')", escapeSingleQuote(m[1]))
}
if m := getByRoleRe.FindStringSubmatch(expr); m != nil {
role := strings.ToLower(m[1])
switch role {
case "button":
return "find.byType(ElevatedButton)"
case "textbox":
return "find.byType(TextField)"
default:
return fmt.Sprintf("find.byKey(const Key('%s'))", escapeSingleQuote(m[1]))
}
}
return expr
}

// resolveFinderOrVar resolves a target expression against known finder variables.
func resolveFinderOrVar(target string, finderVars map[string]string) string {
if finder, ok := finderVars[target]; ok {
return finder
}
translated := translateFinder(target)
if translated != target {
return translated
}
return target
}

// extractRootTagName extracts the tag name from a JSX element string.
func extractRootTagName(jsx string) string {
re := regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9._]*)`)
m := re.FindStringSubmatch(strings.TrimSpace(jsx))
if m != nil {
return m[1]
}
return "Widget"
}

// camelToSnake converts PascalCase/camelCase to snake_case.
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

func escapeSingleQuote(s string) string {
return strings.ReplaceAll(s, "'", `\'`)
}
