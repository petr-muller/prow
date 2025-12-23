# Issue Triage

Helps maintainers triage GitHub issues by orchestrating focused triage subcommands.

## Parameters

- `issue_number` (required): The GitHub issue number to triage

## Instructions

You are helping a project maintainer triage GitHub issue #{issue_number}.

Follow these steps to set up the triage environment:

### 1. Fetch Latest Upstream Development

First, fetch all upstream changes to ensure you're working with the latest code:

```bash
git fetch --all
```

### 2. Create or Switch to Triage Branch

Create a new local branch called `issue-triage-{issue_number}` for this triage session.

- If the branch already exists, switch to it (this means triage for this issue is already in progress)
- If the branch doesn't exist, create it from the current HEAD

Use these git commands:
- Check if branch exists: `git rev-parse --verify issue-triage-{issue_number}`
- If it exists: `git checkout issue-triage-{issue_number}`
- If it doesn't exist: `git checkout -b issue-triage-{issue_number}`

### 3. Initialize or Load Triage Document

The triage content is stored in `ISSUE-TRIAGE.md` in the root of the repository. This file will be committed to the `issue-triage-{issue_number}` branch.

- If `ISSUE-TRIAGE.md` already exists (triage in progress), read it and display its current contents to the user
- If it doesn't exist, create it with the following initial structure:

```markdown
# Triage for Issue #{issue_number}

**Status**: In Progress
**Created**: [current date]

## Issue Information

- **Issue Number**: #{issue_number}
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/{issue_number}

## Findings

(Findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)
```

After creating the file, commit it to the triage branch:

```bash
git add ISSUE-TRIAGE.md
git commit -m "Initialize triage for issue #{issue_number}"
```

### 4. Summary

After completing the setup, provide a summary to the user:

- Confirm the triage branch is active
- Show the current state of the ISSUE-TRIAGE.md file
- Explain that triage subcommands will be available to help analyze different aspects of the issue
- Mention that all findings will be accumulated in ISSUE-TRIAGE.md

## Important Notes

- Always work within the `issue-triage-{issue_number}` branch
- All triage findings should be documented in ISSUE-TRIAGE.md
- The ISSUE-TRIAGE.md file serves as the shared state between different triage subcommands
- Commits should be made regularly to preserve triage progress
