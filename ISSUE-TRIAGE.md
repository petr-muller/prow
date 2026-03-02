# Triage for Issue #634

**Status**: In Progress
**Created**: 2026-03-02

## Issue Information

- **Issue Number**: #634
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/634
- **Title**: release-note plugin: make documentation URL configurable
- **Author**: DanielBlei
- **Created**: 2026-02-27

## Issue Summary

The `release-note` plugin hardcodes the Kubernetes community release note guide URL (`https://git.k8s.io/community/contributors/guide/release-notes.md`) in both bot comments and the `/help` command output. This prevents projects using Prow outside of the Kubernetes ecosystem from pointing contributors to their own release note process documentation.

The proposed solution is to add a configurable `url` field to the `release_note` plugin configuration, defaulting to the existing Kubernetes URL for backwards compatibility.

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests making a hardcoded Kubernetes-specific URL configurable in the `release-note` plugin. The URL `https://git.k8s.io/community/contributors/guide/release-notes.md` appears in exactly two locations in `pkg/plugins/releasenote/releasenote.go`:

1. **Line 41** — `releaseNoteFormat` string: shown in bot comments when no release-note block is detected
2. **Line 87** — `helpProvider()`: shown in the `/release-note-none` command description via `/help`

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: `release-note` plugin
- Exists in this repo: Yes (`pkg/plugins/releasenote/releasenote.go`)
- Relevant code paths: `pkg/plugins/releasenote/releasenote.go`, `pkg/plugins/config.go`

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue clearly identifies the problem, the specific hardcoded URL, both affected locations, and proposes a concrete solution with backwards compatibility
- Real-world motivation provided via KubeVirt cross-reference

### Recommendation

This is a well-written, actionable feature request. The `release-note` plugin currently has no configuration struct in `pkg/plugins/config.go`, so a new one would need to be added. The change is straightforward and the author's proposed approach (configurable URL with default) is the standard Prow pattern for this kind of customization.

Labels `kind/feature` and `area/plugins` have already been applied by a maintainer.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- `pkg/plugins/releasenote/releasenote.go` — The plugin itself. Handles PR body parsing for release-note blocks, label management, bot comments, and `/release-note-*` slash commands.
- `pkg/plugins/config.go` — Central plugin configuration. Currently has **no** config struct for the release-note plugin.

**Architecture Overview**:
The release-note plugin registers two event handlers (`handleIssueComment` and `handlePullRequest`) and a `helpProvider`. The hardcoded URL appears in two contexts:
1. A package-level format string `releaseNoteFormat` (line 41) used to build the `releaseNoteBody` variable (line 49), which is posted as a bot comment when a PR lacks a release-note block.
2. An inline HTML link in `helpProvider()` (line 87), embedded in the `/release-note-none` command description.

Both are compile-time constants/variables, meaning the URL cannot currently be changed without modifying source code.

**Key Code Paths**:
1. `releasenote.go:41` — `releaseNoteFormat` const with hardcoded URL
2. `releasenote.go:49` — `releaseNoteBody` package-level var (computed from `releaseNoteFormat`)
3. `releasenote.go:75-98` — `helpProvider()` with hardcoded URL in `/release-note-none` description
4. `releasenote.go:284` — `releaseNoteBody` used in `handlePR()` to post the comment
5. `releasenote.go:332-333` — `releaseNoteBody` used in `clearStaleComments()` to identify stale bot comments

**Data Flow**:
- `handlePullRequest` → `handlePR()` → if PR body has no release-note block, posts `releaseNoteBody` comment (which contains the hardcoded URL)
- `helpProvider()` → called by Deck to render `/help` page → returns plugin help with hardcoded URL in command description
- `clearStaleComments()` → uses `releaseNoteBody` string to match and delete stale bot comments

### Related Code

**Precedent Plugins with Configurable URLs**:
- **Help plugin** (`config.go:101-118`): Has `HelpGuidelinesURL` field with `setDefaults()` method that defaults to `https://git.k8s.io/community/contributors/guide/help-wanted.md`. This is the closest pattern to follow.
- **Approve plugin** (`config.go:324-350`): Has `CommandHelpLink` and `PrProcessLink` fields with defaults set in the `ApproveFor()` accessor function.
- **Trigger plugin** (`config.go:486-514`): Has `JoinOrgURL` field with conditional default in `SetDefaults()`.

**Configuration Default Orchestration**:
- `config.go:1149-1209` — `Configuration.setDefaults()` calls individual plugin `setDefaults()` methods. A new `ReleaseNote.setDefaults()` would be wired in here.

**Plugin Agent Pattern**:
- `handleIssueComment` and `handlePullRequest` receive a `plugins.Agent` which carries `pc.PluginConfig` — the configuration can be threaded through to the inner handler functions.
- `helpProvider` already receives `*plugins.Configuration` as its first parameter (currently ignored via `_`).

### Test Coverage

**Existing Tests**: `pkg/plugins/releasenote/releasenote_test.go`
- Coverage assessment: Good for existing functionality
- Tests use `fakegithub.NewFakeClient()` and table-driven patterns
- Tests verify label management, comment creation, stale comment cleanup, cherry-pick handling
- Tests do NOT currently pass any `plugins.Configuration` since the plugin has no config

**Test Gaps**:
- No tests for configurable URL (doesn't exist yet)
- Once added, tests should verify: default URL used when config empty, custom URL used when configured, custom URL appears in both bot comments and help output

### Root Cause Analysis

**Primary Cause**:
The release-note plugin predates the plugin configuration infrastructure. When it was written, all Prow plugins were Kubernetes-specific, so the URL was hardcoded as a constant. As Prow has been adopted by other projects (KubeVirt, Istio, etc.), this has become a limitation.

**Contributing Factors**:
1. The URL is baked into a `const` and a package-level `var`, making it impossible to override at runtime
2. The plugin has no configuration struct at all — it's one of the simpler plugins that never needed config
3. The `helpProvider` function ignores its `*plugins.Configuration` parameter

### Proposed Solutions

#### Approach 1: Config Struct with setDefaults (Recommended)

**Description**: Follow the Help plugin pattern. Add a `ReleaseNote` config struct to `config.go` with a URL field and a `setDefaults()` method. Thread the config through to `handlePR`, `handleComment`, and `helpProvider`.

**Changes Required**:
1. `pkg/plugins/config.go`: Add `ReleaseNote` struct with `ReleaseNoteGuidelinesURL string` field, add `setDefaults()` method, add field to `Configuration` struct, wire into `Configuration.setDefaults()`
2. `pkg/plugins/releasenote/releasenote.go`: Change `releaseNoteFormat`/`releaseNoteBody` from package-level const/var to a function that takes the URL as parameter. Update `helpProvider` to read URL from config. Thread config through handler functions.
3. `pkg/plugins/releasenote/releasenote_test.go`: Add test cases for default URL and custom URL behavior.

**Pros**:
- Follows established Prow pattern (identical to Help plugin)
- Backwards compatible (default preserves current behavior)
- Clean separation of config from logic

**Cons**:
- Requires refactoring the package-level `releaseNoteBody` var into a function
- `clearStaleComments` uses `releaseNoteBody` for string matching, which needs care during transition (old comments with old URL still need to be matchable)

**Complexity**: Low

**Backwards Compatibility**: Full. Empty/missing config defaults to existing K8s URL.

#### Stale Comment Matching Consideration

The `clearStaleComments` function (line 316) matches bot comments by checking if they contain `releaseNoteBody`. If the URL changes, old comments posted with the previous URL won't match the new `releaseNoteBody` string. This is a minor issue — worst case, old stale comments won't be auto-cleaned. This is acceptable and consistent with how other plugins handle similar transitions.

#### Recommendation

**Preferred Approach**: Approach 1 (Config struct with setDefaults)

**Key Implementation Considerations**:
1. Follow the Help plugin pattern exactly for consistency
2. The `releaseNoteBody` package-level var must become dynamic (a function or computed per-request from config)
3. `helpProvider` needs to read from `*plugins.Configuration` instead of ignoring it
4. Consider naming the field `ReleaseNoteGuidelinesURL` to match the Help plugin's `HelpGuidelinesURL` pattern
5. Stale comment matching is a minor edge case that doesn't need special handling

**Testing Requirements**:
- Default URL used when config field is empty
- Custom URL used when configured
- Custom URL appears in bot comments
- Custom URL appears in help provider output
- Stale comment cleanup still works with default URL

## Next Steps

- Assess effort level
- Augment the issue with findings
