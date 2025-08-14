# git-rebase-extract-file

> ⚠️ **This tool was vibe-coded and you should trust it as far as you can throw it.** While it has comprehensive tests and follows best practices, it manipulates your git history. Always use `--dry-run` first and ensure you have backups.

A git command that performs an interactive rebase to automatically split commits by extracting changes to specified files or directories into separate commits, while preserving all original metadata.

[![CI](https://github.com/obra/git-rebase-extract-file/actions/workflows/ci.yml/badge.svg)](https://github.com/obra/git-rebase-extract-file/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/obra/git-rebase-extract-file)](https://goreportcard.com/report/github.com/obra/git-rebase-extract-file)

## Overview

Have you ever made commits that mixed changes to specific files with other changes, and later wished you could cleanly separate them? This tool automatically identifies such commits in a range and splits them into two commits:

1. **First commit**: All changes except the target files + notice about the split
2. **Second commit**: Only changes to the target files with prefixed message

## Installation

### From Source

```bash
go install github.com/obra/git-rebase-extract-file@latest
```

### Manual Build

```bash
git clone https://github.com/obra/git-rebase-extract-file.git
cd git-rebase-extract-file
make build
sudo cp bin/git-rebase-extract-file /usr/local/bin/
```

## Usage

### Basic Syntax

```bash
git-rebase-extract-file [--dry-run] <previous-rev> <file-path> [file-path...]
```

### Arguments

- `<previous-rev>`: The commit to rebase from (exclusive). Tool processes commits in range `<previous-rev>..HEAD`
- `<file-path>`: Path to files or directories to extract, specified from repository root
  - Files: `src/components/Button.tsx` 
  - Directories: `src/components/` (extracts all files in directory)
  - Multiple: `src/component1.tsx src/component2.tsx lib/utils.ts`

### Options

- `--dry-run`: Preview what would be done without making any changes

## Examples

### Preview Changes (Recommended First Step)

```bash
# See what commits would be split
git-rebase-extract-file --dry-run main~5 src/auth.go
```

Output:
```
Would split 2 out of 4 commits:

Commit abc1234: "Fix user authentication bug"
├─ Split into: "Fix user authentication bug

Changes to src/auth.go split into a separate commit"
└─ Split into: "src/auth.go: Fix user authentication bug"

Commit def5678: "Add new feature and update auth"  
├─ Split into: "Add new feature and update auth

Changes to src/auth.go split into a separate commit"
└─ Split into: "src/auth.go: Add new feature and update auth"
```

### Perform the Extraction

```bash
# Actually split the commits
git-rebase-extract-file main~5 src/auth.go
```

### Real-World Scenarios

```bash
# Split React component changes from other files
git-rebase-extract-file feature-start src/components/Button.tsx

# Extract multiple related components
git-rebase-extract-file feature-start src/components/Button.tsx src/components/Modal.tsx

# Extract entire directory of components
git-rebase-extract-file feature-start src/components/

# Mix files and directories  
git-rebase-extract-file feature-start src/components/ lib/utils.ts

# Extract configuration changes from a feature branch
git-rebase-extract-file main config/database.yml

# Separate documentation updates
git-rebase-extract-file v1.0.0 README.md
```

## How It Works

1. **Analysis**: Examines commits in the specified range to identify which ones modify both the target files and other files
2. **Backup**: Creates a backup branch before making any changes
3. **Interactive Rebase**: Uses automated interactive rebase to rebuild the history:
   - For mixed commits: Splits into two separate commits
   - For single-purpose commits: Leaves unchanged
4. **Preservation**: Maintains original commit metadata (author, timestamp, etc.)

## Commit Message Format

### Original Commit
```
Fix user authentication bug

This resolves the login timeout issue reported in #123.
```

### After Splitting

**First Commit (non-target changes)**:
```
Fix user authentication bug

This resolves the login timeout issue reported in #123.

Changes to src/auth.go split into a separate commit
```

**Second Commit (target file changes)**:
```
src/auth.go: Fix user authentication bug

This resolves the login timeout issue reported in #123.
```

### Multi-File Example

For multiple files, the messages adapt:

**First Commit**:
```
Add new feature

Changes to target files split into a separate commit  
```

**Second Commit**:
```
target files: Add new feature
```

## Safety Features

- **Backup Branch**: Automatically creates `<current-branch>-backup-<pid>` before making changes
- **Recovery Instructions**: Prints recovery commands upfront so you know how to get back
- **Conflict Detection**: Warns about potential conflicts before starting and provides guidance during rebase
- **Dry Run**: Always preview changes first with `--dry-run`
- **No Action**: If no commits need splitting, tool exits cleanly without changes
- **Git Integration**: Uses standard git interactive rebase for reliability

## Edge Cases Handled

- **Target-only commits**: Left unchanged (no splitting needed)
- **No target changes**: Commits are left as-is
- **Merge commits**: Flattened and changes split according to normal rules
- **Empty results**: If target file not found in range, no changes made

## Development

### Prerequisites

- Go 1.21+
- Git
- golangci-lint (for linting)

### Building

```bash
make build          # Build binary
make test           # Run tests
make lint           # Run linter
make check          # Run all quality checks
make coverage       # View test coverage
```

### Testing

The tool includes comprehensive tests with 84.8% coverage:

```bash
make test
```

Tests include:
- Commit analysis scenarios
- Dry-run output validation
- Actual rebase operations
- Message generation
- Edge cases

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Run `make check` to ensure quality
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Troubleshooting

### Command Not Found
If you get "command not found", ensure the binary is in your PATH:
```bash
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc
```

### Git Errors
- Ensure you're in a git repository
- Check that the revision and file path are valid
- Make sure working directory is clean before running

### Recovery
If something goes wrong, you can always return to your original state:
```bash
git checkout <current-branch>-backup-<pid>
```

## Limitations

- Security warnings in linter due to dynamic git commands (by design)
- File permissions in tests may trigger security warnings (test-only)
- Directory extraction uses prefix matching (files must be under the specified directory)

## Roadmap

- [x] Support for multiple target files
- [x] Integration with `git rebase -i` for better conflict handling  
- [ ] Preservation of commit signatures
- [ ] Support for binary files
- [ ] Interactive mode for commit selection
- [ ] Glob pattern support (e.g., `*.tsx`, `**/*.test.js`)