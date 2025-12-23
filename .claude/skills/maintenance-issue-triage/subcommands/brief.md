# Subcommand: maintenance::issues::triage::brief

## Purpose

Provide an interactive, step-by-step briefing of triage findings to a human maintainer. This subcommand presents information in a concise, slide-like format, similar to an engineer walking an architect through their analysis. The briefing is designed to take approximately 3 minutes for an experienced engineer to walk through and understand.

**Key characteristics**:
- **Interactive**: Present information in steps, wait for user acknowledgment between slides
- **Concise**: Brief bullet points, no walls of text
- **Technical but focused**: Enough detail to understand, not so much to overwhelm
- **Slide-like**: Each step is like a presentation slide with 3-5 key points
- **Conversational**: Like a colleague briefing you, not reading a document

## Parameters

- `issue_number` (required): The GitHub issue number to brief on

## Instructions

You are briefing a maintainer on the triage findings for GitHub issue #{issue_number}.

### Briefing Structure

Present the information in 7 steps (slides). After presenting each slide, **wait for the user to acknowledge** (e.g., "next", "continue", "ok") before proceeding to the next slide. If the user asks questions, answer them before moving on.

---

## Slide 1: Issue Overview (30 seconds)

**Present**:

```
📋 ISSUE OVERVIEW: #{issue_number}

Title: [issue title]
Reported by: [author] on [date]
Current status: [OPEN/CLOSED]

Quick summary:
[1-2 sentence description of what the issue is about]

[If relevant: "Note: PR #XXX is already working on this"]
```

**Then ask**: "Ready for the legitimacy assessment?"

**Wait for user response before continuing.**

---

## Slide 2: Legitimacy Assessment (15 seconds)

**Present**:

```
✅ LEGITIMACY: [LEGITIMATE / NEEDS_INFO / CLOSE / REDIRECT]

[If LEGITIMATE:]
- Valid [bug/feature] for Prow
- [Component] component exists in this repo
- Sufficient information provided
- [One key reason why it's legitimate]

[If NEEDS_INFO:]
- Potentially valid but missing: [what's missing]
- Suggest requesting: [specific information needed]

[If CLOSE:]
- Reason: [misconfiguration / wrong repo / duplicate / etc.]
- Suggest closing with: [brief explanation]

[If REDIRECT:]
- Belongs in: [other repository]
- Reason: [why it belongs there]
```

**Then ask**: "Want to hear about the root cause?" (or if not legitimate: "Want to see the proposed response?")

**Wait for user response before continuing.**

---

## Slide 3: Root Cause Analysis (30 seconds)

**Note**: Only present this slide if the issue is LEGITIMATE. Skip to recommendations for CLOSE/REDIRECT.

**Present**:

```
🔍 ROOT CAUSE

Primary cause:
[1-2 sentence explanation of what's causing the issue]

Why it happens:
- [Key factor 1]
- [Key factor 2]
- [Key factor 3 if applicable]

Conditions to reproduce:
- [Condition 1]
- [Condition 2]
```

**Then ask**: "Ready for technical details?"

**Wait for user response before continuing.**

---

## Slide 4: Technical Analysis (30 seconds)

**Present**:

```
⚙️ TECHNICAL DETAILS

Affected components:
- [Component 1]: [file:line-range] - [what it does]
- [Component 2]: [file:line-range] - [what it does]

Key code paths:
1. [Brief description of path 1]
2. [Brief description of path 2]

Current behavior:
[1 sentence describing current broken/missing behavior]

Test coverage:
[Good / Partial / Missing] - [brief note]
```

**Then ask**: "Want to hear the solution approach?"

**Wait for user response before continuing.**

---

## Slide 5: Solution Approach (30 seconds)

**Present**:

```
💡 SOLUTION

Recommended approach: [Approach name]

How it works:
[2-3 sentence high-level description]

Pros:
- [Key advantage 1]
- [Key advantage 2]

Cons / Trade-offs:
- [Key consideration 1]
- [Key consideration 2]

Scope:
- [N] files affected
- [~N] lines of code
- [Backwards compatible: Yes/No]
```

**Then ask**: "Ready for the effort assessment?"

**Wait for user response before continuing.**

---

## Slide 6: Effort Assessment (30 seconds)

**Present**:

```
📊 EFFORT LEVEL: [1/2/3/4] - [Easy/Moderate/Large/Very Large]

Complexity: [Simple/Moderate/High]
- [One key complexity factor]

Required expertise:
- [Key skill/knowledge area 1]
- [Key skill/knowledge area 2]

Suitable for:
[Level 1: "New contributors with guidance (good-first-issue)"]
[Level 2: "Skilled contributors familiar with [area] (help-wanted)"]
[Level 3: "Experienced Prow contributors only"]
[Level 4: "Requires significant design work / Not recommended"]

Estimated scope: [Small/Medium/Large]
```

**Then ask**: "Ready for recommendations?"

**Wait for user response before continuing.**

---

## Slide 7: Recommendations (15 seconds)

**Present**:

```
🎯 RECOMMENDATIONS

Proposed action:
[One clear action: "Merge PR #XXX" / "Apply labels and wait for contributor" / "Close as wontfix" / etc.]

Labels to apply:
- /area [component]
- /kind [type]
- [/good-first-issue OR /help-wanted OR neither]
- [/priority [level] - if applicable]

[If augmentation comment proposed:]
GitHub comment:
- Add context: [brief what context]
- [Retitle: Yes/No]

Next steps:
1. [Most important next action]
2. [Second priority action]
3. [Third priority action if applicable]
```

**Then say**: "That completes the briefing. Any questions or would you like me to elaborate on any section?"

---

## Interaction Guidelines

### Pacing

- **Wait for acknowledgment** between each slide
- Don't rush - let the user absorb each slide
- If user asks a question, answer it before proceeding
- If user wants to skip ahead, allow it
- If user wants to go back, allow it

### User Responses

Common responses and how to handle them:

**"next" / "continue" / "ok" / "go on"**:
→ Present the next slide

**"skip" / "skip this"**:
→ Acknowledge and move to next slide

**"back" / "go back"**:
→ Re-present the previous slide

**"more details on [topic]"**:
→ Provide additional detail from the triage document, then ask if they want to continue

**"pause" / "wait"**:
→ Acknowledge and wait for them to say "continue"

**Question about specific detail**:
→ Answer the question based on triage findings, then ask if they want to continue

**"skip to recommendations"**:
→ Jump directly to Slide 7

### Tone and Style

- **Conversational**: Like talking to a colleague, not presenting a formal report
- **Confident**: You've done the research, present findings clearly
- **Concise**: Bullet points, not paragraphs
- **Technical**: Use proper terminology, file paths, function names
- **Respectful**: This is a briefing, not a lecture - acknowledge questions

### Example Conversation Flow

```
Agent: 📋 ISSUE OVERVIEW: #337

Title: Tide merges PR when retesting GitHub action
Reported by: saschagrunert on 2024-12-03
Current status: OPEN

Quick summary:
Tide occasionally merges PRs when GitHub Actions are re-triggered, even though the required checks haven't completed yet. This is a race condition in Tide's merge decision logic.

Note: PR #563 is already working on a fix.

Ready for the legitimacy assessment?

User: yes

Agent: ✅ LEGITIMACY: LEGITIMATE

- Valid bug for Prow
- Tide component exists in this repo
- Sufficient information provided
- Reporter included example PR and suspected code location

Want to hear about the root cause?

User: sure

Agent: 🔍 ROOT CAUSE

Primary cause:
When GitHub Actions are re-triggered, GitHub temporarily removes the old CheckRun before creating the new one. Tide's sync loop may run during this window and see "no failures", incorrectly proceeding with merge.

Why it happens:
- Tide doesn't track previously-seen contexts
- Can't distinguish "never existed" from "disappeared"
- GitHub's CheckRun removal creates timing window
- Tide interprets missing required check as "not failed"

Conditions to reproduce:
- Required GitHub Action check exists and passed
- User manually re-triggers the check
- Tide sync occurs during CheckRun removal window

Ready for technical details?

User: what's the timing window?

Agent: The timing window is typically a few seconds - the time between when GitHub removes the old CheckRun from its API and when the new CheckRun appears. It varies based on GitHub's processing time, but it's long enough that Tide's periodic sync (typically every 30-60 seconds) has a reasonable chance of hitting it when checks are re-triggered.

Still want the technical details?

User: yes

Agent: ⚙️ TECHNICAL DETAILS

Affected components:
- Context checker: pkg/tide/tide.go:865-889 - Evaluates if contexts are successful
- GitHub provider: pkg/tide/github.go:333-392 - Fetches check status from API
- CheckRun converter: pkg/tide/tide.go:2200-2216 - Converts GitHub CheckRuns to Tide contexts

Key code paths:
1. Tide sync → isPassingTests() → unsuccessfulContexts() checks for failures
2. Missing required contexts added to "failed" list, but only if never seen before

Current behavior:
When context disappears during re-trigger, it's not in the "unsuccessful" list, so merge proceeds.

Test coverage:
Partial - basic status checking tested, but not the re-trigger race scenario

Want to hear the solution approach?

User: go ahead

[... continues through remaining slides ...]
```

---

## Special Cases

### PR Already Exists

If a PR is already working on the fix (like #563 in our example):
- Mention it in Slide 1 (Overview)
- In Slide 5 (Solution), say "This approach is implemented in PR #XXX"
- In Slide 7 (Recommendations), suggest reviewing/merging the PR

### Issue Should Be Closed

If legitimacy assessment is CLOSE or REDIRECT:
- Present Slides 1 and 2 as normal
- **Skip Slides 3, 4, 5, 6** (no root cause needed for invalid issues)
- Jump to Slide 7 with recommendations for closing/redirecting

### Very Simple Issues (Level 1)

For good-first-issue candidates:
- Keep all slides but make them even more concise
- Emphasize in Slide 6 that it's suitable for new contributors
- In Slide 7, highlight what guidance should be provided

### Very Complex Issues (Level 4)

For near-impossible issues:
- Emphasize in Slide 3 (Root Cause) why it's fundamentally hard
- In Slide 5 (Solution), explain the architectural challenges
- In Slide 6, be clear this requires design work or may not fit Prow
- In Slide 7, consider suggesting alternatives or closing as wontfix

---

## Preparation

Before starting the briefing:

1. **Read the entire ISSUE-TRIAGE.md** to understand all findings
2. **Extract key points** for each slide (don't just copy paragraphs)
3. **Identify the story arc**: What's the most important thing for the maintainer to understand?
4. **Prepare for questions**: Know where details are in case user asks
5. **Check for special cases**: PR exists? Should be closed? Very simple/complex?

---

## After the Briefing

Once all slides are presented and questions answered:

### Update Triage Document

Add a brief note to `ISSUE-TRIAGE.md`:

```markdown
## Briefing Completed

Briefed maintainer on: [date/time]

Key questions asked:
- [Question 1 and brief answer]
- [Question 2 and brief answer]

Maintainer decision:
[What the maintainer decided to do, if they indicated a decision]
```

### Commit

```bash
git add ISSUE-TRIAGE.md
git commit -m "Completed triage briefing for issue #{issue_number}"
```

---

## Tips for Effective Briefings

1. **Start strong**: Slide 1 should immediately orient the maintainer
2. **Be ready to go deeper**: Have triage doc open to answer questions
3. **Read the room**: If maintainer is engaged, can provide more detail; if rushed, stay high-level
4. **Emphasize decisions**: What needs to be decided? What's the recommendation?
5. **Connect the dots**: Each slide should flow logically to the next
6. **End with action**: Slide 7 should be crystal clear on what happens next

Remember: You're helping a busy maintainer quickly understand an issue they haven't looked at yet. Make every word count.
