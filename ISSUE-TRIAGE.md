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

## Next Steps

- Assess effort for remaining work
- Prepare augmentation comment
