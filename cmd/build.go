package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/linkalls/fluttex/internal/generator"
	"github.com/linkalls/fluttex/internal/parser"
	"github.com/spf13/cobra"
)

var outputFlag string

var buildCmd = &cobra.Command{
	Use:   "build <input.tsx>",
	Short: "Compile a TSX file to Flutter/Dart",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]
		return buildFile(inputPath, outputFlag)
	},
}

func init() {
	buildCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output .dart file path (default: same name as input)")
	rootCmd.AddCommand(buildCmd)
}

// buildFile reads, parses, generates, and writes the Dart output for a single TSX file.
func buildFile(inputPath, outPath string) error {
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", inputPath, err)
	}

	nodes := parser.ParseFile(string(src))
	dart := generator.Generate(nodes)

	if outPath == "" {
		base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		outPath = filepath.Join(filepath.Dir(inputPath), base+".dart")
	}

	if err := os.WriteFile(outPath, []byte(dart), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	fmt.Fprintf(os.Stdout, "✓ Generated %s\n", outPath)

	// Run dart format if available
	if dartPath, err := exec.LookPath("dart"); err == nil {
		fmtCmd := exec.Command(dartPath, "format", outPath)
		fmtCmd.Stdout = os.Stdout
		fmtCmd.Stderr = os.Stderr
		_ = fmtCmd.Run() // non-fatal if dart format fails
	}

	return nil
}
