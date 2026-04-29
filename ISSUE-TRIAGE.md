# Triage for Issue #693

**Status**: In Progress
**Created**: 2026-04-29

## Issue Information

- **Issue Number**: #693
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/693

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

This is a feature request to extend Prow's `/retest` command to trigger Netlify preview rebuilds for repositories that use Netlify (specifically k/website and k/contributor-site). The issue was originally filed as kubernetes/test-infra#35103 by Tim Bannister (lmktfy, SIG Docs chair) and transferred to the Prow repo by Caesarsage because `/retest` is a Prow plugin.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: `/retest` command in the trigger plugin
- Exists in this repo: Yes — `pkg/plugins/trigger/generic-comment.go`, `pkg/pjutil/filter.go`
- Relevant code paths: `pkg/plugins/trigger/`, `pkg/pjutil/filter.go` (RetestRe, RetestFilter)
- No existing Netlify integration exists in the codebase

**Information Completeness**:
- Sufficient detail provided: Partially
- Missing information:
  - No concrete Netlify API endpoints identified for triggering rebuilds
  - No design proposal for how credentials would be managed
  - BenTheElder raised security concerns about handing Netlify creds to presubmits (arbitrary user code)
  - lmktfy suggested only a "trigger rebuilds" token would be needed

### Key Context from Original Issue (test-infra#35103)

- BenTheElder (Prow maintainer) raised concerns about security: handing Netlify credentials to presubmits that may run arbitrary user code
- BenTheElder noted Netlify exposes deploy triggers but "that's not quite it" — the exact API mechanism is unclear
- lmktfy believes only a rebuild-trigger token would be needed
- The issue has been bouncing between stale/rotten lifecycle stages since July 2025
- Caesarsage self-assigned and opened this tracking issue on 2026-04-28

### Recommendation

Keep open and continue triage. This is a legitimate feature request for the Prow trigger plugin. However, it has significant open questions around:

1. **Feasibility**: Does Netlify's API support triggering deploy preview rebuilds for a specific PR? (Deploy triggers are for production deploys, not PR previews)
2. **Security**: How to safely provide Netlify credentials without exposing them to arbitrary presubmit code
3. **Architecture**: Whether this belongs as an extension to the trigger plugin, a new plugin, or an external webhook

**Suggested Action**:
- Keep open and continue triage
- Research feasibility of Netlify API for PR preview rebuilds
- Assess effort and architectural approach

## Findings

(Additional findings from triage subcommands will be added here)

## Next Steps

- Research: Investigate Netlify API capabilities for deploy preview rebuilds
- Assess effort and determine if this is feasible within Prow's architecture
- Augment the issue with technical findings
