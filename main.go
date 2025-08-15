// ABOUTME: Entry point for git-rebase-extract-file command
// ABOUTME: Handles CLI parsing and delegates to core rebase logic

// Package main provides the CLI interface for git-rebase-extract-file
package main

import (
	"fmt"
	"os"

	"github.com/obra/git-rebase-extract-file/internal/rebase"
	"github.com/spf13/cobra"
)

var (
	dryRun bool
	debug  bool
)

var rootCmd = &cobra.Command{
	Use:   "git-rebase-extract-file [--dry-run] [--debug] <previous-rev> <file-path> [file-path...]",
	Short: "Split commits by extracting changes to specified files/directories",
	Long: `git-rebase-extract-file performs an interactive rebase that automatically
splits commits containing changes to specified files or directories. The changes to the target
files are extracted into separate commits while preserving all original metadata.

Examples:
  git-rebase-extract-file main~5 src/component.tsx
  git-rebase-extract-file main~5 src/component1.tsx src/component2.tsx
  git-rebase-extract-file main~5 src/components/ lib/utils.ts`,
	Args: cobra.MinimumNArgs(2),
	RunE: run,
}

func init() {
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be done without making changes")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable detailed debug output")
}

func run(_ *cobra.Command, args []string) error {
	previousRev := args[0]
	filePaths := args[1:]

	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	extractor := rebase.NewExtractor(wd, filePaths...)
	extractor.SetDebug(debug)

	if dryRun {
		output, err := extractor.DryRun(previousRev, "HEAD")
		if err != nil {
			return fmt.Errorf("dry run failed: %w", err)
		}
		fmt.Print(output)
		return nil
	}

	return extractor.Extract(previousRev, "HEAD")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
