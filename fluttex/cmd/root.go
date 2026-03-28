package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "fluttex",
	Short: "fluttex compiles TSX code into Flutter Dart code",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("fluttex CLI")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
