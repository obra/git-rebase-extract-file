// ABOUTME: Core rebase logic for extracting file changes into separate commits
// ABOUTME: Provides commit analysis and splitting functionality

package rebase

import (
	"fmt"
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

// GenerateSplitMessages creates the two commit messages for a split
func GenerateSplitMessages(original, targetFile string) (string, string) {
	// First commit: original + split notice
	firstMsg := original + "\n\nChanges to " + targetFile + " split into a separate commit"
	
	// Second commit: prefixed original
	secondMsg := targetFile + ": " + original
	
	return firstMsg, secondMsg
}