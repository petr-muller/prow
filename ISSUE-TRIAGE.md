# Triage for Issue #666

**Status**: In Progress
**Created**: 2026-03-29

## Issue Information

- **Issue Number**: #666
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/666
- **Title**: Allow use of Git subtrees with `mergecommitblocker`
- **Author**: RadaBDimitrova (Rada Dimitrova)
- **Labels**: area/plugins, kind/feature

## Issue Summary

The `mergecommitblocker` plugin blocks all merge commits unconditionally, which prevents Git subtree workflows from functioning. Git subtrees inherently depend on merge commits (created by `git subtree pull`) to track history correctly. The author requests flexibility to exempt certain paths/directories from the merge commit check.

## Findings

(Findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)
