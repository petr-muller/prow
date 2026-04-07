---
name: maintainer-review
description: Reviews a PR from multiple maintainer perspectives by spawning parallel sub-agent reviewers, then walks the maintainer through each perspective interactively. Use when the user asks to review a PR as a maintainer or mentions /maintainer-review. Supports a "gate" subcommand for LGTM'd PRs.
---

# Maintainer Review

Reviews a pull request from multiple maintainer perspectives in parallel, then presents findings as an interactive walkthrough — one perspective at a time — with opportunity for questions after each.

## Subcommands

This skill supports two modes of operation:

- **(default)**: Full maintainer review. A senior maintainer advisor synthesizes all perspectives into an actionable recommendation.
- **gate**: Merge gate check for PRs that have already been LGTM'd by a trusted reviewer. Same three reviewers run, but the advisor focuses strictly on finding reasons to **block** the merge. The bar for blocking is high: critical bugs, serious regressions, or deployment-breaking incompatibilities. Everything else is noted but does not block.

When the user invokes `/maintainer-review gate <number>`, use the gate subcommand. Otherwise, use the default.

## Parameters

- `pr_number` (required): The PR number to review (e.g., 123)
- `subcommand` (optional): `gate` — if specified, use the gate advisor instead of the default advisor

## Instructions

You are orchestrating a multi-perspective maintainer review of PR #{pr_number}.

### Step 1: Gather PR Context

Fetch the PR details so you can pass context to the sub-agents:

```bash
gh pr view {pr_number} --json title,body,state,labels,files,additions,deletions,baseRefName,headRefName,author,url
```

Also fetch the diff:

```bash
gh pr diff {pr_number}
```

If the PR is very large (more than ~2000 lines of diff), note this and still proceed — each reviewer will fetch the diff independently.

### Step 2: Launch Parallel Reviewers in Background

Read the reviewer instruction files:
- `.claude/skills/maintainer-review/reviewers/code-quality.md`
- `.claude/skills/maintainer-review/reviewers/maintainability.md`
- `.claude/skills/maintainer-review/reviewers/deployment-risk.md`

Spawn **three** sub-agents **in parallel in the background** using the Agent tool with `run_in_background: true`. Each agent should receive:
- The PR number
- The PR title and description
- A summary of changed files (from the files list above)
- The full contents of their respective reviewer instruction file, with `{pr_number}`, `{title}`, `{description}`, and `{files_summary}` substituted

Each agent must be instructed to:
1. Fetch the full PR diff using `gh pr diff {pr_number}`
2. Read any files they need for full context using the Read tool
3. Perform their specific review analysis
4. Return structured findings in the format specified in their instruction file

**IMPORTANT**: Launch all three agents in a **single message** with three parallel Agent tool calls. Each agent should use `subagent_type: "general-purpose"`.

While reviewers are working, tell the user:

> Reviewers are analyzing PR #{pr_number}. I've dispatched three independent reviewers:
> 1. **Code Quality** — reviewing as a senior Go engineer
> 2. **Maintainability** — reviewing as a long-term project maintainer
> 3. **Deployment Risk** — reviewing as a Prow platform operator
>
> I'll walk you through each perspective as they complete.

### Step 3: Interactive Walkthrough

Present each reviewer's findings **one at a time**, in the following order. Wait for each reviewer's background agent to complete before presenting. Between each presentation, **pause and explicitly invite questions** before moving on.

#### Presentation 1: Code Quality

When the Code Quality agent completes, present its findings as a brief:

> ---
> ### 1/3 — Code Quality Review
>
> {Condensed summary: 3-5 key bullet points from the agent's findings}
>
> **Verdict**: {APPROVE / REQUEST_CHANGES / COMMENT}
>
> {If there are critical issues, list them clearly}
>
> ---
> *Any questions about the code quality assessment before we move on?*

Wait for the user to respond. If they ask questions, answer them using the full detail from the agent's findings. If the user needs you to look at specific code, do so. Once the user indicates they're ready (e.g., "next", "continue", "no questions", or similar), proceed.

#### Presentation 2: Maintainability

When the Maintainability agent completes, present its findings:

> ---
> ### 2/3 — Maintainability Review
>
> {Condensed summary: 3-5 key bullet points from the agent's findings}
>
> **Maintenance Burden**: {LOW / MEDIUM / HIGH}
> **Verdict**: {APPROVE / REQUEST_CHANGES / COMMENT}
>
> {If there are concerns, list them clearly}
>
> ---
> *Any questions about the maintainability assessment before we move on?*

Wait for the user to respond. Same as above — answer questions, then proceed when ready.

#### Presentation 3: Deployment Risk

When the Deployment Risk agent completes, present its findings:

> ---
> ### 3/3 — Deployment Risk Review
>
> {Condensed summary: 3-5 key bullet points from the agent's findings}
>
> **Risk Level**: {LOW / MEDIUM / HIGH / CRITICAL}
>
> {If there are breaking changes or upgrade considerations, list them clearly}
>
> ---
> *Any questions about the deployment risk assessment before I bring in the maintainer advisor?*

Wait for the user to respond. Same as above.

### Step 4: Advisor Synthesis

After the user has been walked through all three perspectives and has no more questions, spawn a **final agent** (foreground, not background) using the Agent tool with `subagent_type: "general-purpose"`.

**Choose the advisor based on the subcommand**:

- **Default (no subcommand)**: Read the advisor instructions from `.claude/skills/maintainer-review/reviewers/advisor.md`
- **gate subcommand**: Read the advisor instructions from `.claude/skills/maintainer-review/reviewers/gate-advisor.md`

Pass the advisor instructions as the agent's prompt, along with the **full findings from all three reviewers**.

**For the default subcommand**, present the advisor's recommendation:

> ---
> ### Maintainer Advisor — Final Recommendation
>
> {The advisor's structured output}
>
> ---

**For the gate subcommand**, present the gate decision:

> ---
> ### Merge Gate Decision
>
> {The gate advisor's structured output}
>
> ---

### Step 5: Offer Next Steps

After presenting the advisor's recommendation, offer the user:
- Post the review as a PR comment (using `gh pr review` or `gh pr comment`)
- Post only specific parts (e.g., just the recommendation, or just the concerns)
- Refine or revisit any specific perspective
- Dive deeper into any particular concern

## Important Notes

- Each reviewer operates independently — they should not duplicate each other's concerns
- Keep each walkthrough presentation **brief and scannable** — the user can ask for details
- The walkthrough is conversational: adapt to the user's pace and questions
- If a reviewer's agent hasn't completed when it's time to present, tell the user you're waiting and present it as soon as it arrives
- Be constructive: frame concerns as improvement suggestions, not just criticism
- If the PR is trivial (e.g., typo fix, comment update), say so briefly rather than forcing deep analysis across all perspectives
- Flag any finding where two or more reviewers independently identified the same concern — mention this convergence to the user during the walkthrough
