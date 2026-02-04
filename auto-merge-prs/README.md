# Chore: Auto-Merge PRs

This [Tedium](https://github.com/markormesher/tedium) chore auto-merges PRs that meet the following criteria:

- An `automerge` label is applied to the PR.
- No `do not merge` label is applied to the PR.
- The PR has no conflicts.
- The target branch is protected and has at least one required status check.
- The latest commit in the PR satsfies every required status check.
