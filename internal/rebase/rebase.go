// ABOUTME: Core rebase logic for extracting file changes into separate commits
// ABOUTME: Provides commit analysis and splitting functionality

// Package rebase provides commit analysis and splitting functionality for git repositories.
package rebase

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommitInfo represents a commit and whether it needs splitting
type CommitInfo struct {
	Hash       string
	Message    string
	Author     string
	Files      []string
	NeedsSplit bool
}

// Analyzer analyzes commits to determine which need splitting
type Analyzer struct {
	repoDir    string
	targetFile string
}

// NewAnalyzer creates a new commit analyzer
func NewAnalyzer(repoDir, targetFile string) *Analyzer {
	return &Analyzer{
		repoDir:    repoDir,
		targetFile: targetFile,
	}
}

// AnalyzeRange analyzes commits in the given range
func (a *Analyzer) AnalyzeRange(from, to string) ([]CommitInfo, error) {
	// Get list of commits in range
	cmd := exec.Command("git", "rev-list", "--reverse", from+".."+to)
	cmd.Dir = a.repoDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit list: %w", err)
	}

	commitHashes := strings.Fields(strings.TrimSpace(string(output)))
	var commits []CommitInfo

	for _, hash := range commitHashes {
		commit, err := a.analyzeCommit(hash)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze commit %s: %w", hash, err)
		}
		commits = append(commits, commit)
	}

	return commits, nil
}

// analyzeCommit analyzes a single commit to determine if it needs splitting
func (a *Analyzer) analyzeCommit(hash string) (CommitInfo, error) {
	// Get commit message
	cmd := exec.Command("git", "log", "--format=%B", "-n", "1", hash)
	cmd.Dir = a.repoDir
	msgOutput, err := cmd.Output()
	if err != nil {
		return CommitInfo{}, fmt.Errorf("failed to get commit message: %w", err)
	}

	// Get files changed in commit
	cmd = exec.Command("git", "show", "--name-only", "--format=", hash)
	cmd.Dir = a.repoDir
	filesOutput, err := cmd.Output()
	if err != nil {
		return CommitInfo{}, fmt.Errorf("failed to get commit files: %w", err)
	}

	files := strings.Fields(strings.TrimSpace(string(filesOutput)))

	// Check if target file is in the list and if there are other files
	hasTargetFile := false
	hasOtherFiles := false

	for _, file := range files {
		if file == a.targetFile {
			hasTargetFile = true
		} else {
			hasOtherFiles = true
		}
	}

	return CommitInfo{
		Hash:       hash,
		Message:    strings.TrimSpace(string(msgOutput)),
		Files:      files,
		NeedsSplit: hasTargetFile && hasOtherFiles,
	}, nil
}

// Extractor handles the actual rebase and splitting
type Extractor struct {
	repoDir    string
	targetFile string
}

// NewExtractor creates a new commit extractor
func NewExtractor(repoDir, targetFile string) *Extractor {
	return &Extractor{
		repoDir:    repoDir,
		targetFile: targetFile,
	}
}

// DryRun shows what would be done without making changes
func (e *Extractor) DryRun(from, to string) (string, error) {
	analyzer := NewAnalyzer(e.repoDir, e.targetFile)
	commits, err := analyzer.AnalyzeRange(from, to)
	if err != nil {
		return "", fmt.Errorf("failed to analyze commits: %w", err)
	}

	// Count commits that need splitting
	splitCount := 0
	for _, commit := range commits {
		if commit.NeedsSplit {
			splitCount++
		}
	}

	var output strings.Builder
	fmt.Fprintf(&output, "Would split %d out of %d commits:\n\n", splitCount, len(commits))

	// Show details for each commit that would be split
	for _, commit := range commits {
		if commit.NeedsSplit {
			firstMsg, secondMsg := GenerateSplitMessages(commit.Message, e.targetFile)

			// Show original commit and its splits
			fmt.Fprintf(&output, "Commit %s: \"%s\"\n", commit.Hash[:7], commit.Message)
			fmt.Fprintf(&output, "├─ Split into: \"%s\"\n", firstMsg)
			fmt.Fprintf(&output, "└─ Split into: \"%s\"\n\n", secondMsg)
		}
	}

	return output.String(), nil
}

// Extract performs the actual rebase with commit splitting
func (e *Extractor) Extract(from, to string) error {
	// Check for clean working directory
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = e.repoDir
	statusOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}
	if len(strings.TrimSpace(string(statusOutput))) > 0 {
		return fmt.Errorf("working directory is not clean. Please commit or stash changes first:\n%s", string(statusOutput))
	}

	// Capture original HEAD before making any changes
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = e.repoDir
	originalHeadOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get original HEAD: %w", err)
	}
	originalHead := strings.TrimSpace(string(originalHeadOutput))

	analyzer := NewAnalyzer(e.repoDir, e.targetFile)
	commits, err := analyzer.AnalyzeRange(from, to)
	if err != nil {
		return fmt.Errorf("failed to analyze commits: %w", err)
	}

	// Check if any commits need splitting
	needsWork := false
	for _, commit := range commits {
		if commit.NeedsSplit {
			needsWork = true
			break
		}
	}

	if !needsWork {
		fmt.Println("No commits need splitting")
		return nil
	}

	// Start interactive rebase
	err = e.performRebase(from, commits)
	if err != nil {
		return err
	}

	// Print revert instruction
	fmt.Printf("\n✅ Successfully extracted changes to %s into separate commits.\n", e.targetFile)
	fmt.Printf("To revert this branch to its previous state, run:\n")
	fmt.Printf("  git reset --hard %s\n\n", originalHead)

	return nil
}

// performRebase executes the git rebase with commit splitting
func (e *Extractor) performRebase(from string, commits []CommitInfo) error {
	// This is the complex part - we need to use git rebase --interactive
	// with a custom sequence that splits the necessary commits

	// For now, implement a simple approach using git cherry-pick
	// to rebuild the history with split commits

	// Get current branch name
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = e.repoDir
	branchOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	currentBranch := strings.TrimSpace(string(branchOutput))

	// Create backup branch
	backupBranch := currentBranch + "-backup-" + fmt.Sprintf("%d", os.Getpid())
	cmd = exec.Command("git", "branch", backupBranch)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create backup branch: %w", err)
	}

	// Reset to the base commit
	cmd = exec.Command("git", "reset", "--hard", from)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reset to base: %w", err)
	}

	// Replay commits, splitting as needed
	for _, commit := range commits {
		if err := e.replayCommit(commit); err != nil {
			return fmt.Errorf("failed to replay commit %s: %w", commit.Hash, err)
		}
	}

	return nil
}

// replayCommit replays a single commit, splitting if necessary
func (e *Extractor) replayCommit(commit CommitInfo) error {
	if !commit.NeedsSplit {
		// Simple cherry-pick for commits that don't need splitting
		cmd := exec.Command("git", "cherry-pick", commit.Hash)
		cmd.Dir = e.repoDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cherry-pick failed for commit %s: %w\n\nTo recover: git cherry-pick --abort", commit.Hash, err)
		}
		return nil
	}

	// Split the commit
	return e.splitCommit(commit)
}

// splitCommit splits a commit into two parts
func (e *Extractor) splitCommit(commit CommitInfo) error {
	// Apply all changes from the original commit
	cmd := exec.Command("git", "cherry-pick", "--no-commit", commit.Hash)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cherry-pick failed for commit %s: %w\n\nTo recover: git reset --hard HEAD", commit.Hash, err)
	}

	// Reset the target file to exclude it from the first commit
	cmd = exec.Command("git", "reset", "HEAD", e.targetFile)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reset target file: %w", err)
	}

	// Create first commit with everything except target file
	firstMsg, secondMsg := GenerateSplitMessages(commit.Message, e.targetFile)
	cmd = exec.Command("git", "commit", "-m", firstMsg)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create first split commit: %w", err)
	}

	// Add and commit the target file
	cmd = exec.Command("git", "add", e.targetFile)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stage target file: %w", err)
	}

	cmd = exec.Command("git", "commit", "-m", secondMsg)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create second split commit: %w", err)
	}

	return nil
}

// GenerateSplitMessages creates the two commit messages for a split
func GenerateSplitMessages(original, targetFile string) (string, string) {
	// First commit: original + split notice
	firstMsg := original + "\n\nChanges to " + targetFile + " split into a separate commit"

	// Second commit: prefixed original
	secondMsg := targetFile + ": " + original

	return firstMsg, secondMsg
}
