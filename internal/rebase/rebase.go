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

	// Capture original HEAD for recovery instructions and print them immediately
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = e.repoDir
	headOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current HEAD: %w", err)
	}
	originalHead := strings.TrimSpace(string(headOutput))
	
	// Print recovery instructions at the start so user knows how to get back
	fmt.Printf("To recover the repository state: git reset --hard %s\n", originalHead)

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

	// Check for potential conflicts before starting
	if conflicts := e.checkPotentialConflicts(from); len(conflicts) > 0 {
		fmt.Printf("⚠️  Warning: Potential conflicts detected in:\n")
		for _, conflict := range conflicts {
			fmt.Printf("  - %s\n", conflict)
		}
		fmt.Printf("\nThese files have been modified in multiple commits and may cause conflicts.\n")
		fmt.Printf("Consider resolving manually if the rebase fails.\n\n")
	}

	// Perform the rebase with splitting
	if err := e.performRebase(from, commits); err != nil {
		fmt.Printf("\n🚨 Rebase failed. To recover:\n")
		fmt.Printf("  git reset --hard %s\n", originalHead)
		return fmt.Errorf("rebase failed: %w", err)
	}

	// Print success message with recovery info
	fmt.Printf("\n✅ Successfully split commits. If you need to revert:\n")
	fmt.Printf("  git reset --hard %s\n", originalHead)

	return nil
}

// performRebase executes the git rebase with commit splitting
func (e *Extractor) performRebase(from string, commits []CommitInfo) error {
	// Get current branch name for backup
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
	fmt.Printf("Created backup branch: %s\n", backupBranch)

	// Process each commit that needs splitting using proper interactive rebase
	// Work backwards through commits to maintain proper order
	for i := len(commits) - 1; i >= 0; i-- {
		commit := commits[i]
		if commit.NeedsSplit {
			if err := e.splitCommitUsingInteractiveRebase(commit, from); err != nil {
				return fmt.Errorf("failed to split commit %s: %w", commit.Hash, err)
			}
		}
	}

	return nil
}

// splitCommitUsingInteractiveRebase splits a buried commit using interactive rebase
func (e *Extractor) splitCommitUsingInteractiveRebase(commit CommitInfo, from string) error {
	// Create a custom rebase sequence that marks our target commit for editing
	// and picks all others
	sequenceFile := fmt.Sprintf("/tmp/rebase-sequence-%d", os.Getpid())
	defer os.Remove(sequenceFile)
	
	// Generate the rebase todo list
	cmd := exec.Command("git", "log", "--reverse", "--format=%H %s", from+"..HEAD")
	cmd.Dir = e.repoDir
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get commit list: %w", err)
	}
	
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var sequence []string
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		message := parts[1]
		
		if hash == commit.Hash {
			// Mark this commit for editing
			sequence = append(sequence, fmt.Sprintf("edit %s %s", hash[:7], message))
		} else {
			// Pick other commits normally
			sequence = append(sequence, fmt.Sprintf("pick %s %s", hash[:7], message))
		}
	}
	
	// Write the sequence file
	sequenceContent := strings.Join(sequence, "\n") + "\n"
	if err := os.WriteFile(sequenceFile, []byte(sequenceContent), 0644); err != nil {
		return fmt.Errorf("failed to write sequence file: %w", err)
	}
	
	// Create a simple sequence editor that uses our pre-written file
	editorScript := fmt.Sprintf("#!/bin/sh\ncp %s \"$1\"\n", sequenceFile)
	editorPath := fmt.Sprintf("/tmp/rebase-editor-%d.sh", os.Getpid())
	if err := os.WriteFile(editorPath, []byte(editorScript), 0755); err != nil {
		return fmt.Errorf("failed to create editor script: %w", err)
	}
	defer os.Remove(editorPath)
	
	// Start the interactive rebase
	cmd = exec.Command("git", "rebase", "-i", from)
	cmd.Dir = e.repoDir
	cmd.Env = append(os.Environ(), "GIT_SEQUENCE_EDITOR="+editorPath)
	
	if err := cmd.Run(); err != nil {
		// Check if we're in a rebase state with conflicts
		if isRebaseInProgress, conflictMsg := e.checkRebaseConflicts(); isRebaseInProgress {
			return fmt.Errorf("rebase stopped due to conflicts:\n%s\n\nTo resolve:\n1. Manually resolve conflicts in the affected files\n2. Run: git add <resolved-files>\n3. Run: git rebase --continue\n4. Or run: git rebase --abort to cancel", conflictMsg)
		}
		return fmt.Errorf("failed to start interactive rebase: %w", err)
	}
	
	// Check if rebase is still in progress (stopped at our edit point)
	if isRebaseInProgress, _ := e.checkRebaseConflicts(); isRebaseInProgress {
		// We're in edit mode, proceed with splitting
		if err := e.splitCurrentCommit(commit); err != nil {
			exec.Command("git", "rebase", "--abort").Run()
			return fmt.Errorf("failed to split commit during rebase: %w", err)
		}
	} else {
		// Rebase completed without stopping - this shouldn't happen with our edit command
		return fmt.Errorf("rebase completed unexpectedly without stopping for editing")
	}
	
	// Continue the rebase
	cmd = exec.Command("git", "rebase", "--continue")
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to continue rebase: %w", err)
	}
	
	return nil
}

// splitCurrentCommit splits the current commit during a rebase
func (e *Extractor) splitCurrentCommit(commit CommitInfo) error {
	// Reset the commit but keep the changes in the working directory
	cmd := exec.Command("git", "reset", "HEAD^")
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reset commit: %w", err)
	}
	
	firstMsg, secondMsg := GenerateSplitMessages(commit.Message, e.targetFile)

	// Stage all files except the target file
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	// Unstage the target file
	cmd = exec.Command("git", "reset", "HEAD", e.targetFile)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unstage target file: %w", err)
	}

	// Create first commit (everything except target file)
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

// splitHeadCommit splits the HEAD commit
func (e *Extractor) splitHeadCommit(commit CommitInfo) error {
	// Reset the commit but keep changes in working directory
	cmd := exec.Command("git", "reset", "--soft", "HEAD~1")
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reset HEAD commit: %w", err)
	}

	firstMsg, secondMsg := GenerateSplitMessages(commit.Message, e.targetFile)

	// Stage all files except the target file
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	// Unstage the target file
	cmd = exec.Command("git", "reset", "HEAD", e.targetFile)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unstage target file: %w", err)
	}

	// Create first commit (everything except target file)
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

// checkRebaseConflicts checks if we're in a rebase state and returns conflict information
func (e *Extractor) checkRebaseConflicts() (bool, string) {
	// Check if rebase is in progress by looking for .git/rebase-merge directory
	rebaseMergeDir := fmt.Sprintf("%s/.git/rebase-merge", e.repoDir)
	if _, err := os.Stat(rebaseMergeDir); os.IsNotExist(err) {
		return false, ""
	}

	// Get status to check for conflicts
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = e.repoDir
	output, err := cmd.Output()
	if err != nil {
		return true, "Unable to check git status"
	}

	status := strings.TrimSpace(string(output))
	if status == "" {
		return true, "Rebase in progress - ready for editing"
	}

	// Look for conflict markers in status
	lines := strings.Split(status, "\n")
	var conflicts []string
	var staged []string
	
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		
		// Parse git status format: XY filename
		statusCode := line[:2]
		filename := line[3:]
		
		if strings.Contains(statusCode, "U") || statusCode == "AA" || statusCode == "DD" {
			conflicts = append(conflicts, filename)
		} else if statusCode[0] != ' ' && statusCode[0] != '?' {
			staged = append(staged, filename)
		}
	}

	if len(conflicts) > 0 {
		return true, fmt.Sprintf("Merge conflicts in: %s", strings.Join(conflicts, ", "))
	}
	
	if len(staged) > 0 {
		return true, fmt.Sprintf("Changes ready to commit: %s", strings.Join(staged, ", "))
	}
	
	return true, "Rebase in progress"
}

// checkPotentialConflicts identifies files that might cause conflicts during rebase
func (e *Extractor) checkPotentialConflicts(from string) []string {
	// Get all files modified in the range
	cmd := exec.Command("git", "log", "--name-only", "--format=", from+"..HEAD")
	cmd.Dir = e.repoDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	// Count occurrences of each file
	fileCount := make(map[string]int)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			fileCount[line]++
		}
	}

	// Find files modified in multiple commits
	var potentialConflicts []string
	for file, count := range fileCount {
		if count > 1 {
			potentialConflicts = append(potentialConflicts, file)
		}
	}

	return potentialConflicts
}

