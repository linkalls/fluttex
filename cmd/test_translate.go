package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/linkalls/fluttex/internal/testtranslator"
	"github.com/spf13/cobra"
)

var testOutputFlag string

var testCmd = &cobra.Command{
	Use:   "test <input.test.tsx>",
	Short: "Translate a Jest/Vitest TSX test file to a Dart widget test",
	Long: `Translates a Jest/Vitest TSX test file (e.g. App.test.tsx) into a Dart
flutter_test compatible file (e.g. app_test.dart) so that native Flutter
widget tests can be run on TSX-authored projects.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]
		return translateTestFile(inputPath, testOutputFlag)
	},
}

func init() {
	testCmd.Flags().StringVarP(&testOutputFlag, "output", "o", "", "Output _test.dart file path (default: derived from input name)")
	rootCmd.AddCommand(testCmd)
}

func translateTestFile(inputPath, outPath string) error {
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", inputPath, err)
	}

	dart := testtranslator.TranslateFile(string(src))

	if outPath == "" {
		base := filepath.Base(inputPath)
		// Remove .test.tsx, .spec.tsx, .test.ts, .spec.ts suffixes
		for _, suffix := range []string{".test.tsx", ".spec.tsx", ".test.ts", ".spec.ts"} {
			if strings.HasSuffix(base, suffix) {
				base = strings.TrimSuffix(base, suffix)
				break
			}
		}
		// snake_case the name and append _test.dart
		snakeName := toSnakeCase(base)
		outPath = filepath.Join(filepath.Dir(inputPath), snakeName+"_test.dart")
	}

	if err := os.WriteFile(outPath, []byte(dart), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	fmt.Fprintf(os.Stdout, "✓ Generated %s\n", outPath)
	return nil
}

// toSnakeCase converts a PascalCase or camelCase string to snake_case.
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, []rune(strings.ToLower(string(r)))...)
	}
	return string(result)
}
