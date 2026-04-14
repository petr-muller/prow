# Triage for Issue #190

**Status**: In Progress
**Created**: 2026-04-14

## Issue Information

- **Issue Number**: #190
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/190

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that when a presubmit job was renamed (`pull-etcd-unit-test` to `pull-etcd-unit-test-amd64`), the status reconciler triggered the newly-named job on many PRs — including **draft pull requests**. Draft PRs should not have had jobs triggered on them.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: status-reconciler
- Exists in this repo: Yes (`pkg/statusreconciler/`, `cmd/status-reconciler/`)
- Relevant code paths: `pkg/statusreconciler/controller.go`, `pkg/statusreconciler/status.go`

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue includes: a clear description, the specific rename that triggered the problem, an example draft PR that was incorrectly triggered, and a link to the job history showing the spurious runs
- Missing information: None significant — the report is well-written

### Recommendation

This is a valid bug report. The status reconciler is a component in this repository, and the described behavior (triggering jobs on draft PRs during reconciliation after a job rename) is clearly unintended. Draft PRs are explicitly excluded from presubmit triggering in normal Prow operation, so the status reconciler should respect the same exclusion.

**Suggested Action**:
- Keep open and continue triage

## Findings

(Additional findings from triage subcommands will be added here)

## Next Steps

- Proceed with `research` subcommand to investigate the status reconciler code
- Determine if draft PR filtering exists and where it's missing
