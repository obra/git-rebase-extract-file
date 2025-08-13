# Git Rebase Extract File

## Overview

A git command that performs an interactive rebase on a commit range, automatically splitting any commits that contain changes to a specified file. The changes to the target file are extracted into separate commits while preserving all original metadata.

## Command Interface

```bash
git rebase-extract-file [--dry-run] <previous-rev> <file-path>
```

### Arguments
- `<previous-rev>`: The revision to rebase from (exclusive). Command processes commits in range `<previous-rev>..HEAD`
- `<file-path>`: Path to the file to extract, specified from repository root (e.g., `src/components/Button.tsx`)

### Options
- `--dry-run`: Preview mode that shows what commits would be affected and how they would be split, without making any changes

## Behavior

### Commit Processing
1. Examines all commits in range `<previous-rev>..HEAD`
2. For each commit that contains changes to `<file-path>` AND other files:
   - Splits into two commits preserving original order in history
   - First commit: all changes EXCEPT the target file
   - Second commit: ONLY changes to the target file
3. Preserves commits that only touch the target file unchanged
4. Flattens merge commits and applies splitting logic to the resulting changes

### Commit Message Format
When splitting a commit with original message "Fix user authentication bug":

**First commit (non-target changes):**
```
Fix user authentication bug

Changes to src/components/Button.tsx split into a separate commit
```

**Second commit (target file changes):**
```
src/components/Button.tsx: Fix user authentication bug
```

### Metadata Preservation
- Original author information preserved on both commits
- Original commit timestamps preserved as much as possible
- All other git metadata (committer, etc.) preserved

## Dry Run Output

Shows list of affected commits with before/after commit messages:

```
Would split 3 out of 7 commits:

Commit abc1234: "Fix user authentication bug"
├─ Split into: "Fix user authentication bug\n\nChanges to src/components/Button.tsx split into a separate commit"
└─ Split into: "src/components/Button.tsx: Fix user authentication bug"

Commit def5678: "Add new feature X"  
├─ Split into: "Add new feature X\n\nChanges to src/components/Button.tsx split into a separate commit"
└─ Split into: "src/components/Button.tsx: Add new feature X"

Commit ghi9012: "Update styling and fix typo"
├─ Split into: "Update styling and fix typo\n\nChanges to src/components/Button.tsx split into a separate commit"  
└─ Split into: "src/components/Button.tsx: Update styling and fix typo"
```

## Edge Cases

1. **Commits with only target file changes**: Left unchanged, no splitting occurs
2. **Merge commits**: Flattened and changes split according to normal rules
3. **Empty commits after splitting**: Should not occur, but if somehow created, should be dropped
4. **Conflicts during rebase**: Standard git conflict resolution applies

## Prerequisites

- Must be run from within a git repository
- Working directory should be clean (no uncommitted changes)
- Target file path must exist in at least one commit in the range