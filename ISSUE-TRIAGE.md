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

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that the `trusted_apps` configuration option for the DCO plugin is not functioning — commits from GitHub Copilot fail DCO checks despite being listed as a trusted app. This is a bug report for the DCO plugin code that lives in this repository.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: DCO plugin (`trusted_apps` feature)
- Exists in this repo: Yes
- Relevant code paths: `pkg/plugins/dco/dco.go`, `pkg/plugins/dco/dco_test.go`, `pkg/plugins/config.go`

**Information Completeness**:
- Sufficient detail provided: Yes
- The reporter provides: Prow version, exact config YAML, expected vs actual behavior, reproduction steps, environment verification (configmap check, API verification of author login, hook logs), and a workaround
- No information is missing — this is a well-written bug report

### Recommendation

Keep open and continue triage. This is a valid bug report for a specific feature (`trusted_apps`) of a plugin (`dco`) that lives in this repository. The reporter has done thorough investigation including verifying config loading and commit author identity. The `lifecycle/stale` label is from automated bot activity, not from maintainer assessment.

**Suggested Action**:
- Keep open and continue triage
- Remove `lifecycle/stale` label after triage is complete

## Findings

(Further findings from triage subcommands will be added here)

## Next Steps

(Action items will be added here)
