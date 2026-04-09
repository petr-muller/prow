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

## Next Steps

- Assess effort
- Augment the issue with findings
