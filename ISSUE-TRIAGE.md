# Triage for Issue #617

**Status**: In Progress
**Created**: 2026-02-11

## Issue Information

- **Issue Number**: #617
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/617
- **Title**: Add a plugin to block merging PRs with `fixup!` commits
- **Author**: nojnhuh
- **Labels**: area/plugins, kind/feature
- **State**: OPEN

## Issue Summary

The author uses `git commit --fixup` during iterative review, then `git rebase --autosquash` before merging. They sometimes forget to `/hold` PRs and merge with `fixup!` commits still present. They request a plugin similar to `mergecommitblocker` that would:
- Detect commits whose messages start with `fixup!` or `amend!`
- Automatically add a `do-not-merge/*` label
- Remove the label when no such commits exist (e.g., after `git rebase --autosquash`)

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests a feature to automatically block merging of PRs that contain `fixup!` or `amend!` commits. This is a well-defined, practical feature request for the Prow plugin ecosystem.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Prow plugins (similar to `mergecommitblocker`)
- Exists in this repo: Yes — both `invalidcommitmsg` (`pkg/plugins/invalidcommitmsg/`) and `mergecommitblocker` (`pkg/plugins/mergecommitblocker/`) are plugins in this repository
- Relevant code paths: `pkg/plugins/invalidcommitmsg/`, `pkg/plugins/mergecommitblocker/`

**Information Completeness**:
- Sufficient detail provided: Yes
- Use case clearly described: Author uses `git commit --fixup` workflow and sometimes forgets to squash before merge
- Desired behavior well-defined: Detect `fixup!`/`amend!` prefixes, add `do-not-merge/*` label, remove label when commits are cleaned up
- Reference to existing similar plugin (`mergecommitblocker`) as model

**Maintainer Input**: A maintainer (petr-muller) has already commented suggesting this could be added to the existing `invalidcommitmsg` plugin rather than creating a new one.

### Recommendation

This is a legitimate feature request. The use case is common in iterative code review workflows. The requested functionality fits naturally within Prow's plugin architecture, and there's already a closely related plugin (`invalidcommitmsg`) that handles similar commit message validation with the same `do-not-merge/invalid-commit-message` label.

**Suggested Action**:
- Keep open and continue triage
- Investigate whether extending `invalidcommitmsg` (as the maintainer suggested) is the right approach

## Findings

(Further findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)
