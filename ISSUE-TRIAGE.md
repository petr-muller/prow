# Triage for Issue #606

**Status**: In Progress
**Created**: 2026-04-29

## Issue Information

- **Issue Number**: #606
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/606
- **Title**: DCO plugin trusted_apps config not working
- **Author**: clubanderson
- **Created**: 2026-01-27
- **Labels**: kind/bug, lifecycle/stale

## Issue Summary

The `trusted_apps` configuration option for the DCO plugin doesn't work. Despite correct configuration (`trusted_apps: [Copilot]`), the DCO plugin still fails commits from GitHub Copilot (`app/copilot-swe-agent`). The reporter has verified the config is loaded in the plugins configmap and that the commit author login matches `Copilot`.

## Findings

(Findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)
