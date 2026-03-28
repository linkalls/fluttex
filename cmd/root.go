package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "fluttex",
	Short: "TSX to Flutter/Dart transpiler",
	Long: `fluttex compiles TypeScript/TSX source files into idiomatic
Flutter/Dart code compatible with the standard flutter_test framework.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
