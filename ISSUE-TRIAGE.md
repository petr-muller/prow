# Triage for Issue #572

**Status**: In Progress
**Created**: 2026-03-21

## Issue Information

- **Issue Number**: #572
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/572
- **Title**: Suggest similar commands when users type non-existent commands
- **Author**: kfess
- **Created**: 2025-12-10
- **Labels**: area/hook, area/plugins, kind/feature, lifecycle/stale
- **State**: OPEN

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests that Prow suggest similar valid commands when a user types a non-existent or incorrect command in a PR/issue comment. The example given: when a user types `/label release-note-none` (invalid label), Prow could suggest `/release-note-none` (correct command handled by a different plugin).

**Issue Category**: Feature Request

**Repository Scope Check**:
- Components mentioned: hook (dispatcher), plugins (label, releasenote)
- Exists in this repo: Yes
- Relevant code paths: hook server, plugin dispatch system, plugin command handling

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue clearly describes the problem, the current behavior, the expected behavior, and even suggests an implementation approach (Levenshtein distance)

### Recommendation

Keep open and continue triage. This is a valid feature request for improving the Prow user experience, particularly for new contributors. The maintainer has already commented with architectural analysis noting the complexity of the change due to how hook dispatches to plugins without centralized command parsing.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- **Hook Server**: `pkg/hook/server.go` - Receives GitHub webhooks, validates them, dispatches to handlers
- **Event Dispatcher**: `pkg/hook/events.go:357-378` - `handleGenericComment()` dispatches to all registered plugins in parallel goroutines
- **Plugin Registry**: `pkg/plugins/plugins.go:57-180` - Global maps of plugin handlers, registration functions
- **PluginHelp System**: `pkg/pluginhelp/pluginhelp.go` - Defines `Command` and `PluginHelp` structs that describe available commands
- **Label Plugin**: `pkg/plugins/label/label.go` - Handles `/label`, `/area`, `/kind`, etc. commands
- **Release Note Plugin**: `pkg/plugins/releasenote/releasenote.go` - Handles `/release-note-none`, etc.

**Architecture Overview**:

The hook server receives GitHub webhook events and converts them into a unified `GenericCommentEvent` type (defined at `pkg/github/types.go:1310-1328`). This event contains the raw comment `Body` and is dispatched to ALL registered `GenericCommentHandler` plugins simultaneously via goroutines. Each plugin independently parses commands from the body using its own regex patterns.

**Key Code Paths**:
1. Webhook reception: `pkg/hook/server.go:57-75` (`ServeHTTP`)
2. Event demux: `pkg/hook/server.go:77-183` (`demuxEvent`)
3. Generic comment dispatch: `pkg/hook/events.go:357-378` (`handleGenericComment`)
4. Plugin registration: `pkg/plugins/plugins.go:177-180` (`RegisterGenericCommentHandler`)
5. Label command parsing: `pkg/plugins/label/label.go:43-46` (regex patterns)
6. Label error response: `pkg/plugins/label/label.go:288-291` (error message for invalid labels)

**Data Flow**:
1. GitHub webhook → `ServeHTTP` → `demuxEvent`
2. `demuxEvent` → creates `GenericCommentEvent` with raw comment body
3. `handleGenericComment` → iterates all registered handlers for the repo
4. Each handler runs in its own goroutine: receives `GenericCommentEvent`, parses commands via regex
5. If a command matches, the plugin processes it; if not, the plugin silently ignores the event
6. No feedback to hook about which plugins successfully processed a command

### Related Code

**Command Parsing Patterns** (each plugin has its own regex):
- Label: `pkg/plugins/label/label.go:43` - `(?m)^/(area|committee|kind|language|priority|sig|triage|wg)\s*(.*?)\s*$`
- Custom label: `pkg/plugins/label/label.go:45` - `(?m)^/label\s*(.*?)\s*$`
- Release note: `pkg/plugins/releasenote/releasenote.go:64-67` - Separate regexes for each `/release-note-*` variant
- Approve: `pkg/plugins/approve/approve.go:52` - `(?m)^/([^\s]+)[\t ]*([^\n\r]*)`
- Help: `pkg/plugins/help/help.go:35-38` - `/help`, `/good-first-issue` patterns
- LGTM: `pkg/plugins/lgtm/lgtm.go:49-51` - `/lgtm`, `/remove-lgtm`

**PluginHelp Command Structure** (`pkg/pluginhelp/pluginhelp.go:22-36`):
- `Usage` (string): e.g., `/[remove-](area|committee|kind|...)`
- `Description` (string): what the command does
- `Examples` ([]string): example usages
- `Featured` (bool), `WhoCanUse` (string)

**Help Aggregation** (`pkg/pluginhelp/hook/hook.go:79-96`):
- `HelpAgent.generateNormalPluginHelp()` iterates all registered `HelpProviders()`
- Collects `PluginHelp` including `Commands` from every plugin
- This data is already available at runtime in the hook server

### Test Coverage

**Existing Tests**:
- `pkg/plugins/label/label_test.go`: Tests label application, removal, error messages for invalid labels
- `pkg/plugins/releasenote/releasenote_test.go`: Tests release note command handling
- `pkg/hook/events_test.go`: Tests event dispatching
- Coverage assessment: Good for individual plugin behavior, but no tests for cross-plugin command suggestion

**Test Gaps**:
- No tests for "unrecognized command" scenarios at the hook level
- No tests for cross-plugin command awareness

### Root Cause Analysis

**Primary Cause: Decentralized Command Processing Architecture**

The core issue is that Prow's plugin architecture intentionally decouples command parsing from the dispatcher. The hook server does not parse commands - it passes raw comment bodies to plugins. Each plugin independently decides whether a comment contains a command it handles. This means:

1. **No centralized command registry at dispatch time**: Hook knows which plugins are registered but doesn't know their commands
2. **No feedback loop**: Plugins don't report back whether they handled a command
3. **Silent failures**: If no plugin recognizes a command, nothing happens - no error, no suggestion

**Two Distinct Sub-Problems**:

1. **Completely unrecognized commands** (e.g., `/aprove` typo for `/approve`): No plugin matches, user gets zero feedback. A centralized "catch-all" could detect and suggest corrections.

2. **Cross-plugin confusion** (the issue's example: `/label release-note-none` vs `/release-note-none`): The label plugin *does* match the command but the argument is invalid. The label plugin reports the error but has no knowledge of other plugins' commands to suggest alternatives. Solving this requires cross-plugin awareness.

**Contributing Factors**:
1. `PluginHelp.Commands` contains the information needed for suggestions but is only used for the help endpoint, not at dispatch time
2. Plugin handlers run in parallel goroutines with no coordination
3. The `Command.Usage` field uses regex-like patterns (e.g., `/[remove-](area|...)`) rather than machine-parseable command names, making exact matching harder

### Proposed Solutions

#### Approach 1: Centralized Command Suggestion Plugin

**Description**: Create a new "command-suggestion" plugin that registers as a `GenericCommentHandler`. It would:
- Extract all `/command` patterns from the comment body
- Build a list of all known commands from `HelpProviders()` at startup
- For each extracted command, check if any known command matches
- For unmatched commands, use Levenshtein distance to find the closest known commands
- Post a "Did you mean...?" comment for close matches

**Pros**:
- Implemented as a standard plugin, no architectural changes needed
- Can leverage existing `PluginHelp.Commands` for the command registry
- Backwards compatible - opt-in via plugin configuration

**Cons**:
- Cannot detect Case 2 (valid command, wrong arguments) since both the label plugin and suggestion plugin run in parallel with no coordination
- The `Command.Usage` patterns are human-readable, not machine-parseable - need to extract actual command names from usage strings or examples
- May race with actual command handlers - user could see both an error from label plugin AND a suggestion

**Affected Components**: New plugin only, no changes to existing code

**Complexity**: Medium

**Backwards Compatibility**: Fully backwards compatible (opt-in plugin)

#### Approach 2: Hook-Level Command Interception

**Description**: Add centralized command parsing to the hook server's `handleGenericComment`:
1. Before dispatching to plugins, extract all `/command` patterns from the comment
2. Build command registry from `HelpProviders()` (cached)
3. After all plugin handlers complete, check which commands were "claimed" (requires new plugin interface: handlers return list of commands they processed)
4. For unclaimed commands, suggest closest matches

**Pros**:
- Clean centralized solution
- Can detect truly unhandled commands
- Single source of truth for command registry

**Cons**:
- Requires changing the `GenericCommentHandler` signature (breaking change to plugin interface)
- All plugins must be updated to report which commands they handled
- Complex coordination: must wait for all parallel handlers to complete before suggesting
- Still cannot easily detect Case 2 (label plugin "handles" the command but with an error)

**Affected Components**: `pkg/hook/events.go`, `pkg/plugins/plugins.go` (handler type), all plugins

**Complexity**: High

**Backwards Compatibility**: Breaking change to plugin handler interface

#### Approach 3: Enhanced Error Messages in Individual Plugins

**Description**: Improve error messages within specific plugins (starting with label plugin) to check whether failed arguments match commands from other plugins:
- When label plugin detects an invalid label like `release-note-none`, query `HelpProviders()` to check if `/release-note-none` is a valid command
- If found, include "Did you mean `/release-note-none`?" in the error message
- Each plugin that generates error messages could be enhanced similarly

**Pros**:
- Directly addresses the specific example in the issue
- No architectural changes
- Incremental improvement possible (one plugin at a time)
- The label plugin already has access to `plugins.Agent` which could be extended to provide command lookup

**Cons**:
- Only helps when a plugin *does* match and *does* produce an error
- Does nothing for completely unrecognized commands (the `/aprove` case)
- Coupling between plugins (label plugin needs to know about other plugins' commands)
- Each plugin must be individually enhanced

**Affected Components**: Individual plugins (primarily label plugin)

**Complexity**: Low

**Backwards Compatibility**: Fully backwards compatible

#### Recommendation

**Preferred Approach**: Approach 1 (Centralized Command Suggestion Plugin), possibly combined with Approach 3 for specific high-value cases.

Approach 1 solves the broadest class of the problem (unrecognized commands) without any architectural changes. It can be implemented as a new opt-in plugin. The main challenge is parsing machine-usable command names from the `PluginHelp.Commands` data, which currently uses human-readable `Usage` patterns.

Approach 3 is a good complement for the specific cross-plugin confusion case (`/label release-note-none` → `/release-note-none`), which Approach 1 cannot solve because the label plugin does claim the command.

**Key Implementation Considerations**:
1. Need to parse actual command names from `Command.Usage` strings or `Command.Examples` fields
2. A Levenshtein distance implementation (or similar) needs to be added - none exists in the codebase
3. The suggestion plugin should be conservative: only suggest when distance is small to avoid noise
4. Consider rate limiting / deduplication to avoid spamming on edited comments
5. The `Command.Examples` field is more reliably parseable than `Usage` for extracting command names

**Testing Requirements**:
- Unit tests for Levenshtein distance / fuzzy matching
- Unit tests for command extraction from PluginHelp
- Integration tests for the suggestion plugin with mock comment events
- Tests for edge cases: multiple commands in one comment, commands in code blocks (should be ignored)

## Effort Assessment

**Effort Level**: 3 - Large (requires expertise)

### Summary

Implementing command suggestions requires either a new plugin with cross-plugin awareness or architectural changes to the hook-plugin interface. The recommended approach (new suggestion plugin) avoids breaking changes but still requires understanding the plugin system deeply, building a command registry from `PluginHelp` data, implementing fuzzy matching, and handling concurrency concerns. The problem itself has significant design uncertainty with multiple viable approaches and trade-offs.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: Approach 1 (suggestion plugin): new plugin (1 file + tests ~300-500 LOC), possibly minor changes to `plugins.Agent` to expose help data. Approach 2 (hook-level): changes to `pkg/hook/events.go`, `pkg/plugins/plugins.go`, and every plugin handler (~20+ files).
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: High
- **Details**: Building a reliable command registry from `Command.Usage` patterns (which use regex-like notation, not plain command names) is non-trivial. Fuzzy matching must be tuned to avoid false positives. Concurrency with parallel plugin handlers creates race conditions for suggestion timing. Two distinct sub-problems (unrecognized commands vs cross-plugin confusion) require different solutions.
- **Level Indication**: 3-4

#### Required Expertise
- **Assessment**: Deep
- **Details**: Requires understanding of the hook dispatch architecture, the plugin registration system, how `PluginHelp` is aggregated, the parallel goroutine execution model, and how individual plugins parse commands. A contributor must understand *why* the architecture is decentralized to make good design decisions.
- **Level Indication**: 3-4

#### Clarity and Certainty
- **Assessment**: Some uncertainty
- **Details**: The problem is well-understood but the solution has significant design trade-offs. The maintainer's own comment on the issue highlights the architectural challenges. No single approach cleanly solves both sub-problems. Approach 1 leaves Case 2 (cross-plugin confusion) unsolved; Approach 2 requires breaking the plugin interface.
- **Level Indication**: 2-3

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Need unit tests for fuzzy matching, command extraction from `PluginHelp`, and the suggestion plugin itself. Existing test patterns for plugins can be followed. No new test infrastructure needed, but edge cases (commands in code blocks, multiple commands per comment, edited comments) require careful coverage.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible (Approach 1) / Breaking changes (Approach 2)
- **Details**: Approach 1 is opt-in via plugin configuration, no impact on existing deployments. Approach 2 would break the `GenericCommentHandler` type signature. Recommendation is to go with Approach 1.
- **Level Indication**: 1-2 (for recommended approach)

#### Architectural Alignment
- **Assessment**: Requires new patterns
- **Details**: No existing plugin performs cross-plugin awareness. The suggestion plugin would need to query `HelpProviders()` to build a command registry, which is a new pattern - currently only the help HTTP endpoint does this. The plugin would also need to handle the fact that it cannot know whether another plugin successfully handled a command (since they run in parallel with no coordination).
- **Level Indication**: 3

#### External Dependencies
- **Assessment**: None
- **Details**: No external system constraints. This is purely internal to Prow's plugin system. GitHub API is only used for posting comments (already well-supported).
- **Level Indication**: 1-3

### Recommended Labels

- [x] `kind/feature`: New functionality for command suggestion
- [x] `area/hook`: The dispatcher is central to the problem
- [x] `area/plugins`: Affects the plugin system
- [ ] `good-first-issue`: Too complex, requires deep architectural understanding
- [ ] `help-wanted`: Design uncertainty makes this hard for external contributors without maintainer guidance

### Guidance for Contributors

**For Level 3 (Large)**:
- Requires experience with Prow's plugin architecture, particularly the hook dispatch system
- Should review before starting:
  - `pkg/hook/events.go`: How `handleGenericComment` dispatches to plugins
  - `pkg/plugins/plugins.go`: Plugin registration and handler types
  - `pkg/pluginhelp/pluginhelp.go`: `Command` and `PluginHelp` structures
  - `pkg/pluginhelp/hook/hook.go`: How `HelpAgent` aggregates plugin commands
  - `pkg/plugins/label/label.go`: Example of command parsing and error messages
- Key architectural considerations:
  - The suggestion plugin runs in parallel with all other plugins - cannot wait for them to finish
  - `Command.Usage` patterns are human-readable, not machine-parseable - need reliable extraction of command names
  - Must handle commands inside code blocks (should be ignored, see `pkg/markdown/code_block.go`)
  - Must avoid suggesting when another plugin will handle the command correctly
- Should discuss design approach with maintainers before implementation

### Caveats and Considerations

- The effort could be reduced to Level 2 if scope is limited to *only* Approach 3 (enhancing label plugin error messages for the specific cross-plugin confusion case). This would be a targeted improvement that solves the exact example in the issue.
- A full solution (Approach 1 + 3) that covers both sub-problems is solidly Level 3.
- The issue has `lifecycle/stale` label, indicating limited community interest. The effort-to-impact ratio should be considered.

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title "Suggest similar commands when users type non-existent commands" is clear, specific, and accurate.

### Proposed GitHub Comment

```
Following up on my earlier architectural analysis with some concrete findings from a deeper code review.

There are actually **two distinct sub-problems** here that require different solutions:

1. **Completely unrecognized commands** (e.g., `/aprove` typo for `/approve`): No plugin matches, the user gets zero feedback. This is the easier case.
2. **Cross-plugin confusion** (the example in this issue: `/label release-note-none` vs `/release-note-none`): The label plugin *does* match the `/label` command but the argument is invalid. The label plugin then reports the error but has no knowledge of the `/release-note-none` command handled by the `releasenote` plugin. This is the harder case.

The good news is that Prow already has the data needed for command suggestions: every plugin provides a `PluginHelp` struct with a `Commands` field (defined in `pkg/pluginhelp/pluginhelp.go`) that includes `Usage` patterns and `Examples`. The `HelpAgent` in `pkg/pluginhelp/hook/hook.go` already aggregates this data from all plugins at runtime for the help endpoint. The missing piece is leveraging this data for command suggestion at dispatch time.

The most viable approach seems to be a **new "command-suggestion" plugin** that registers as a `GenericCommentHandler`, extracts all `/command` patterns from the comment body, compares them against a command registry built from `PluginHelp.Commands`, and posts a "Did you mean...?" comment for close matches using edit distance. This avoids any breaking changes to the plugin interface - it's opt-in via plugin configuration. The main challenge is that this plugin runs in parallel with all other plugins, so it cannot know whether another plugin will successfully handle a command. It would address sub-problem 1 (unrecognized commands) but not sub-problem 2 (cross-plugin confusion). For sub-problem 2, individual plugins (like `label`) could be enhanced to check whether a failed argument matches a known command from another plugin.

This is a **Level 3 (large/expert)** change due to the architectural complexity: cross-plugin awareness is a new pattern in Prow, the `Command.Usage` field uses human-readable regex-like patterns that need parsing, and there's significant design uncertainty. A contributor should discuss the approach with maintainers before starting.

/remove-lifecycle stale
```

### Rationale

**What's being added**:
- The two-sub-problem framing (not in original issue or prior comments) - helps contributors understand why this is harder than it looks
- Concrete reference to `PluginHelp.Commands` as the data source for a command registry - actionable information for would-be implementers
- A specific viable approach (suggestion plugin) that avoids architectural changes - builds on the architectural concerns raised in my earlier comment
- Effort level assessment (Level 3) - sets expectations for contributors
- Removal of stale lifecycle label to keep the issue active

**Why these labels**:
- `/remove-lifecycle stale`: The triage activity shows this issue is still being considered; remove stale label
- No new area/kind labels needed: `area/hook`, `area/plugins`, `kind/feature` already correctly applied
- No difficulty label: Level 3, not appropriate for `good-first-issue` or `help-wanted`

**What's NOT included**:
- No `/retitle`: Current title is already clear and specific
- No priority label: Nice-to-have feature enhancement, not urgent
- No detailed code implementation: Level 3 issue needs design discussion first, not prescriptive code guidance

## Briefing Completed

Briefed maintainer on: 2026-03-21

Key questions asked:
- None - maintainer acknowledged all slides without questions

Maintainer decision:
Proceed with wrapup.

## Wrapup

- Branches pushed to origin: 2026-03-21
- Comment posted: No (maintainer declined)
