// ABOUTME: Git operations and utilities for repository manipulation
// ABOUTME: Provides safe wrappers around git commands with proper error handling

// Package git provides git repository operations and utilities.
package git

import (
	"os/exec"
)

// Repository represents a git repository
type Repository struct {
	Dir string
}

// NewRepository creates a new repository instance
func NewRepository(dir string) *Repository {
	return &Repository{Dir: dir}
}

// RunGit executes a git command in the repository
func (r *Repository) RunGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	return cmd.Run()
}

// GitOutput executes a git command and returns its output
func (r *Repository) GitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

