# Triage for Issue #676

**Status**: In Progress
**Created**: 2026-04-09

## Issue Information

- **Issue Number**: #676
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/676

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests a new Prow plugin to validate Go dependency licenses against a configured allowlist when `go.mod` or `go.tool.mod` files change. The author provides a real-world example where an incompatible-license dependency was merged undetected into kubernetes-sigs/external-dns.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Prow plugin system (plugins under `pkg/plugins/`)
- Exists in this repo: Yes - Prow has a well-established plugin architecture
- Relevant code paths: `pkg/plugins/`, `pkg/hook/plugin-imports/`, existing plugins like `verify-owners`
- Note: The existing "license check" referenced by the author (licensecheck presubmit) is actually a boilerplate/header check, not a dependency license check. These are fundamentally different concerns.

**Information Completeness**:
- Sufficient detail provided: Yes
- The author provides: problem description, real-world example, proposed solution, alternatives evaluated (go-licenses, skywalking-eyes), and reference to existing Kubernetes hack script
- Missing: specific allowlist format proposal, but that's a design detail

### Context from Comments

A Prow maintainer (stmcginnis) responded suggesting this might be better as a GitHub Action rather than a Prow plugin, noting that Prow's existing license work is just source header checks. The author acknowledged the uncertainty about the right location but noted concerns about reducing third-party dependencies due to recent supply chain compromises.

### Recommendation

This is a legitimate feature request. The issue correctly identifies a gap: there is no automated dependency license compliance check in Prow's plugin ecosystem. While there's a valid architectural question about whether this belongs as a Prow plugin vs. external tooling (raised by the maintainer), that's a design discussion, not a reason to reject the issue.

**Suggested Action**:
- Keep open and continue triage
- The architectural question (plugin vs. external action) should be part of the research phase

## Findings

(Additional findings from triage subcommands will be added below)

## Next Steps

- Research: Investigate plugin architecture feasibility and the plugin-vs-action tradeoff
- Assess effort
- Augment the issue with findings
