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
)

var rootCmd = &cobra.Command{
	Use:   "git-rebase-extract-file [--dry-run] <previous-rev> <file-path>",
	Short: "Split commits by extracting changes to a specific file",
	Long: `git-rebase-extract-file performs an interactive rebase that automatically
splits commits containing changes to a specified file. The changes to the target
file are extracted into separate commits while preserving all original metadata.`,
	Args: cobra.ExactArgs(2),
	RunE: run,
}

func init() {
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be done without making changes")
}

func run(_ *cobra.Command, args []string) error {
	previousRev := args[0]
	filePath := args[1]

	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	extractor := rebase.NewExtractor(wd, filePath)

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
