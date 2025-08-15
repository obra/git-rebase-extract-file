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
	repoDir     string
	targetFiles []string
}

// NewAnalyzer creates a new commit analyzer
func NewAnalyzer(repoDir string, targetFiles ...string) *Analyzer {
	return &Analyzer{
		repoDir:     repoDir,
		targetFiles: targetFiles,
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
	// Get commit message and author
	cmd := exec.Command("git", "log", "--format=%B", "-n", "1", hash)
	cmd.Dir = a.repoDir
	msgOutput, err := cmd.Output()
	if err != nil {
		return CommitInfo{}, fmt.Errorf("failed to get commit message: %w", err)
	}

	// Get author information
	cmd = exec.Command("git", "log", "--format=%an <%ae>", "-n", "1", hash)
	cmd.Dir = a.repoDir
	authorOutput, err := cmd.Output()
	if err != nil {
		return CommitInfo{}, fmt.Errorf("failed to get commit author: %w", err)
	}

	// Get files changed in commit
	cmd = exec.Command("git", "show", "--name-only", "--format=", hash)
	cmd.Dir = a.repoDir
	filesOutput, err := cmd.Output()
	if err != nil {
		return CommitInfo{}, fmt.Errorf("failed to get commit files: %w", err)
	}

	files := strings.Fields(strings.TrimSpace(string(filesOutput)))

	// Check if any target files are in the list and if there are other files
	hasTargetFile := false
	hasOtherFiles := false

	for _, file := range files {
		if a.isTargetFile(file) {
			hasTargetFile = true
		} else {
			hasOtherFiles = true
		}
	}

	return CommitInfo{
		Hash:       hash,
		Message:    strings.TrimSpace(string(msgOutput)),
		Author:     strings.TrimSpace(string(authorOutput)),
		Files:      files,
		NeedsSplit: hasTargetFile && hasOtherFiles,
	}, nil
}

// isTargetFile checks if a file matches any of the target file patterns
func (a *Analyzer) isTargetFile(file string) bool {
	for _, target := range a.targetFiles {
		// Exact match
		if file == target {
			return true
		}
		// Directory prefix match (e.g., "src/" matches "src/component.tsx")
		if strings.HasSuffix(target, "/") && strings.HasPrefix(file, target) {
			return true
		}
	}
	return false
}

// Extractor handles the actual rebase and splitting
type Extractor struct {
	repoDir     string
	targetFiles []string
	debug       bool
}

// NewExtractor creates a new commit extractor
func NewExtractor(repoDir string, targetFiles ...string) *Extractor {
	return &Extractor{
		repoDir:     repoDir,
		targetFiles: targetFiles,
		debug:       false,
	}
}

// SetDebug enables or disables debug output
func (e *Extractor) SetDebug(debug bool) {
	e.debug = debug
}

// debugf prints debug output if debug mode is enabled
func (e *Extractor) debugf(format string, args ...interface{}) {
	if e.debug {
		fmt.Printf("🔧 DEBUG: "+format, args...)
	}
}

// DryRun shows what would be done without making changes
func (e *Extractor) DryRun(from, to string) (string, error) {
	analyzer := NewAnalyzer(e.repoDir, e.targetFiles...)
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
			firstMsg, secondMsg := GenerateSplitMessages(commit.Message, e.targetFiles)

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

	analyzer := NewAnalyzer(e.repoDir, e.targetFiles...)
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
	e.debugf("Starting to split commit %s\n", commit.Hash[:7])
	
	// Reset the commit but keep the changes in the working directory
	e.debugf("Resetting commit to HEAD^\n")
	cmd := exec.Command("git", "reset", "HEAD^")
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reset commit: %w", err)
	}
	
	// Show what's in working directory after reset
	e.debugGitStatus("After resetting commit")
	
	firstMsg, secondMsg := GenerateSplitMessages(commit.Message, e.targetFiles)

	// Stage all files except the target files
	e.debugf("Staging all files with 'git add .'\n")
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	// Show what's staged after add .
	e.debugGitStatus("After staging all files")

	// Unstage the target files
	e.debugf("Unstaging target files: %v\n", e.targetFiles)
	for _, targetFile := range e.targetFiles {
		e.debugf("Running 'git reset HEAD %s'\n", targetFile)
		cmd = exec.Command("git", "reset", "HEAD", targetFile)
		cmd.Dir = e.repoDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			e.debugf("Reset failed for %s: %v, output: %s\n", targetFile, err, string(output))
			// Continue anyway - file might not be staged
			continue
		}
		e.debugf("Reset successful for %s, output: %s\n", targetFile, string(output))
	}

	// Show what's staged after unstaging target files
	e.debugGitStatus("After unstaging target files")

	// Create first commit (everything except target files)
	e.debugf("Creating first commit with message: %q\n", firstMsg)
	e.debugf("Preserving author: %s\n", commit.Author)
	cmd = exec.Command("git", "commit", "-m", firstMsg, "--author", commit.Author)
	cmd.Dir = e.repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.debugf("First commit failed: %v, output: %s\n", err, string(output))
		return fmt.Errorf("failed to create first split commit: %w, output: %s", err, string(output))
	}
	e.debugf("First commit successful, output: %s\n", string(output))

	// Show repo state after first commit
	e.debugGitStatus("After first commit")

	// Add the target files back
	e.debugf("Adding target files back\n")
	targetFilesAdded := 0
	for _, targetFile := range e.targetFiles {
		e.debugf("Running 'git add %s'\n", targetFile)
		cmd = exec.Command("git", "add", targetFile)
		cmd.Dir = e.repoDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			// If normal add fails, try with --force to handle .gitignore'd files
			e.debugf("Add failed for %s: %v, output: %s\n", targetFile, err, string(output))
			e.debugf("Retrying with 'git add --force %s'\n", targetFile)
			cmd = exec.Command("git", "add", "--force", targetFile)
			cmd.Dir = e.repoDir
			output, err = cmd.CombinedOutput()
			if err != nil {
				e.debugf("Force add also failed for %s: %v, output: %s\n", targetFile, err, string(output))
				// Continue anyway - file might not exist in working dir
				continue
			}
			e.debugf("Force add successful for %s, output: %s\n", targetFile, string(output))
		} else {
			e.debugf("Add successful for %s, output: %s\n", targetFile, string(output))
		}
		targetFilesAdded++
	}

	e.debugf("Successfully added %d target files\n", targetFilesAdded)
	
	// Show what's staged before second commit
	e.debugGitStatus("Before second commit")

	// Check if we have anything to commit
	if targetFilesAdded == 0 {
		return fmt.Errorf("no target files were successfully staged for second commit")
	}

	// Create second commit (target files only)
	e.debugf("Creating second commit with message: %q\n", secondMsg)
	e.debugf("Preserving author: %s\n", commit.Author)
	cmd = exec.Command("git", "commit", "-m", secondMsg, "--author", commit.Author)
	cmd.Dir = e.repoDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		e.debugf("Second commit failed: %v, output: %s\n", err, string(output))
		return fmt.Errorf("failed to create second split commit: %w, output: %s", err, string(output))
	}
	e.debugf("Second commit successful, output: %s\n", string(output))

	e.debugf("Commit splitting completed successfully\n")
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

	firstMsg, secondMsg := GenerateSplitMessages(commit.Message, e.targetFiles)

	// Stage all files except the target file
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	// Unstage the target files
	for _, targetFile := range e.targetFiles {
		cmd = exec.Command("git", "reset", "HEAD", targetFile)
		cmd.Dir = e.repoDir
		if err := cmd.Run(); err != nil {
			// Ignore errors for files that don't exist in this commit
			continue
		}
	}

	// Create first commit (everything except target file)
	cmd = exec.Command("git", "commit", "-m", firstMsg, "--author", commit.Author)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create first split commit: %w", err)
	}

	// Add and commit the target files
	for _, targetFile := range e.targetFiles {
		cmd = exec.Command("git", "add", targetFile)
		cmd.Dir = e.repoDir
		if err := cmd.Run(); err != nil {
			// If normal add fails, try with --force to handle .gitignore'd files
			cmd = exec.Command("git", "add", "--force", targetFile)
			cmd.Dir = e.repoDir
			if err := cmd.Run(); err != nil {
				// Ignore errors for files that don't exist in working dir
				continue
			}
		}
	}

	cmd = exec.Command("git", "commit", "-m", secondMsg, "--author", commit.Author)
	cmd.Dir = e.repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create second split commit: %w", err)
	}

	return nil
}




// GenerateSplitMessages creates the two commit messages for a split
func GenerateSplitMessages(original string, targetFiles []string) (string, string) {
	// First commit: original + split notice
	var firstMsg string
	if len(targetFiles) == 1 {
		firstMsg = original + "\n\nChanges to " + targetFiles[0] + " split into a separate commit"
	} else {
		firstMsg = original + "\n\nChanges to target files split into a separate commit"
	}

	// Second commit: prefixed original
	var secondMsg string
	if len(targetFiles) == 1 {
		secondMsg = targetFiles[0] + ": " + original
	} else {
		secondMsg = "target files: " + original
	}

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

// debugGitStatus shows the current git status for debugging
func (e *Extractor) debugGitStatus(label string) {
	e.debugf("Git status %s:\n", label)
	
	// Get porcelain status
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = e.repoDir
	output, err := cmd.Output()
	if err != nil {
		e.debugf("Failed to get git status: %v\n", err)
		return
	}

	status := string(output)
	if status == "" {
		e.debugf("Working directory clean\n")
	} else {
		e.debugf("Status output:\n%s", status)
	}
	
	// Also show what's staged specifically
	cmd = exec.Command("git", "diff", "--cached", "--name-status")
	cmd.Dir = e.repoDir
	output, err = cmd.Output()
	if err != nil {
		e.debugf("Failed to get staged changes: %v\n", err)
		return
	}
	
	staged := string(output)
	if staged == "" {
		e.debugf("No staged changes\n")
	} else {
		e.debugf("Staged changes:\n%s", staged)
	}
	
	e.debugf("---\n")
}

