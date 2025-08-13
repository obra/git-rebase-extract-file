// ABOUTME: Core tests for the rebase extract file functionality
// ABOUTME: Tests commit analysis, splitting logic, and dry-run output

package rebase

import (
	"strings"
	"testing"

	"github.com/obra/git-rebase-extract-file/internal/testutils"
)

func TestAnalyzeCommits_NoTargetFileChanges(t *testing.T) {
	repo := testutils.NewTestRepo(t)

	// Create commits that don't touch target file
	repo.WriteFile("main.go", "package main\n")
	commit1 := repo.Commit("Initial commit")

	repo.WriteFile("other.go", "package other\n")
	repo.Commit("Add other file")

	analyzer := NewAnalyzer(repo.Dir, "target.txt")
	commits, err := analyzer.AnalyzeRange(commit1, "HEAD")

	if err != nil {
		t.Fatalf("AnalyzeRange failed: %v", err)
	}

	// Should find no commits to split
	splitCount := 0
	for _, commit := range commits {
		if commit.NeedsSplit {
			splitCount++
		}
	}

	if splitCount != 0 {
		t.Errorf("Expected 0 commits to split, got %d", splitCount)
	}
}

func TestAnalyzeCommits_TargetFileOnly(t *testing.T) {
	repo := testutils.NewTestRepo(t)

	// Create initial commit
	repo.WriteFile("main.go", "package main\n")
	baseCommit := repo.Commit("Initial commit")

	// Create commit with only target file
	repo.WriteFile("target.txt", "content")
	repo.Commit("Update target file only")

	analyzer := NewAnalyzer(repo.Dir, "target.txt")
	commits, err := analyzer.AnalyzeRange(baseCommit, "HEAD")

	if err != nil {
		t.Fatalf("AnalyzeRange failed: %v", err)
	}

	// Should find one commit, but it shouldn't need splitting
	if len(commits) != 1 {
		t.Fatalf("Expected 1 commit, got %d", len(commits))
	}

	if commits[0].NeedsSplit {
		t.Error("Commit with only target file should not need splitting")
	}
}

func TestAnalyzeCommits_TargetFileWithOthers(t *testing.T) {
	repo := testutils.NewTestRepo(t)

	// Create initial commit
	repo.WriteFile("main.go", "package main\n")
	baseCommit := repo.Commit("Initial commit")

	// Create commit with target file AND other files
	repo.WriteFile("target.txt", "content")
	repo.WriteFile("other.go", "package other\n")
	repo.Commit("Update multiple files")

	analyzer := NewAnalyzer(repo.Dir, "target.txt")
	commits, err := analyzer.AnalyzeRange(baseCommit, "HEAD")

	if err != nil {
		t.Fatalf("AnalyzeRange failed: %v", err)
	}

	// Should find one commit that needs splitting
	if len(commits) != 1 {
		t.Fatalf("Expected 1 commit, got %d", len(commits))
	}

	if !commits[0].NeedsSplit {
		t.Error("Commit with target file + others should need splitting")
	}
}

func TestDryRun_Output(t *testing.T) {
	repo := testutils.NewTestRepo(t)

	// Setup commits
	repo.WriteFile("main.go", "package main\n")
	baseCommit := repo.Commit("Initial commit")

	repo.WriteFile("target.txt", "content")
	repo.WriteFile("other.go", "package other\n")
	repo.Commit("Fix user authentication bug")

	// Test dry run
	extractor := NewExtractor(repo.Dir, "target.txt")
	output, err := extractor.DryRun(baseCommit, "HEAD")

	if err != nil {
		t.Fatalf("DryRun failed: %v", err)
	}

	// Verify output format
	expectedParts := []string{
		"Would split 1 out of 1 commits:",
		"Fix user authentication bug",
		"Changes to target.txt split into a separate commit",
		"target.txt: Fix user authentication bug",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Expected dry run output to contain '%s', got:\n%s", part, output)
		}
	}
}

func TestCommitMessageGeneration(t *testing.T) {
	tests := []struct {
		name         string
		original     string
		targetFile   string
		expectFirst  string
		expectSecond string
	}{
		{
			name:         "simple message",
			original:     "Fix user authentication bug",
			targetFile:   "target.txt",
			expectFirst:  "Fix user authentication bug\n\nChanges to target.txt split into a separate commit",
			expectSecond: "target.txt: Fix user authentication bug",
		},
		{
			name:         "multiline message",
			original:     "Fix user authentication bug\n\nThis fixes issue #123",
			targetFile:   "src/auth.go",
			expectFirst:  "Fix user authentication bug\n\nThis fixes issue #123\n\nChanges to src/auth.go split into a separate commit",
			expectSecond: "src/auth.go: Fix user authentication bug\n\nThis fixes issue #123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, second := GenerateSplitMessages(tt.original, tt.targetFile)

			if first != tt.expectFirst {
				t.Errorf("First message mismatch:\nExpected: %q\nGot: %q", tt.expectFirst, first)
			}

			if second != tt.expectSecond {
				t.Errorf("Second message mismatch:\nExpected: %q\nGot: %q", tt.expectSecond, second)
			}
		})
	}
}

func TestExtractFile_ActualRebase(t *testing.T) {
	repo := testutils.NewTestRepo(t)

	// Create initial commit
	repo.WriteFile("main.go", "package main\n")
	baseCommit := repo.Commit("Initial commit")

	// Create commit with target file AND other files
	repo.WriteFile("target.txt", "original content")
	repo.WriteFile("other.go", "package other\n")
	repo.Commit("Fix user authentication bug")

	// Create another regular commit
	repo.WriteFile("main.go", "package main\n\nfunc main() {}\n")
	repo.Commit("Add main function")

	// Perform the extraction
	extractor := NewExtractor(repo.Dir, "target.txt")
	err := extractor.Extract(baseCommit, "HEAD")

	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify the result: should have 4 commits total
	// 1. Initial commit (unchanged)
	// 2. "Fix user authentication bug" + split notice (other.go changes)
	// 3. "target.txt: Fix user authentication bug" (target.txt changes)
	// 4. "Add main function" (unchanged)

	analyzer := NewAnalyzer(repo.Dir, "target.txt")
	commits, err := analyzer.AnalyzeRange(baseCommit, "HEAD")
	if err != nil {
		t.Fatalf("Failed to analyze result: %v", err)
	}

	if len(commits) != 3 { // baseCommit not included in range
		t.Fatalf("Expected 3 commits after extraction, got %d", len(commits))
	}

	// Check that no commits need splitting anymore
	for i, commit := range commits {
		if commit.NeedsSplit {
			t.Errorf("Commit %d still needs splitting after extraction: %s", i, commit.Hash)
		}
	}

	// Verify the split commit messages
	if !strings.Contains(commits[0].Message, "Changes to target.txt split into a separate commit") {
		t.Errorf("First split commit missing split notice: %s", commits[0].Message)
	}

	if !strings.HasPrefix(commits[1].Message, "target.txt: ") {
		t.Errorf("Second split commit missing file prefix: %s", commits[1].Message)
	}
}

