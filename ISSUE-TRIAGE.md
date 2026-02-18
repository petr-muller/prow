# Triage for Issue #177

**Status**: In Progress
**Created**: 2026-02-18

## Issue Information

- **Issue Number**: #177
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/177

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that the "Details" link on the Tide status check for GitHub PRs does not work correctly when the PR is authored by a bot user (e.g., `dependabot`). The Tide status URL includes `author:dependabot` in the query, but GitHub's search requires `author:app/dependabot` for GitHub App bot users. This causes the link to return no results.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide (status check "Details" link)
- Exists in this repo: Yes
- Relevant code paths:
  - `pkg/tide/status.go` — `targetURL()` function (lines 379-405) constructs the PR query URL using `crc.AuthorLogin`
  - `pkg/tide/status_test.go` — test coverage for `targetURL()`
  - `pkg/config/tide.go` — configuration for `PRStatusBaseURL`

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue includes:
  - A concrete example PR (dependabot PR in cluster-api-provider-azure)
  - The broken URL with the incorrect `author:dependabot` query
  - The expected working URL with `author:app/dependabot` query
  - A screenshot showing the "Details" link

### Recommendation

This is a valid bug report for the Tide component. The `targetURL()` function in `pkg/tide/status.go` constructs a query using `crc.AuthorLogin` directly, but for GitHub App bot users, the login needs to be prefixed with `app/` for the GitHub search query to match correctly.

The issue was originally filed in kubernetes/test-infra and correctly migrated to this repository. It has existing labels `kind/bug` and `help wanted`, and was confirmed still active by the author in August 2024.

**Suggested Action**:
- Keep open and continue triage

## Findings

(Further findings from triage subcommands will be added here)

## Next Steps

- Proceed with research subcommand to investigate root cause and solution approaches
