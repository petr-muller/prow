# Triage for Issue #391

**Status**: In Progress
**Created**: 2026-04-04

## Issue Information

- **Issue Number**: #391
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/391

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests a new configuration option for the `assign` plugin that would restrict `/assign` commands to org members only. The motivation is clear: during GSoC periods, non-org participants do "drive-by assigns" on good-first-issues, claiming them without the intent or ability to work on them.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: `assign` plugin (`pkg/plugins/assign/assign.go`)
- Exists in this repo: Yes
- Relevant code paths:
  - `pkg/plugins/assign/assign.go` — the assign handler, specifically the `add` logic around line 148
  - `pkg/plugins/config.go` — plugin configuration definitions

**Information Completeness**:
- Sufficient detail provided: Yes
- The author (Daniel Hiller, KubeVirt contributor) provides:
  - Clear use case with real-world example
  - Specific proposed solution (`onlyOrgMembers` bool config per repo)
  - Links to relevant code locations
  - Context from maintainer discussion (petr-muller) about default behavior

### Recommendation

Keep open and continue triage. This is a well-defined, legitimate feature request for the assign plugin. The issue author is a known contributor and the feature addresses a real operational pain point. A Prow maintainer (petr-muller) has already engaged with the issue, validating its relevance.

**Suggested Action**:
- Keep open and continue triage
- The issue has been reopened by the author after bot auto-close, indicating continued interest
- A maintainer has kept it alive by removing lifecycle/stale labels multiple times

## Findings

(Further findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)
