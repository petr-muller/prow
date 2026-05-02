# Triage for Issue #51

**Status**: In Progress
**Created**: 2026-05-02

## Issue Information

- **Issue Number**: #51
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/51

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests the ability to run Prow in environments with OPA (Open Policy Agent) constraints — specifically preventing privileged container modes while preserving core functionality. This is a valid feature request for the Prow project.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: Prow core (all components that generate or run pods)
- Exists in this repo: Yes
- Relevant code paths:
  - `pkg/pod-utils/decorate/podspec.go` — pod decoration logic, SecurityContext handling
  - `pkg/apis/prowjobs/v1/types.go` — DecorationConfig (RunAsUser, RunAsGroup, FsGroup)
  - `cmd/checkconfig/testdata/` — test configs with privileged: true (Maistra builders)
  - `test/integration/config/nginx.yaml` — allowPrivilegeEscalation, NET_BIND_SERVICE cap
  - `test/integration/config/prow/deck.yaml` — hostPath mounts for Kind support

**Current State of OPA Compatibility**:
- Prow core Go code does NOT hardcode privileged mode or root user. The `decorateSpec()` function applies configurable RunAsUser/RunAsGroup/FsGroup from DecorationConfig, and only when PodSecurityContext isn't already set.
- `privileged: true` appears only in test/example ProwJob configs (Maistra builder), not in Prow's own deployment.
- `allowPrivilegeEscalation: true` appears only in the nginx integration test setup.
- No docker.sock mounts or PodSecurityPolicy references found.
- hostPath volumes exist only in test/integration presets (Kind support) and local dev mode.

**Information Completeness**:
- Sufficient detail provided: No — the issue is very brief
- Missing information:
  - Which specific OPA constraints are they hitting?
  - Which Prow components are violating policies?
  - What error messages or policy denials are they seeing?
  - What is their deployment method (Helm, raw manifests, etc.)?
  - Are they running Prow control plane, ProwJobs, or both?

### Recommendation

The issue is **legitimate** — making Prow compatible with restrictive policy environments (OPA, Kyverno, Pod Security Standards) is a reasonable feature request. However, the issue is extremely vague. It was filed in April 2023, has been closed by stale-bot twice, and reopened by the author twice (most recently January 2026), suggesting ongoing interest but no concrete progress.

The irony is that Prow's core code is already largely OPA-compatible: it doesn't hardcode privileged mode, supports configurable security contexts, and doesn't require root. The real question is what specific constraints the author is hitting — it could be about ProwJob pods (user-defined), Prow control plane pods, or deployment manifests.

**Suggested Action**:
- Keep open and continue triage
- Research what specific OPA-conflicting patterns exist and what changes could be made
- The issue would benefit significantly from augmentation with technical context

## Findings

(Further findings from triage subcommands will be added here)

## Next Steps

- Research: Investigate specific OPA conflict patterns and potential solutions
- Assess effort for addressing the identified areas
- Augment the issue with technical details
