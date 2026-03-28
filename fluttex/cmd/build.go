package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/microsoft/typescript-go/fluttex/pkg/compiler"
)

var buildCmd = &cobra.Command{
	Use:   "build [file.tsx]",
	Short: "Builds a TSX file into Dart",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputFile := args[0]

		b, err := os.ReadFile(inputFile)
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			os.Exit(1)
		}

		tr := compiler.NewTranspiler()
		out, err := tr.Transpile(string(b))
		if err != nil {
			fmt.Printf("Error transpiling: %v\n", err)
			os.Exit(1)
		}

		// Ensure lib directory exists
		os.MkdirAll("lib", 0755)

		baseName := filepath.Base(inputFile)
		ext := filepath.Ext(baseName)
		nameWithoutExt := baseName[0 : len(baseName)-len(ext)]
		outputFile := filepath.Join("lib", nameWithoutExt+".dart")

		err = os.WriteFile(outputFile, []byte(out), 0644)
		if err != nil {
			fmt.Printf("Error writing file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully compiled %s to %s\n", inputFile, outputFile)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
