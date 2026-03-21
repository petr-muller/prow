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

## Next Steps

(Action items will be added here)
