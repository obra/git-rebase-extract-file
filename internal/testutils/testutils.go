// ABOUTME: Test utilities for git operations and repository setup
// ABOUTME: Provides helper functions to create test repos with various commit scenarios

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepo represents a test git repository
type TestRepo struct {
	Dir string
	t   *testing.T
}

// NewTestRepo creates a new temporary git repository for testing
func NewTestRepo(t *testing.T) *TestRepo {
	t.Helper()
	
	dir, err := os.MkdirTemp("", "git-rebase-extract-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	repo := &TestRepo{Dir: dir, t: t}
	repo.runGit("init")
	repo.runGit("config", "user.name", "Test User")
	repo.runGit("config", "user.email", "test@example.com")
	
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	
	return repo
}

// WriteFile writes content to a file in the test repo
func (r *TestRepo) WriteFile(path, content string) {
	r.t.Helper()
	
	fullPath := filepath.Join(r.Dir, path)
	dir := filepath.Dir(fullPath)
	
	if err := os.MkdirAll(dir, 0755); err != nil {
		r.t.Fatalf("Failed to create directory %s: %v", dir, err)
	}
	
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		r.t.Fatalf("Failed to write file %s: %v", fullPath, err)
	}
}

// Commit adds all files and creates a commit with the given message
func (r *TestRepo) Commit(message string) string {
	r.t.Helper()
	
	r.runGit("add", ".")
	r.runGit("commit", "-m", message)
	
	output, err := r.gitOutput("rev-parse", "HEAD")
	if err != nil {
		r.t.Fatalf("Failed to get HEAD commit: %v", err)
	}
	
	return strings.TrimSpace(output)
}

// CommitFile adds a specific file and commits it
func (r *TestRepo) CommitFile(file, message string) string {
	r.t.Helper()
	
	r.runGit("add", file)
	r.runGit("commit", "-m", message)
	
	output, err := r.gitOutput("rev-parse", "HEAD")
	if err != nil {
		r.t.Fatalf("Failed to get HEAD commit: %v", err)
	}
	
	return strings.TrimSpace(output)
}

// GetCommitMessage returns the commit message for a given commit
func (r *TestRepo) GetCommitMessage(commit string) string {
	r.t.Helper()
	
	output, err := r.gitOutput("log", "--format=%B", "-n", "1", commit)
	if err != nil {
		r.t.Fatalf("Failed to get commit message: %v", err)
	}
	
	return output
}

// GetCommitFiles returns the list of files changed in a commit
func (r *TestRepo) GetCommitFiles(commit string) []string {
	r.t.Helper()
	
	output, err := r.gitOutput("show", "--name-only", "--format=", commit)
	if err != nil {
		r.t.Fatalf("Failed to get commit files: %v", err)
	}
	
	if output == "" {
		return []string{}
	}
	
	return []string{output} // Simplified for now
}

// runGit executes a git command in the test repo
func (r *TestRepo) runGit(args ...string) {
	r.t.Helper()
	
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	
	if err := cmd.Run(); err != nil {
		r.t.Fatalf("Git command failed: git %v, error: %v", args, err)
	}
}

// gitOutput executes a git command and returns its output
func (r *TestRepo) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(output)), nil
}