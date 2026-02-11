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

## Findings

(Findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)
