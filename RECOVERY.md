# Recovery Guide

If `git-rebase-extract-file` fails and leaves you in a broken state, here's how to recover:

## Current State: Merge Conflicts with No Active Rebase

If you see:
```
Unmerged paths:
  (use "git restore --staged <file>..." to unstage)
  (use "git add <file>..." to mark resolution)
        both modified:   some/file.txt

❯ git rebase --abort
fatal: No rebase in progress?
```

**Recovery Steps:**

1. **Reset to clean state:**
   ```bash
   git reset --hard HEAD
   ```

2. **If that doesn't work, reset to your original branch:**
   ```bash
   # Find your backup branch (look for pattern: branchname-backup-<pid>)
   git branch | grep backup
   
   # Reset to the backup
   git reset --hard <backup-branch-name>
   ```

3. **If no backup branch exists, use reflog:**
   ```bash
   # Show recent commits
   git reflog
   
   # Reset to the state before running the tool
   git reset --hard HEAD@{N}  # Replace N with appropriate number
   ```

## Prevention

Always use `--dry-run` first:
```bash
git-rebase-extract-file --dry-run <rev> <file>
```

## Common Issues

- **Cherry-pick conflicts**: The tool can't handle complex merge conflicts automatically
- **Diverged branches**: Make sure your branch is up to date before running
- **File dependencies**: If the target file changes depend on other changes in the same commit, conflicts are likely

## Better Alternative

For complex histories with conflicts, consider manual approach:
1. `git rebase -i <rev>`
2. Mark commits for editing (`e`)
3. Manually split using `git reset HEAD~1` and selective `git add`