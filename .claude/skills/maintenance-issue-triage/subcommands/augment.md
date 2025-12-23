# Subcommand: maintenance::issues::triage::augment

## Purpose

Propose improvements to the issue based on triage findings. This subcommand synthesizes all triage research into a GitHub comment that:
- Adds missing context and information discovered during triage
- Improves the issue title if needed (more clear, precise, succinct)
- Applies appropriate area and kind labels
- Applies difficulty labels (good-first-issue, help-needed) based on effort assessment
- Transforms the issue into a high-quality bug report/feature request

The goal is to make the issue as informative as if it were reported by an omniscient expert who knows everything about the codebase and never forgets important details.

## Parameters

- `issue_number` (required): The GitHub issue number to augment

## Instructions

You are proposing improvements for GitHub issue #{issue_number}.

### Step 1: Review All Triage Findings

Read `ISSUE-TRIAGE.md` completely to understand:
- Initial validation results
- Code research findings (root cause, architecture, implementation details)
- Effort assessment
- What information is already in the original issue
- What critical information is missing

### Step 2: Evaluate Issue Title

**Review the current title** and determine if it needs improvement:

**Good titles are**:
- Specific and descriptive (not vague)
- Mention the affected component
- Concise (typically 5-12 words)
- Indicate the type of issue (bug behavior, feature request)
- Use present tense for bugs ("merges" not "merged")

**Examples of improvements**:
- ❌ "Problem with Tide" → ✅ "Tide merges PR when GitHub Actions are re-triggered"
- ❌ "Add feature" → ✅ "Add support for custom merge commit messages in Tide"
- ❌ "Error in deck" → ✅ "Deck fails to render plugin help when plugin name contains slash"

**When to retitle**:
- Current title is vague or unclear
- Component name is missing but should be included
- Title doesn't reflect the actual issue (refined during triage)
- Title is too long or verbose
- Title uses past tense for ongoing bug

**When NOT to retitle**:
- Current title is already clear and specific
- Only minor wording improvements (not worth the noise)
- Title accurately reflects the issue

### Step 3: Identify Missing Context

Determine what critical information from triage is **not** in the original issue:

**For bugs, consider adding**:
- Root cause explanation (if identified during research)
- Affected components and code paths
- Why the bug occurs (architectural/implementation reason)
- Reproduction conditions (if clearer than original)
- Related issues or PRs (if discovered)
- Workarounds (if any exist)

**For features, consider adding**:
- How this fits with existing architecture
- Similar functionality that exists elsewhere
- Implementation complexity notes
- Potential approaches or patterns to follow

**Important guidelines**:
- Only add information that's **missing** from the original issue
- Don't repeat what the reporter already said
- Focus on insights from code research
- Keep it concise (2-3 paragraphs maximum)
- Use technical precision but remain accessible

### Step 4: Determine Appropriate Labels

Based on triage findings, determine which labels to apply:

#### Area Labels (/area command)

Identify which Prow component(s) this affects:
- `tide` - Tide (merge automation)
- `deck` - Deck (UI)
- `plank` - Plank (job execution)
- `hook` - Hook (webhook handling)
- `crier` - Crier (reporting)
- `gerrit` - Gerrit support
- `github-provider` - GitHub provider
- `config` - Configuration
- `plugins` - Prow plugins
- etc.

**Apply the most specific area label(s)** - typically 1-2 labels.

#### Kind Labels (/kind command)

Determine the issue type:
- `bug` - Something is broken
- `feature` - New functionality request
- `cleanup` - Code cleanup, refactoring, tech debt
- `documentation` - Documentation improvements
- `flake` - Flaky test
- `failing-test` - Consistently failing test

**Apply exactly one kind label.**

#### Difficulty Labels

Based on effort assessment:
- **Level 1**: `/good-first-issue` - Easy for new contributors
- **Level 2**: `/help-wanted` - Moderate, seeking contributors
- **Level 3**: No label - Requires expertise, experienced contributors will self-select
- **Level 4**: No label - May want to discuss or close instead

**Note**: For Level 3-4 issues, do NOT add good-first-issue or help-wanted labels. These are complex issues that require expertise.

#### Priority Labels (optional, use sparingly)

Only suggest priority labels if clearly warranted:
- `/priority critical-urgent` - System is broken, blocking many users
- `/priority important-soon` - Important bug affecting functionality
- `/priority important-longterm` - Important but not urgent

**Don't suggest priority labels** for:
- Issues already being worked on (PR exists)
- Minor bugs or enhancements
- When unsure - let maintainers decide

### Step 5: Draft the GitHub Comment

Compose a comment with the following structure:

```markdown
[Optional: /retitle New Title Here - only if title needs improvement]

[2-3 paragraphs of missing context - only information NOT in the original issue]

[Paragraph 1: Root cause or key insight from research]
[Paragraph 2: Additional technical context, affected components, or implementation notes]
[Paragraph 3: Related work, workarounds, or next steps - if applicable]

/area [component-name]
/kind [issue-type]
[/good-first-issue OR /help-wanted - only for Level 1-2]
[/priority [level] - only if clearly warranted]
```

### Step 6: Update Triage Document

Add your proposed comment to `ISSUE-TRIAGE.md`:

```markdown
## Proposed Issue Augmentation

### Title Change
- **Current**: [current title]
- **Proposed**: [new title]
- **Rationale**: [why the change improves clarity]

OR

- **No change needed**: Current title is clear and specific

### Proposed GitHub Comment

```
[The full comment as drafted in Step 5]
```

### Rationale

**What's being added**:
- [Explanation of what context is being added and why]

**Why these labels**:
- `/area [label]`: [Justification]
- `/kind [label]`: [Justification]
- `/[difficulty-label]`: [Justification based on effort assessment]

**What's NOT included**:
- [Explain what was considered but not included, and why]
```

### Step 7: Commit Augmentation Proposal

Save your proposal to the triage document:

```bash
git add ISSUE-TRIAGE.md
git commit -m "Proposed augmentation for issue #{issue_number}

- [Retitle: new title] OR [No title change needed]
- Added missing context: [brief summary]
- Labels: area/[x], kind/[x], [difficulty if applicable]"
```

## Augmentation Examples

### Example 1: Bug with Clear Title, Adding Context

**Original Issue**:
```
Title: Tide merges PR when retesting GitHub action
Body: It happens from time to time that tide merges a PR when re-triggering a GitHub action...
[screenshot, example PR, code reference]
```

**Proposed Augmentation**:
```markdown
## Root Cause

This is a race condition in Tide's context checking logic. When a GitHub Action is re-triggered, GitHub temporarily **removes** the old CheckRun from its API before creating the new one. During this brief window (typically a few seconds), the required check is completely missing from GitHub's status API. If Tide's sync loop runs during this window, it sees "no unsuccessful contexts" and incorrectly proceeds with the merge.

## Technical Details

The issue occurs in `pkg/tide/tide.go:865-889` (`unsuccessfulContexts` function). Tide doesn't currently track which contexts were previously seen for a commit, so when a required context disappears, there's no way to distinguish between "context never existed" (might be OK) and "context disappeared" (suspicious, likely re-trigger). The fix in PR #563 addresses this by maintaining state of previously-seen contexts per PR/commit.

/area tide
/kind bug
/priority important-soon
```

**Rationale**:
- No /retitle needed - title is already clear
- Added: Root cause explanation (not in original issue)
- Added: Technical implementation details from code research
- Added: Reference to the fix PR
- Labels: area/tide (component), kind/bug (type), priority/important-soon (can cause incorrect merges)
- No difficulty label: Level 3 issue, requires expertise

### Example 2: Vague Title and Missing Context

**Original Issue**:
```
Title: Problem with deck
Body: Deck crashes when I try to view some plugins. Error message: "invalid plugin name"
```

**Proposed Augmentation**:
```markdown
/retitle Deck fails to render plugin help when plugin name contains slash

## Root Cause

This occurs because Deck's plugin help rendering code in `pkg/deck/plugin_help.go` uses the plugin name directly in a URL path without proper escaping. When a plugin name contains a slash (e.g., `my/plugin`), it's interpreted as a path separator, causing the router to fail with "invalid plugin name".

## Fix Approach

The fix should URL-encode plugin names when constructing help URLs. There's existing URL encoding in `pkg/deck/jobs.go:156` that handles similar cases and can serve as a pattern to follow. This is a localized fix affecting only the plugin help rendering code path.

/area deck
/kind bug
/good-first-issue
```

**Rationale**:
- /retitle: Original title "Problem with deck" is too vague; new title is specific and descriptive
- Added: Root cause (URL encoding issue)
- Added: Where the bug is (file reference)
- Added: How to fix it (follow existing pattern)
- Labels: area/deck, kind/bug, good-first-issue (Level 1 - simple, well-defined fix)

### Example 3: Feature Request Needing Clarity

**Original Issue**:
```
Title: Support for custom merge messages
Body: It would be nice if we could customize the commit message when Tide merges PRs.
```

**Proposed Augmentation**:
```markdown
/retitle Add support for custom merge commit messages in Tide

## Architecture Context

Tide currently generates merge commit messages in `pkg/tide/github.go:mergePR()` using a fixed template: `"Merge pull request #%d from %s"`. To support customization, this would need:
1. New configuration field in `config.Tide` (following the pattern of existing config like `MergeMethod`)
2. Template parsing logic (Go's `text/template` is already used elsewhere in Prow)
3. Access to PR metadata for template variables (title, author, labels, etc.)

## Similar Functionality

The `mergeCommitTemplate` pattern is similar to how Gerrit provider handles commit messages in `pkg/tide/gerrit.go:255-270`. That implementation could serve as a reference for the template structure and variable substitution approach.

/area tide
/kind feature
/help-wanted
```

**Rationale**:
- /retitle: Added "in Tide" for specificity
- Added: Architecture context (where code lives, what needs changing)
- Added: Reference to similar functionality
- Labels: area/tide, kind/feature, help-wanted (Level 2 - moderate complexity, well-scoped)

### Example 4: Issue That Should Be Closed

**Original Issue**:
```
Title: Prow doesn't work with BitBucket
Body: We use BitBucket for our repos. Can Prow support BitBucket?
```

**Proposed Augmentation**:
```markdown
## Architectural Limitations

Prow is fundamentally designed around GitHub's webhook events, API structure, and authentication model. Supporting BitBucket would require:
- Abstraction layer across 50+ files currently using GitHub APIs directly
- Different webhook event models (BitBucket events don't map 1:1 to GitHub)
- Missing features in BitBucket API that Prow relies on (e.g., specific check run states)
- Parallel test infrastructure for both providers

This would essentially be a rewrite of Prow's core, contradicting its design as a GitHub-native CI system. The effort would be equivalent to building a new CI system.

## Alternatives

For BitBucket-based CI/CD, consider:
- BitBucket Pipelines (native BitBucket CI)
- Jenkins with BitBucket plugin
- Generic CI systems that support multiple providers

/kind feature
/wontfix
```

**Rationale**:
- No /retitle: Title is accurate
- Added: Why this doesn't fit Prow's architecture
- Added: Alternatives (constructive response)
- Labels: kind/feature, wontfix (Level 4 - contradicts architecture)
- No area label: Not specific to any Prow component
- No difficulty label: This won't be implemented

## Important Notes

### Writing Style

- **Be precise**: Use specific file paths, function names, line numbers
- **Be concise**: 2-3 paragraphs maximum
- **Be additive**: Only include information not in the original issue
- **Be technical but accessible**: Explain technical concepts clearly
- **Be constructive**: Even when suggesting wontfix, explain why and offer alternatives

### Label Guidelines

- **Don't over-label**: Typically 1 area + 1 kind + maybe 1 difficulty
- **Be specific with area**: Use most specific component, not generic "core"
- **One kind only**: Issues are either bugs OR features, not both
- **Difficulty = invitation**: good-first-issue and help-wanted invite contributions; use only when appropriate
- **Priority is optional**: Only add when clearly warranted (security, data loss, blocking)

### Common Mistakes to Avoid

- ❌ Repeating information already in the issue
- ❌ Being overly verbose (keep it to 2-3 paragraphs)
- ❌ Suggesting /retitle for minor wording changes
- ❌ Adding good-first-issue to complex issues (Level 3-4)
- ❌ Adding priority labels unnecessarily
- ❌ Writing a novel instead of a comment (be concise!)
- ❌ Using jargon without explanation
- ❌ Proposing changes without consulting triage findings

### When to Skip Augmentation

Consider not posting an augmentation comment if:
- Issue is already very well written and complete
- PR already exists and is close to merging
- Issue will likely be closed as duplicate/wontfix
- Original reporter already provided all necessary context
- The only changes would be labels (just apply them without commentary)

In these cases, you might still update the triage document with your analysis but recommend not posting a public comment.

## Final Step: Review

Before committing, verify:
- [ ] Title change (if proposed) makes the issue clearer
- [ ] Added context is truly missing from original issue
- [ ] Comment is 2-3 paragraphs (not a wall of text)
- [ ] Labels are accurate based on triage findings
- [ ] Difficulty label matches effort assessment (L1→good-first-issue, L2→help-wanted, L3-4→none)
- [ ] Technical details are accurate (file paths, function names)
- [ ] Tone is helpful and constructive
- [ ] You would want to see this comment if you reported the issue
