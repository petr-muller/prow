# Triage for Issue #438

**Status**: In Progress
**Created**: 2026-02-21

## Issue Information

- **Issue Number**: #438
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/438

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests adding regex support for branch matching in Tide's `includedBranches` and `excludedBranches` fields. Currently these fields only support exact string matching, while the branchprotector component already supports regex patterns for branch names. The author argues this inconsistency forces users to list each branch explicitly, even when branches follow a pattern (e.g., `release-*`, `feature-*`).

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Tide status controller (`pkg/tide/status.go`)
- Exists in this repo: Yes
- Relevant code paths: `pkg/tide/status.go`, `pkg/tide/status_test.go`, `pkg/config/tide.go`, `pkg/config/tide_test.go`
- Reference component: `cmd/branchprotector/protect.go` (existing regex support)

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue clearly describes the current behavior, desired behavior, and references existing regex support in branchprotector as precedent

### Recommendation

This is a valid feature request for a Prow component that lives in this repository. The request is well-reasoned: it addresses an inconsistency between two Prow components and has a clear precedent in the branchprotector implementation. The author (@kaovilai) has actively maintained the issue by removing stale labels twice, indicating continued interest. A maintainer (petr-muller) has already labeled it with `area/tide` and `kind/feature` and noted that #482 may be related.

**Suggested Action**:
- Keep open and continue triage

## Findings

(Additional findings from triage subcommands will be added here)

## Next Steps

- Research: Investigate how branch matching currently works in Tide vs branchprotector
- Assess effort required to add regex support
- Augment the issue with technical findings
