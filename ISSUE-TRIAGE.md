# Triage for Issue #676

**Status**: In Progress
**Created**: 2026-04-09

## Issue Information

- **Issue Number**: #676
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/676

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests a new Prow plugin to validate Go dependency licenses against a configured allowlist when `go.mod` or `go.tool.mod` files change. The author provides a real-world example where an incompatible-license dependency was merged undetected into kubernetes-sigs/external-dns.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Prow plugin system (plugins under `pkg/plugins/`)
- Exists in this repo: Yes - Prow has a well-established plugin architecture
- Relevant code paths: `pkg/plugins/`, `pkg/hook/plugin-imports/`, existing plugins like `verify-owners`
- Note: The existing "license check" referenced by the author (licensecheck presubmit) is actually a boilerplate/header check, not a dependency license check. These are fundamentally different concerns.

**Information Completeness**:
- Sufficient detail provided: Yes
- The author provides: problem description, real-world example, proposed solution, alternatives evaluated (go-licenses, skywalking-eyes), and reference to existing Kubernetes hack script
- Missing: specific allowlist format proposal, but that's a design detail

### Context from Comments

A Prow maintainer (stmcginnis) responded suggesting this might be better as a GitHub Action rather than a Prow plugin, noting that Prow's existing license work is just source header checks. The author acknowledged the uncertainty about the right location but noted concerns about reducing third-party dependencies due to recent supply chain compromises.

### Recommendation

This is a legitimate feature request. The issue correctly identifies a gap: there is no automated dependency license compliance check in Prow's plugin ecosystem. While there's a valid architectural question about whether this belongs as a Prow plugin vs. external tooling (raised by the maintainer), that's a design discussion, not a reason to reject the issue.

**Suggested Action**:
- Keep open and continue triage
- The architectural question (plugin vs. external action) should be part of the research phase

## Code Research

### Current Implementation

There is no existing dependency license checking functionality in Prow. The "license check" referenced in the issue (licensecheck presubmit visible at prow.k8s.io) is actually a source file boilerplate/header check (`hack/boilerplate/verify_boilerplate.py`), which is a standalone verification script, not a plugin.

**Primary Components (Plugin System)**:
- Plugin interface: `pkg/plugins/plugins.go` - Defines handler types (PullRequestHandler, GenericCommentHandler, etc.) and the Agent struct providing GitHub/Git/Config access
- Plugin registration: `cmd/hook/plugin-imports/plugin-imports.go` and `pkg/hook/plugin-imports/plugin-imports.go` - Blank imports to register all built-in plugins
- Plugin config: `pkg/plugins/config.go` - Top-level Configuration struct with per-plugin config sections
- Event dispatch: `pkg/hook/events.go` - Demuxes webhook events to registered handlers

**Analogous Existing Plugins**:
- `verify-owners` (`pkg/plugins/verify-owners/verify-owners.go`): File validation plugin - triggers on PR events, gets changed files via `GetPullRequestChanges()`, clones repo, parses/validates file contents, posts review comments and manages labels
- `buildifier` (`pkg/plugins/buildifier/buildifier.go`): Runs external tool (buildifier) on changed files programmatically via Go library, posts review with suggestions
- `golint` (`pkg/plugins/golint/golint.go`): Runs lint on .go files using Go library, reports issues at specific file positions
- `dco` (`pkg/plugins/dco/dco.go`): Content validation without external tools, manages labels and comments

**External Plugin Pattern**:
- External plugins are standalone HTTP servers (e.g., `cmd/external-plugins/needs-rebase/`)
- Hook server forwards webhook events as HTTP POST requests
- Suited for slow operations, privileged/isolated execution, or integrating external services
- Documented in `site/content/en/docs/components/plugins/_index.md`

### Architecture Overview

Prow's plugin system supports three approaches for this kind of check:

1. **Internal plugin** (compiled into hook binary): Fast, simple deployment, runs in hook process
2. **External plugin** (standalone HTTP server): Isolated, independently deployable, suited for slow/privileged operations
3. **Standalone verification script** (like boilerplate check): Runs in CI pipeline, not event-driven

### Root Cause Analysis

**Primary Gap**: No mechanism exists in Prow to validate dependency licenses. The boilerplate check is sometimes confused with license compliance checking, but they serve fundamentally different purposes:
- Boilerplate check: Verifies source file headers contain correct copyright/license text
- Dependency license check: Verifies that imported Go modules use compatible licenses (SPDX analysis)

**Contributing Factors**:
1. The Kubernetes ecosystem uses a separate hack script (`hack/verify-licenses.sh`) for this, which is not portable
2. Third-party tools (go-licenses, skywalking-eyes) exist but add external dependencies
3. No standard Prow plugin pattern for dependency-level (vs file-level) validation

### Proposed Solutions

#### Approach 1: Internal Prow Plugin

**Description**: New plugin in `pkg/plugins/license-deps/` that triggers on PR events when `go.mod` or `go.tool.mod` files change. Parses module dependencies, resolves their licenses (via module cache or API), and validates against a configured allowlist.

**Pros**:
- Consistent with existing Prow plugin patterns (verify-owners, golint)
- Single deployment unit (compiled into hook)
- Configuration lives in `plugins.yaml` alongside other plugin config
- Native access to PR changes, labels, comments

**Cons**:
- License resolution is potentially slow (fetching module metadata)
- Adds complexity to the hook binary
- License detection libraries (go-licenses, etc.) are substantial dependencies
- Hook process handles all webhooks; a slow plugin impacts everything

**Affected Components**:
- New: `pkg/plugins/license-deps/`
- Modified: `pkg/plugins/config.go` (add config struct), plugin-imports files
- Config: New section in `plugins.yaml`

**Complexity**: High - License resolution is the hard part, not the plugin plumbing

**Backwards Compatibility**: No impact (new plugin, opt-in)

#### Approach 2: External Prow Plugin

**Description**: Standalone external plugin server that receives PR webhook events, checks dependency licenses independently, and reports back via GitHub API.

**Pros**:
- Isolated from hook process - slow license resolution doesn't block other plugins
- Can be deployed/scaled independently
- Official docs recommend external plugins for slow operations
- Can be developed and released independently of Prow core

**Cons**:
- More operational complexity (separate deployment, networking)
- Requires its own configuration management
- Less discoverable than built-in plugins

**Complexity**: High (same license resolution challenge, plus deployment infrastructure)

**Backwards Compatibility**: No impact

#### Approach 3: Not a Prow Plugin (CI Script / GitHub Action)

**Description**: Provide a standalone verification script (similar to boilerplate check) or recommend existing tools (go-licenses, skywalking-eyes) to be integrated into CI pipelines independently.

**Pros**:
- Simplest approach for Prow maintainers (no code change needed)
- Existing tools already do this (go-licenses, skywalking-eyes)
- Each project can customize their allowlist independently
- No coupling to Prow release cycle

**Cons**:
- Not integrated with Prow's label/comment workflow
- Each project reinvents the wheel
- The author specifically wants to reduce third-party tool usage (supply chain concern)
- Doesn't address the core request

**Complexity**: Low for Prow, but pushes complexity to consumers

#### Recommendation

**Preferred Approach**: Approach 3 (Not a Prow plugin) is the most architecturally sound, aligning with maintainer stmcginnis's feedback. The core challenge here is license resolution/detection, which is a complex domain unto itself. Prow's strength is webhook event handling and GitHub workflow automation, not license compliance analysis. The right boundary is:

- License detection tools (go-licenses, skywalking-eyes, etc.) handle the analysis
- CI pipelines (ProwJobs, GitHub Actions) orchestrate when the check runs
- Prow's role is running the job, not implementing license analysis

If the community strongly wants Prow integration, Approach 2 (external plugin) is preferred over Approach 1, since license resolution is inherently slow and should be isolated from the hook process.

**Key Implementation Considerations**:
1. The hard problem is license resolution, not Prow integration
2. Existing tools (go-licenses, skywalking-eyes) already solve the hard problem
3. A ProwJob running a license check script is the simplest viable integration
4. An external plugin would be the next step if tighter GitHub integration is needed

**Testing Requirements**:
- If implemented as a plugin: unit tests for allowlist parsing, integration tests for license resolution, e2e tests with mock GitHub API
- If implemented as a script: standard script testing patterns

## Effort Assessment

**Effort Level**: 3 - Large (requires expertise)

### Summary

While the Prow plugin plumbing itself is well-understood, the core challenge is license resolution — a complex domain requiring SPDX analysis, module metadata fetching, and transitive dependency handling. The architectural question (plugin vs. external tool vs. CI script) adds design uncertainty. This is not a good-first-issue regardless of approach chosen.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Large (if plugin) / Small (if CI script recommendation)
- **Details**: A new plugin would require 5+ new files (plugin code, config, tests, docs, plugin-imports registration). A CI script approach requires no Prow code changes but needs documentation.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: High
- **Details**: The fundamental challenge is license resolution: fetching module metadata, detecting license files in module sources, classifying licenses against SPDX identifiers, handling transitive dependencies, edge cases (vendored deps, replaced modules, multi-license packages). This is why go-licenses and skywalking-eyes are substantial projects themselves.
- **Level Indication**: 3-4

#### Required Expertise
- **Assessment**: Deep
- **Details**: Requires expertise in Go module system internals, license classification (SPDX), Prow plugin architecture, and understanding of supply chain compliance requirements. Domain expertise in software licensing is non-trivial.
- **Level Indication**: 3-4

#### Clarity and Certainty
- **Assessment**: Significant uncertainty
- **Details**: The fundamental design question (should this be a Prow plugin at all?) is unresolved. A maintainer has already pushed back on the plugin approach. The issue also doesn't specify: allowlist format, handling of transitive deps, what constitutes "incompatible", how to handle license detection failures.
- **Level Indication**: 3-4

#### Testing Requirements
- **Assessment**: Complex
- **Details**: License resolution testing requires mock module proxies, test Go modules with various license types, SPDX classification validation, and edge case coverage (no license file, dual-licensed, vendor directory). Integration tests with GitHub API needed if plugin approach.
- **Level Indication**: 3-4

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: New feature, entirely opt-in regardless of approach. No impact on existing deployments.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Questionable fit
- **Details**: Prow plugins handle GitHub webhook events and perform GitHub API operations. License compliance analysis is a domain-specific static analysis task that doesn't naturally fit the webhook-driven model. Existing analogues (boilerplate check, verify-licenses.sh in Kubernetes) are standalone scripts, not plugins. A maintainer has already noted this may not be the right location.
- **Level Indication**: 3-4

#### External Dependencies
- **Assessment**: Heavy external dependency
- **Details**: License resolution inherently depends on fetching module metadata from Go module proxy, reading license files from module sources, and potentially querying license databases. The author noted that existing tools (go-licenses) are effectively unmaintained. Building this from scratch is substantial; depending on external tools raises supply chain concerns the author wants to avoid.
- **Level Indication**: 3-4

### Recommended Labels

- [x] `kind/feature`: New capability request
- [x] `area/plugins`: Relates to plugin system
- [ ] `good-first-issue`: Far too complex and uncertain
- [ ] `help-wanted`: Design questions need resolution before implementation can begin

### Guidance for Contributors

**For Level 3 (Large)**:
- Requires design discussion with maintainers before any implementation
- Key unresolved questions:
  1. Should this be a Prow plugin, external plugin, or CI script pattern?
  2. Which license detection library/approach to use?
  3. How to handle transitive dependency licenses?
  4. What allowlist/denylist format to standardize on?
- Should review existing approaches: Kubernetes `hack/verify-licenses.sh`, go-licenses, skywalking-eyes
- Consider starting with a KEP/design doc to get maintainer alignment
- Consult with Prow maintainers (stmcginnis has already weighed in)

### Caveats and Considerations

The effort level depends heavily on which approach is chosen:
- **If "recommend CI script" (Approach 3)**: The Prow-side effort is Level 1 (documentation only), but the overall ecosystem effort remains Level 3
- **If internal plugin (Approach 1)**: Level 3-4, substantial new code with license detection complexity
- **If external plugin (Approach 2)**: Level 3, similar complexity but better architectural isolation

The maintainer feedback suggests the community may lean toward Approach 3, which would reduce this to a documentation/guidance issue rather than a code change.

## Next Steps

- Augment the issue with findings
