package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch <input.tsx>",
	Short: "Watch a TSX file and recompile on changes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]
		return watchFile(inputPath)
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
}

func watchFile(inputPath string) error {
	fmt.Fprintf(os.Stdout, "Watching %s ...\n", inputPath)

	var lastMod time.Time

	// Get initial modification time
	if info, err := os.Stat(inputPath); err == nil {
		lastMod = info.ModTime()
	}

	// Initial build
	if err := buildFile(inputPath, ""); err != nil {
		fmt.Fprintf(os.Stderr, "build error: %v\n", err)
	}

	for {
		time.Sleep(500 * time.Millisecond)

		info, err := os.Stat(inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stat error: %v\n", err)
			continue
		}

		if info.ModTime().After(lastMod) {
			lastMod = info.ModTime()
			fmt.Fprintf(os.Stdout, "Change detected, rebuilding...\n")
			if err := buildFile(inputPath, ""); err != nil {
				fmt.Fprintf(os.Stderr, "build error: %v\n", err)
			}
		}
	}
}
