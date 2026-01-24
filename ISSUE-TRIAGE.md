# Triage for Issue #376

**Status**: In Progress
**Created**: 2026-01-24

## Issue Information

- **Issue Number**: #376
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/376

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Bug / Missing Feature

**Issue Summary**:
The issue reporter (loispostula) cannot run Prow Deck at a subpath (e.g., `https://infra.example.com/prow`) instead of at the root path. While the main Deck page loads when accessed via a subpath, static assets and API endpoints (like `prowjobs.js`) fail because they're requested from the root domain rather than the configured subpath. The reporter has reviewed the code and cannot find a way to configure a path prefix.

**Repository Scope Check**:
- Component mentioned: Prow Deck
- Exists in this repo: Yes - Deck is a core Prow component at `cmd/deck/`
- Relevant code paths: `cmd/deck/`, deck templates, static asset serving

**Information Completeness**:
- Sufficient detail provided: Yes
- Issue includes:
  - Clear description of the problem
  - Concrete example with Kubernetes ingress configuration
  - Expected vs actual behavior
  - Note that they've searched the code for a solution
- Missing information: None - the issue is well-described

**Current Status**:
- Issue is already labeled as `kind/bug` and `area/deck`
- A contributor (tsj-30) has assigned themselves and proposed a solution approach
- Maintainer (petr-muller) has approved the approach and suggested breaking it into smaller PRs
- Active work is in progress as of November 2025

**Analysis**:
This is a legitimate architectural limitation in Prow Deck. The component assumes it runs at the root path (/) and doesn't support running under a subpath. This affects users who need to deploy Deck behind a reverse proxy or ingress controller on a subpath, which is a common deployment pattern in Kubernetes environments where multiple services share a single domain.

The issue represents a real gap in Deck's deployment flexibility. The proposed solution involves:
1. Adding BasePath awareness to templates
2. Extending URL routing logic (Simplify function)
3. Updating static resource and API endpoint references

This is not a misconfiguration or user error - it's a missing feature/capability in Deck itself.

### Recommendation

**Suggested Action**: Keep open and continue triage

**Rationale**:
- This is a legitimate bug/missing feature in Prow Deck
- The issue is well-documented with clear reproduction details
- It affects a valid use case (running behind a reverse proxy on a subpath)
- Work is already in progress by an active contributor
- Proper labels are already applied (kind/bug, area/deck)

**Next Steps**:
1. Proceed with research subcommand to understand the code architecture and implementation details
2. Assess effort level once solution approach is fully understood
3. Possibly augment the issue with additional technical context from code exploration

## Next Steps

1. Run research subcommand to explore Deck's routing, template rendering, and static asset serving
2. Run assess-effort subcommand to evaluate implementation complexity
3. Run augment subcommand to enhance issue with technical details
4. Run brief subcommand for final review
5. Run wrapup subcommand to finalize triage
