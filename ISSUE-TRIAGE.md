# Triage for Issue #502

**Status**: In Progress
**Created**: 2026-01-31

## Issue Information

- **Issue Number**: #502
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/502

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

#### Analysis

This issue is a well-documented feature request to remove the mutual exclusivity constraint between `run_if_changed` and `skip_if_only_changed` in Prow job configuration.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Job triggering configuration validation
- Exists in this repo: Yes
- Relevant code paths: `pkg/config/config.go` (validateTriggering function)

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue provides:
  - Current implementation reference (commit 419f5e43)
  - Code snippet showing the constraint
  - Use case motivation
  - Proposed implementation approach
  - Backward compatibility considerations

#### Maintainer Feedback Already Present

Two maintainers have weighed in negatively on this proposal:

1. **petr-muller**: 👎 - The example scenario doesn't demonstrate a legitimate need (the `run_if_changed` alone would already skip docs-only changes). Concerned about footgun potential from misconfiguration between the two fields.

2. **BenTheElder**: Worries about making this feature "even more of a footgun and difficult to reason about." Notes that `skip_if_only_changed` is generally the safer approach.

**Author's Clarification**:
The author (kaovilai) provided a more concrete use case: needing to match `.yaml` files across ~10 directories but exclude the `docs/` directory. Since Go's RE2 regexp doesn't support negative lookahead, the workaround regex becomes complex and unmaintainable.

#### Recommendation

**Keep open** - This is a legitimate feature request for Prow configuration. While maintainers have expressed concerns, the discussion is ongoing and the author has provided additional real-world use cases. The issue should remain open for:
1. Further community input
2. Potential reconsideration if more compelling use cases emerge
3. Possible alternative solutions

**Suggested Action**: Continue triage to fully understand the technical implications, then document the maintainer decision rationale for future reference.

## Next Steps

(Action items will be added here)
