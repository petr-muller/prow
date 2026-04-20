# Triage for Issue #527

**Status**: In Progress
**Created**: 2026-04-20

## Issue Information

- **Issue Number**: #527
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/527

## Initial Validation

**Assessment**: LEGITIMATE (but largely addressed)

### Analysis

The issue requests adding documentation for running Prow outside GKE, specifically on local/Kind environments, to lower the barrier for contributors. Filed 2025-10-13 by @tsj-30.

**Issue Category**: Feature Request (documentation improvement)

**Repository Scope Check**:
- Component mentioned: Documentation (`site/content/en/docs/getting-started-deploy.md`)
- Exists in this repo: Yes
- Relevant code paths:
  - `site/content/en/docs/getting-started-deploy.md` — the GKE-centric deploy guide
  - `site/content/en/docs/local-dev.md` — **new** local dev guide (added 2026-03-26)
  - `site/content/en/docs/local-dev-tilt.md` — Tilt-based dev workflow (added 2026-03-26)
  - `hack/dev-env.sh` — script powering `make dev`

**Information Completeness**:
- Sufficient detail provided: Yes
- Missing information: N/A

### Key Discovery: Issue Largely Addressed

Since this issue was filed, significant work has been done that addresses most of the request:

1. **`local-dev.md`** (added 2026-03-26): A comprehensive guide for running a complete Prow stack locally using Kind, with fake replacements for all external services. Covers prerequisites, quick start (`make dev`), component profiles, rebuilding single components, sending fake webhooks, running integration tests, and developing hook plugins.

2. **`local-dev-tilt.md`** (added 2026-03-26): An additional guide for automatic rebuild/redeploy using Tilt on top of the Kind-based environment.

3. **Issue #283** (now closed): The starter config has been moved from test-infra to this repo (2025-11-13), which was identified as the main actionable item by maintainer @BenTheElder.

**What remains potentially unaddressed**:
- The `getting-started-deploy.md` guide itself may still be GKE-centric without cross-referencing the new local-dev docs
- @petr-muller's vision (2026-01-19) of contribution-path-oriented docs (hook plugin, tide change, frontend work, ProwJob controllers) is partially fulfilled by `local-dev.md` but could be expanded

### Recommendation

The issue is legitimate but has been **substantially addressed** by subsequent work. The remaining gap is small: ensuring the deploy guide cross-references the local dev guide, and potentially expanding contribution-path-specific documentation.

**Suggested Action**:
- Keep open for now — continue triage to assess remaining gaps
- May be closeable as mostly-addressed, with any remaining work tracked separately

## Code Research

### Current Documentation State

**Primary Documents**:
- `site/content/en/docs/getting-started-deploy.md` — Production deployment guide, heavily GKE-centric
- `site/content/en/docs/local-dev.md` (weight 75) — Comprehensive local dev with Kind + fakes
- `site/content/en/docs/local-dev-tilt.md` (weight 76) — Tilt-based auto-rebuild layer
- `site/content/en/docs/getting-started-develop.md` (weight 90) — Contributing guide

**Cross-reference Analysis**:
- `getting-started-develop.md` DOES cross-reference `local-dev.md` (good)
- `getting-started-deploy.md` does NOT mention `local-dev.md` (gap)
- `getting-started-deploy.md` opening says it's "focused on GKE but should work on any kubernetes distro" — no pointer to the simpler local path

**Navigation/Discoverability**:
- `local-dev.md` is well-positioned (weight 75, before deploy guide at 80)
- But developers arriving at the deploy guide (e.g., from search) have no signal that a lighter path exists

### Contribution Path Coverage

Per @petr-muller's vision of contribution-path-oriented docs:

| Contribution Path | Status | Location |
|---|---|---|
| Hook plugin development | **Covered** | `local-dev.md` (lines 170-181) + `getting-started-develop.md` |
| Tide changes | **Not covered** | Tide user docs exist but no development workflow |
| Deck/frontend work | **Partially covered** | `deck/_index.md` has `runlocal` but not full dev workflow |
| ProwJob controllers | **Not covered** | Component docs exist, no dev guide |
| Gerrit integration | **Not covered** | Full profile deploys it, no dev guide |

### Local Tooling Coverage

| Tool | Documented | Location |
|---|---|---|
| `hack/dev-env.sh` | Yes | `local-dev.md` |
| `hack/phony.sh` | Yes | `local-dev.md` |
| `hack/tilt-apply-config.sh` | No | — |
| `hack/tilt-build.sh` | No | — |
| Integration test scripts | Partially | `local-dev.md`, `build-test-update.md` |

### Root Cause Analysis

**Primary Cause**: The issue was filed when no local dev documentation existed. The core ask (local/Kind setup docs) has been addressed by `local-dev.md`.

**Remaining Gaps**:
1. **Cross-reference gap**: `getting-started-deploy.md` doesn't point developers to `local-dev.md` as the preferred starting point for exploration/development
2. **Contribution path gap**: Only hook plugin development has a dedicated workflow; other common paths (Tide, Deck, ProwJob controllers) lack development guides
3. **Scope shift**: The issue has evolved from "how to run Prow on Kind" (solved) to the broader vision of a "Prow development guide" (@BenTheElder, @petr-muller comments)

### Proposed Solutions

#### Approach 1: Close as Addressed + Open New Issue

**Description**: Close this issue as substantially addressed by the March 2026 local-dev work. Open a new, more targeted issue for the remaining gaps (deploy guide cross-reference, contribution path docs).

**Pros**:
- Acknowledges the substantial work done
- Creates a cleaner, more focused follow-up issue
- The original ask (Kind setup docs) is genuinely solved

**Cons**:
- Splits discussion history

**Complexity**: Low

#### Approach 2: Keep Open with Reduced Scope

**Description**: Add a comment noting what's been addressed, retitle to focus on remaining gaps (cross-reference + contribution paths), keep open as a tracking issue.

**Pros**:
- Preserves discussion context
- Acknowledges both progress and remaining work
- The broader vision from maintainer comments stays attached

**Cons**:
- Issue scope has shifted significantly from original

**Complexity**: Low

#### Recommendation

**Preferred Approach**: Approach 2 (Keep Open with Reduced Scope)

The issue has evolved through maintainer discussion into something broader than the original ask. The discussion thread itself contains valuable context about the intended direction. Adding a comprehensive comment that acknowledges the progress and reframes the remaining work is the cleanest path.

**Remaining actionable items**:
1. Add cross-reference from `getting-started-deploy.md` to `local-dev.md` (trivial PR)
2. Contribution-path development guides (larger effort, could be separate issues)

## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue) for remaining work

### Summary

The core request (local/Kind setup documentation) has already been implemented. The remaining work is a small cross-reference addition to `getting-started-deploy.md` — a trivial, well-defined documentation change. The broader contribution-path documentation vision is a separate, larger effort that should be tracked independently.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: 1 file (`getting-started-deploy.md`), ~5 lines to add a cross-reference note
- **Level Indication**: 1

#### Complexity
- **Assessment**: Simple
- **Details**: Adding a paragraph/note pointing to `local-dev.md` for developers who want to explore locally
- **Level Indication**: 1

#### Required Expertise
- **Assessment**: Minimal
- **Details**: Basic markdown, understanding of the doc structure
- **Level Indication**: 1

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: The gap is clear (missing cross-reference), the solution is obvious (add one)
- **Level Indication**: 1

#### Testing Requirements
- **Assessment**: Simple
- **Details**: Documentation only — verify Hugo renders correctly, no code tests needed
- **Level Indication**: 1

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Additive documentation change, no behavior impact
- **Level Indication**: 1

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Cross-referencing between related docs is standard practice
- **Level Indication**: 1

#### External Dependencies
- **Assessment**: None
- **Details**: Internal documentation only
- **Level Indication**: 1

### Recommended Labels

- [x] `good-first-issue`: Trivial cross-reference addition, perfect for newcomers
- [x] `area/documentation`: Already applied
- [x] `kind/cleanup`: Improving existing documentation
- [ ] `help-needed`: Too simple for this label

### Guidance for Contributors

This is a great first contribution:
1. Add a note near the top of `site/content/en/docs/getting-started-deploy.md` directing developers who want to explore/develop locally to the [Local Development Environment](/docs/local-dev/) guide
2. The note should clarify that `local-dev.md` is for development/exploration without cloud infrastructure, while the deploy guide is for production-like deployments
3. Submit a PR with the change

### Caveats and Considerations

The broader "Prow development guide" vision discussed by @BenTheElder and @petr-muller is a separate Level 2-3 effort that should be tracked in its own issue. That vision includes contribution-path-specific guides for Tide, Deck, ProwJob controllers, and Gerrit — each requiring domain expertise to write well.

## Next Steps

- Prepare augmentation comment
