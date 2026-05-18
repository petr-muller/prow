---
issue: kubernetes-sigs/prow#572
state: closed
labels: area/hook,area/plugins,kind/feature,lifecycle/rotten
triaged_at: 2026-05-18T15:30:00Z
main_sha: 8db280f94
verdict: legitimate
refresh_log:
  - at: 2026-04-23T00:00:00Z
    summary: Initial triage completed; issue was open with lifecycle/rotten
---

# Triage for Issue #572

**Issue:** [#572 - Suggest similar commands when users type non-existent commands](https://github.com/kubernetes-sigs/prow/issues/572)
**Status:** CLOSED (not-planned) — auto-closed 2026-05-18
**Created:** 2026-04-21
**Assessment:** LEGITIMATE
**Effort Level:** 3 - Large (requires expertise)
**Triage Branch:** [572-triage](https://github.com/petr-muller/prow/blob/572-triage/ISSUE-TRIAGE.md)

---

## Initial Validation

**Assessment:** LEGITIMATE
**Category:** Feature Request

### Repository Scope Check

- **Components mentioned:** hook, plugins (specifically label and releasenote plugins)
- **Exists in this repo:** Yes
- **Relevant code paths:** prow/cmd/hook/, prow/plugins/

### Analysis

This is a well-articulated feature request to improve Prow's user experience by suggesting similar commands when users make typos. The issue demonstrates a real UX problem: new contributors who type `/label release-note-none` (incorrect) receive a confusing error message listing available labels, when they should have typed `/release-note-none` (correct command).

The issue has already been triaged by maintainer @petr-muller who:
- Applied appropriate labels (area/hook, area/plugins, kind/feature)
- Acknowledged the value of the feature
- Explained architectural challenges: hook dispatches to plugins without parsing commands, and plugins don't know about each other's commands
- Suggested a significant restructuring would be needed for proper implementation
- Decided to keep it open to gather interest and ideas

---

## Code Research

### Primary Components

- `pkg/hook/server.go` - Receives GitHub webhooks and dispatches to plugins
- `pkg/hook/events.go:357-378` - Launches plugin handlers in parallel goroutines
- `pkg/plugins/plugins.go` - Global registries for handlers and help providers
- `pkg/pluginhelp/pluginhelp.go` - Command and PluginHelp struct definitions
- `pkg/plugins/label/label.go` - Example plugin handling `/label` commands
- `pkg/plugins/releasenote/releasenote.go` - Handles `/release-note-none` commands

### Architecture Overview

Prow's hook component follows a simple dispatcher-handler pattern:

1. Hook receives GitHub webhook events (comments, issues, PRs, etc.)
2. Hook deserializes JSON into typed event structs (GenericCommentEvent, etc.)
3. Hook determines which plugins are enabled for the org/repo
4. Hook spawns **parallel goroutines**, one per plugin, passing the full event
5. Each plugin independently parses the comment body using its own regex patterns
6. Plugins execute their logic and respond to GitHub

**Critical architectural constraints:**
- Hook does NOT parse commands - it only routes events
- Hook does NOT know about plugin commands during dispatch
- Plugins are completely independent - they don't know about each other
- Each plugin defines its own regex patterns for command matching

### Root Cause Analysis

The current architecture was designed for **plugin independence and simplicity**:
- Hook acts as a dumb dispatcher - minimal logic, just routing
- Each plugin is self-contained - easier to develop and test
- No shared command parsing - plugins have full control over their syntax
- No cross-plugin coordination - simpler, no deadlocks or ordering issues

**Why adding command suggestions is challenging:**

1. **Hook doesn't parse commands:** Hook only knows about event types, not command content
2. **No centralized command registry:** Command information exists only in HelpProviders, called separately for the help endpoint
3. **Help info disconnected from dispatch:** HelpProviders generate Command structs, but these aren't available during event handling
4. **Plugins don't know about each other:** Label plugin can't suggest "did you mean `/release-note-none`?"
5. **Parallel dispatch complicates coordination:** All plugins run concurrently, hard to aggregate "no one handled this"

**The example case is particularly hard:** `/label release-note-none` IS a valid command (handled by label plugin), but "release-note-none" is not a valid label name. The correct alternative `/release-note-none` is handled by a different plugin (releasenote) that label has no knowledge of.

### Proposed Solutions

#### Approach 1: Plugin-Level Suggestions (Partial)

Each plugin suggests similar commands from its own command set when it detects an error.

**Pros:** Minimal architectural changes, incremental adoption, no cross-plugin coordination needed
**Cons:** Doesn't solve cross-plugin confusion, requires modifying every plugin, limited scope
**Complexity:** Low-Medium

#### Approach 2: Centralized Command Registry with Hook-Level Suggestions

Build a centralized command registry that hook can access during dispatch. When all plugins finish and none took action, hook analyzes the comment and suggests similar commands.

**Pros:** Provides cross-plugin suggestions, centralized logic, comprehensive solution
**Cons:** Significant architectural refactoring, need "did any plugin handle this?" coordination, hook becomes more complex
**Complexity:** High

#### Approach 4: Shared Fuzzy Matching Utility

Provide a shared string distance utility package that plugins can optionally use to enhance their error messages.

**Pros:** Minimal changes, opt-in adoption, keeps plugin independence intact
**Cons:** Doesn't solve cross-plugin confusion, inconsistent adoption
**Complexity:** Low

#### Recommended Approach

**Approach 2 (Centralized Command Registry) with Approach 4 (Shared Utility) as a first step**

- **Phase 1:** Implement Approach 4 (shared utility) to provide quick wins for intra-plugin suggestions
- **Phase 2:** Implement Approach 2 (command registry) for cross-plugin suggestions

---

## Effort Assessment

**Effort Level: 3 - Large (requires expertise)**

*Note: An incremental partial solution (Approach 4: Shared Utility) would be Level 2.*

### Factor Analysis

| Factor | Assessment | Level Indication |
|--------|-----------|-----------------|
| Scope of Changes | Moderate-Large: ~5-10 files, 200-400 LOC | 2-3 |
| Complexity | Moderate-High: Command registry design, coordination | 2-3 |
| Required Expertise | Moderate-Deep: Hook/plugin architecture understanding | 2-3 |
| Clarity and Certainty | Well-defined with some implementation uncertainty | 2-3 |
| Testing Requirements | Moderate: Unit and integration tests needed | 2-3 |
| Backwards Compatibility | Fully compatible: Additive only | 1-2 |
| Architectural Alignment | Good fit with new patterns required | 2-3 |
| External Dependencies | None (minor library consideration) | 1-3 |

### Recommended Labels

- ✓ `kind/feature` — Already applied, appropriate
- ✓ `area/hook` — Already applied, appropriate
- ✓ `area/plugins` — Already applied, appropriate
- ✗ `good-first-issue` — Not appropriate (requires architectural expertise)
- ? `help-wanted` — Could apply if scope reduced to Approach 4 only
- ✓ `lifecycle/frozen` — Remove lifecycle/rotten, prevent auto-close
- ✓ `priority/important-longterm` — Improves contributor UX but not urgent

---

## Proposed Issue Augmentation

### Title Change

**No change needed:** Current title "Suggest similar commands when users type non-existent commands" is clear and specific.

### Proposed GitHub Comment

---

**Architectural Context**

@petr-muller's assessment is correct - this requires rethinking the hook/plugin interface. Currently, hook operates as a pure dispatcher in `pkg/hook/events.go:357-378`: it receives comment events and launches all enabled plugins in parallel goroutines without parsing the comment body. Each plugin independently parses commands using its own regex patterns (e.g., `pkg/plugins/label/label.go:39-46` defines regex for `/label`, `/area`, etc.). Critically, the `PluginHelp` structure with command information (`pkg/pluginhelp/pluginhelp.go:22-36`) is only used for the `/plugin-help` HTTP endpoint, not during event dispatch.

**Incremental Implementation Path**

Rather than a full architectural refactoring, this could be tackled incrementally:

**Phase 1 (Moderate complexity):** Create a shared fuzzy matching utility in `pkg/plugins/suggestion/` with Levenshtein distance calculation. Individual plugins could then enhance their error messages to suggest similar commands from their own command sets (e.g., `/aera` → "Did you mean `/area`?"). This solves many common typos without touching hook's architecture.

**Phase 2 (Requires expertise):** Build a centralized command registry alongside existing HelpProviders. After all plugins finish processing, hook could check if any plugin took action (using the existing Agent.TookAction mechanism), and if not, extract potential commands from the comment and suggest similar ones using the registry. This enables cross-plugin suggestions like the `/label release-note-none` → `/release-note-none` example.

The plugin `pkg/plugins/releasenote/releasenote.go` and its interaction with `label` plugin demonstrates the cross-plugin confusion this would address.

```
/lifecycle frozen
/priority important-longterm
```

---

## Briefing Completed

**Briefed maintainer on:** 2026-04-23
**Questions asked:** None — maintainer followed full walkthrough without questions.
**Maintainer decision:** Proceeding to wrapup phase.

## Comment Not Posted

**Decision:** Maintainer chose not to post augmentation comment on 2026-04-23.

The proposed comment is documented above in "Proposed Issue Augmentation" for future reference.

## Since Previous Triage (2026-04-23 → 2026-05-18)

- **2026-05-18:** `k8s-triage-robot` closed the issue as "not-planned" via `/close not-planned` after the 30-day lifecycle/rotten inactivity timer expired.
- **2026-05-18:** `k8s-ci-robot` confirmed closure.
- No human comments, no new cross-references, no linked PRs.

The augmentation comment (which would have applied `/lifecycle frozen`) was never posted, so the bot's auto-close proceeded as expected given the lifecycle/rotten label that had been on the issue since 2026-04-18.

## Next Steps

The triage analysis remains valid — this is a legitimate Level 3 feature request. The closure was bot-driven due to inactivity, not a maintainer decision. Options:

1. **Reopen and apply labels:** Reopen the issue with `/reopen`, post the prepared augmentation comment (applying `/lifecycle frozen` and `/priority important-longterm`) to prevent further auto-close. Suitable if the feature is worth keeping alive for an experienced contributor.
2. **Accept the closure:** Leave closed as "not-planned" if the implementation effort doesn't justify the benefit in the current contributor pool. The triage document captures the architectural analysis for future reference.
3. **Split into Phase 1:** Open a new, narrower issue for the shared fuzzy matching utility (Approach 4, Level 2 effort), which could attract a help-wanted contributor without requiring architectural expertise.

---

*Generated by [Claude triage helper](https://github.com/petr-muller/prow/blob/claude-maintenance-helpers/.claude/skills/maintenance-issue-triage/SKILL.md) · Full triage: [ISSUE-TRIAGE.md](https://github.com/petr-muller/prow/blob/572-triage/ISSUE-TRIAGE.md)*
