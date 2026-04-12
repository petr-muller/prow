# Triage for Issue #658

**Status**: In Progress
**Created**: 2026-04-12

## Issue Information

- **Issue Number**: #658
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/658

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests a Deck feature to surface debugging hints from job artifacts. Specifically, the proposal is to render a `$ARTIFACTS/DEBUGGING.md` file as a semi-collapsed viewer in Spyglass test results pages, helping contributors (especially new ones) discover useful debug artifacts beyond raw build logs.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Deck (Spyglass UI for viewing job results)
- Exists in this repo: Yes (`cmd/deck/`, `pkg/spyglass/`, `pkg/spyglass/lenses/`)
- Relevant code paths: Spyglass lens system in `pkg/spyglass/lenses/`

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue describes the problem (artifacts are powerful but unintuitive), the proposed UX (semi-collapsed rendered viewer near top of results), and concrete examples of what would go in the debugging guide
- Missing information: None critical — implementation details would be determined during development

**Author context**: BenTheElder (Benjamin Elder) is a well-known Kubernetes project member and SIG Testing contributor. The discussion includes thoughtful comments from pohly (concerns about job maintainer burden) and petr-muller (noting OpenShift's existing HTML lens approach as prior art).

### Recommendation

Keep open and continue triage. This is a well-articulated feature request for Deck's Spyglass component from a credible project member. The feature would improve the contributor experience by making debug artifacts more discoverable. The discussion already contains useful design considerations (automatic vs manual generation, shared tooling across job types, prior art from OpenShift's HTML lens).

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- Spyglass lens system: `pkg/spyglass/lenses/lenses.go` — lens registration, `Lens` interface
- Lens API: `pkg/spyglass/api/spyglass.go` — `Lens` and `Artifact` interfaces
- Spyglass controller: `pkg/spyglass/spyglass.go` — artifact listing, lens ordering
- Deck integration: `cmd/deck/main.go` — artifact-to-lens matching (lines 1031-1057), lens handler init (lines 708-745)
- Frontend: `cmd/deck/static/spyglass/spyglass.ts` — iframe-based lens loading
- Page template: `cmd/deck/template/spyglass.html` — Spyglass page layout

**Architecture Overview**:
Spyglass uses a plugin-based lens system. Each lens is a Go package implementing `Header()`, `Body()`, and `Callback()` methods. Lenses register via `init()` functions and are imported as side-effect imports in `cmd/deck/main.go`. Configuration maps artifact filename regex patterns to lenses. When a user views a job, Spyglass lists all artifacts, matches them against lens configs (RequiredFiles=AND, OptionalFiles=OR), and renders matching lenses as iframes ordered by priority.

**Key Code Paths**:
1. Lens interface: `pkg/spyglass/lenses/lenses.go:54-77` — `Config()`, `Header()`, `Body()`, `Callback()`
2. Lens registration: `pkg/spyglass/lenses/lenses.go:85-98` — `RegisterLens()` into global registry
3. Artifact matching: `cmd/deck/main.go:1031-1057` — regex matching of artifact names to lens configs
4. Lens ordering: `pkg/spyglass/spyglass.go:106-142` — priority-based sorting
5. Lens serving: `pkg/spyglass/lenses/common/common.go:102-155` — HTTP handler wrapping lens calls

**Existing Lenses** (registered in `cmd/deck/main.go:88-96`):
- `buildlog` — plain text log rendering with error highlighting (priority 10)
- `html` — renders HTML artifacts in sandboxed iframe (priority 3, HideTitle=true)
- `junit` — JUnit XML test results
- `metadata` — structured JSON from started.json/finished.json (priority 0)
- `links`, `podinfo`, `coverage`, `restcoverage` — specialized viewers

**Data Flow**:
1. User navigates to Spyglass view → `renderSpyglass()` in deck
2. `sg.ListArtifacts()` lists all artifacts for the job
3. Each `LensFileConfig` checked: all `RequiredFiles` regexes must match ≥1 artifact
4. Matching artifacts collected, lens added to render list
5. Lenses sorted by priority, rendered as iframes via `spyglass.html` template
6. Each iframe POSTs to `/spyglass/lens/{name}/iframe` → lens `Header()`+`Body()` called
7. Lens reads artifact content via `Artifact.ReadAll()` and renders HTML

### Related Code

**Configuration structs** (`pkg/config/config.go`):
- `Spyglass` struct (line 1159): top-level config with `Lenses []LensFileConfig`
- `LensFileConfig` (line 1126): `RequiredFiles`, `OptionalFiles`, `Lens`, `RemoteConfig`
- `LensConfig` (line 1116): `Name`, `Config` (JSON)
- Regex compilation at config load: lines 2624-2637

**Similar Functionality**:
- HTML lens (`pkg/spyglass/lenses/html/html.go`): closest existing lens to what's needed — renders file content as HTML in iframe. OpenShift already uses this to surface HTML debugging artifacts.
- Buildlog lens: demonstrates reading text artifacts, line processing, error highlighting

**Markdown Infrastructure**:
- `pkg/markdown/code_block.go` — only a `DropCodeBlock()` utility, NOT a markdown renderer
- No existing markdown-to-HTML rendering in the codebase

### Test Coverage

**Existing Tests**:
- `pkg/spyglass/lenses/html/html_test.go` — tests HTML lens rendering
- `pkg/spyglass/lenses/buildlog/lens_test.go` — tests buildlog rendering
- `pkg/spyglass/spyglass_test.go` — tests artifact listing and lens ordering
- `cmd/deck/main_test.go` — tests deck handlers including Spyglass integration
- Coverage assessment: Good for existing lenses, patterns well-established for new lens tests

### Root Cause Analysis

**Primary Cause**:
This is a feature gap, not a bug. Spyglass has a powerful artifact viewing system but no built-in way to surface debugging guidance. The artifacts link is one of many on the page and doesn't stand out to new contributors. Job-specific debugging information (e.g., "check apiserver logs for test failures") has no standard mechanism to be surfaced.

**Contributing Factors**:
1. The Spyglass UI treats all artifacts equally — no mechanism to "promote" certain artifacts to prominence
2. No markdown rendering lens exists, so even if DEBUGGING.md were present, it wouldn't render nicely
3. The lens system requires explicit configuration per Prow deployment — there's no way for a job to declare "show this lens for my artifacts"

### Proposed Solutions

#### Approach 1: New Dedicated "Debugging" Markdown Lens

**Description**: Create a new Spyglass lens (`pkg/spyglass/lenses/debugging/`) that matches `DEBUGGING.md` artifacts, renders markdown to HTML server-side (using a Go markdown library like `goldmark`), and displays the result prominently with high priority.

**Pros**:
- Clean, purpose-built solution matching the issue request exactly
- High priority placement ensures visibility
- Could support collapsible display (semi-collapsed by default as requested)
- Markdown rendering gives rich formatting (links, headers, code blocks)
- Follows established lens patterns — straightforward implementation

**Cons**:
- Adds a new Go dependency (markdown library)
- Requires configuration in each Prow deployment's config
- Job tooling must explicitly produce DEBUGGING.md files

**Affected Components**:
- New: `pkg/spyglass/lenses/debugging/` (lens implementation + template)
- Modified: `cmd/deck/main.go` (add import)
- Config: Prow deployment configs need lens entry

**Complexity**: Low-Medium

**Backwards Compatibility**: Fully backwards compatible — lens only activates when DEBUGGING.md artifact exists and lens is configured

#### Approach 2: Generic Markdown Lens

**Description**: Instead of a debugging-specific lens, create a general-purpose markdown rendering lens that can be configured to match any `.md` artifact. The "debugging" use case becomes one configuration of this general lens.

**Pros**:
- More versatile — can render any markdown artifact (README.md, RESULTS.md, etc.)
- Single lens serves multiple use cases
- Prow deployments can configure it for whatever markdown files their jobs produce
- More likely to see adoption since it's generally useful

**Cons**:
- Less opinionated about UX (no built-in collapsible/prominent behavior for debugging)
- Configuration burden shifts to deployment operators
- May need more features to handle various markdown use cases

**Affected Components**: Same as Approach 1, but with `lenses/markdown/` instead of `lenses/debugging/`

**Complexity**: Low-Medium

**Backwards Compatibility**: Fully backwards compatible

#### Approach 3: Leverage Existing HTML Lens

**Description**: Rather than creating a new lens, recommend that job tooling emit `DEBUGGING.html` instead of `DEBUGGING.md`. The existing HTML lens already renders HTML artifacts. This requires no Prow code changes — just documentation and job tooling updates.

**Pros**:
- Zero Prow code changes needed
- Already works (OpenShift uses this pattern today, as noted by petr-muller)
- Proven in production
- Fastest path to value

**Cons**:
- Requires job tools to emit HTML instead of simpler markdown
- HTML lens renders with HideTitle=true and low priority (3) — may not be prominent enough
- No collapsible UX out of the box
- Puts burden on job maintainers to produce well-formatted HTML
- Doesn't address the discoverability problem (artifacts link still not prominent)

**Affected Components**: None in Prow — only job tooling and documentation

**Complexity**: None (Prow side)

**Backwards Compatibility**: N/A

#### Recommendation

**Preferred Approach**: Approach 2 (Generic Markdown Lens)

A general-purpose markdown lens provides the most value. It directly enables the DEBUGGING.md use case while also being useful for other markdown artifacts. The implementation is straightforward following established lens patterns. The key advantage over the HTML lens (Approach 3) is that markdown is much simpler for job maintainers to produce, directly addressing pohly's concern about maintainer burden.

**Key Implementation Considerations**:
1. Choose a well-maintained Go markdown library (goldmark is the standard choice)
2. Support configurable priority so deployments can make it prominent
3. Consider optional collapsible/expandable display mode
4. Sanitize rendered HTML to prevent XSS from artifact content
5. Support relative links within artifacts (as the issue mentions)

**Testing Requirements**:
- Unit tests for markdown rendering with various inputs
- Tests for artifact matching and configuration
- Tests for HTML sanitization
- Integration test with Spyglass rendering pipeline

## Next Steps

(Action items will be added here)
