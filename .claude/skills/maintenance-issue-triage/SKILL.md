---
name: maintenance-issue-triage
description: Helps maintainers triage GitHub issues by setting up a dedicated triage branch and managing the triage workflow. Use when the user asks to triage a GitHub issue or mentions triaging issue numbers.
---

# Issue Triage

Helps maintainers triage GitHub issues by orchestrating focused triage subcommands.

## Parameters

- `issue_number` (required): The GitHub issue number to triage

## Instructions

You are helping a project maintainer triage GitHub issue #{issue_number}.

Follow these steps to set up the triage environment:

### 1. Update claude-maintenance-helpers Branch

The `.claude` directory with triage skills exists only in the `claude-maintenance-helpers` branch (not in upstream). First, ensure this branch is up to date from the `origin` remote:

```bash
# Fetch all remotes
git fetch --all

# Switch to claude-maintenance-helpers and update it from origin
git checkout claude-maintenance-helpers
git pull --rebase origin claude-maintenance-helpers
```

### 2. Create or Update Triage Branch

Create or update a branch called `issue-triage-{issue_number}` for this triage session. This branch must be based on `claude-maintenance-helpers` to have access to the triage skills.

**If the branch doesn't exist (new triage):**
```bash
# Check if branch exists
git rev-parse --verify issue-triage-{issue_number} 2>/dev/null

# If it doesn't exist, create it from claude-maintenance-helpers
git checkout -b issue-triage-{issue_number} claude-maintenance-helpers
```

**If the branch already exists (triage in progress):**
```bash
# Switch to the branch
git checkout issue-triage-{issue_number}

# Rebase it on top of the updated claude-maintenance-helpers
git rebase claude-maintenance-helpers
```

Note: If the rebase encounters conflicts, help the user resolve them before continuing.

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
- Explain that triage subcommands are available to help analyze different aspects of the issue
- Mention that all findings will be accumulated in ISSUE-TRIAGE.md
- Suggest running the `initial` subcommand first to validate the issue

## Available Subcommands

After the triage environment is set up, you can use focused subcommands to analyze specific aspects of the issue. Each subcommand updates the ISSUE-TRIAGE.md file with its findings.

### Subcommand: initial

**Purpose**: Validate whether the issue constitutes a legitimate record that should be kept.

**When to use**: First step after setting up the triage environment.

**What it does**:
- Analyzes issue title, description, and context
- Determines if it's a bug/feature for this repository vs misconfiguration/wrong repo
- Checks if sufficient information is provided
- Provides recommendation: LEGITIMATE, NEEDS_INFO, CLOSE, or REDIRECT

**How to invoke**: After the user mentions wanting to run initial validation, read the instructions from `.claude/skills/maintenance-issue-triage/subcommands/initial.md` and follow them.

### Subcommand: research

**Purpose**: Conduct in-depth code research to understand the issue and propose high-level architectural solutions.

**When to use**: After initial validation confirms the issue is legitimate.

**What it does**:
- Explores relevant code paths and components
- Analyzes architecture and design patterns
- Examines test coverage and documentation
- Identifies root causes of the issue
- Proposes high-level (architectural) approaches to fixing the issue
- Does NOT write code - focuses on understanding and analysis

**How to invoke**: After the user mentions wanting to research the code or understand the implementation, read the instructions from `.claude/skills/maintenance-issue-triage/subcommands/research.md` and follow them.

### Subcommand: assess-effort

**Purpose**: Assess the effort required to address an issue and categorize it into effort levels (1-4).

**When to use**: After research has been completed and the solution approach is understood.

**What it does**:
- Evaluates scope of changes (files, lines of code, components)
- Assesses complexity and required expertise
- Considers backwards compatibility impact
- Evaluates architectural alignment with Prow
- Analyzes testing requirements
- Categorizes effort as Level 1 (easy/good-first-issue), Level 2 (moderate/help-needed), Level 3 (large/expert), or Level 4 (very large/impossible)
- Recommends appropriate labels
- Provides guidance for potential contributors

**Effort Levels**:
- **Level 1**: Easy, well-defined, good for new contributors (good-first-issue)
- **Level 2**: Moderate, well-defined but involved (help-needed)
- **Level 3**: Large change or significant uncertainty, requires expertise
- **Level 4**: Very large or contradicts architecture, near impossible

**How to invoke**: After the user mentions wanting to assess effort or determine appropriate labels, read the instructions from `.claude/skills/maintenance-issue-triage/subcommands/assess-effort.md` and follow them.

### Future Subcommands

Additional focused subcommands may be added to assist with:
- Reproduction attempt
- Related issue identification
- Priority and severity assignment

## Important Notes

- Always work within the `issue-triage-{issue_number}` branch
- All triage findings should be documented in ISSUE-TRIAGE.md
- The ISSUE-TRIAGE.md file serves as the shared state between different triage subcommands
- Commits should be made regularly to preserve triage progress
- Run subcommands in sequence, starting with `initial` validation
