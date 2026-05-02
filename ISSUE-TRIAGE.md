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

## Code Research

### Current Implementation

**Primary Components**:
- **Pod Decoration**: `pkg/pod-utils/decorate/podspec.go` — converts ProwJobs to Pods, applies SecurityContext
- **DecorationConfig API**: `pkg/apis/prowjobs/v1/types.go` — defines configurable security fields (RunAsUser, RunAsGroup, FsGroup)
- **Prow Config**: `pkg/config/config.go` — global default decoration configs, filterable by org/repo/job
- **Admission Webhook**: `cmd/admission/admission.go` — validates ProwJob spec immutability (no security validation)

**Architecture Overview**:
The security model has two distinct surfaces:
1. **Control plane pods** (deck, hook, sinker, plank, tide, crier, horologium) — deployed via manifests outside Prow's code
2. **ProwJob pods** — created by plank/crier via `ProwJobToPod()`, with SecurityContext applied from DecorationConfig

### Key Findings

#### 1. Control Plane Pods Have NO SecurityContext

All starter and integration test deployment manifests ship without any SecurityContext:
- `config/prow/cluster/starter/starter-gcs.yaml`
- `test/integration/config/prow/cluster/deck_deployment.yaml`
- `test/integration/config/prow/cluster/hook_deployment.yaml`
- `test/integration/config/prow/cluster/crier_deployment.yaml`
- `test/integration/config/prow/cluster/horologium_deployment.yaml`
- `test/integration/config/prow/cluster/tide_deployment.yaml`

Missing from all deployments:
- `runAsNonRoot: true`
- `allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true`
- `capabilities.drop: [ALL]`
- `seccompProfile.type: RuntimeDefault`

This means in a PSS-restricted namespace, Prow control plane pods would fail admission.

#### 2. ProwJob Pods: SecurityContext is Partially Configurable

`podspec.go:831-844` — `decorateSpec()` applies SecurityContext from DecorationConfig, but only:
- `RunAsUser` (pod-level)
- `RunAsGroup` (pod-level)
- `FsGroup` (pod-level)

NOT configurable via DecorationConfig:
- `RunAsNonRoot`
- `AllowPrivilegeEscalation` (container-level)
- `ReadOnlyRootFilesystem` (container-level)
- `Capabilities` (container-level)
- `SeccompProfile` (pod or container-level)

#### 3. Utility Containers Have No Container-Level SecurityContext

The four utility containers injected by decoration (clonerefs, initupload, entrypoint, sidecar) are created at `podspec.go:494-505, 569-579, 649-661, 937-950` without any container-level SecurityContext. They inherit pod-level settings only.

In a PSS "restricted" environment, each container must explicitly set:
- `allowPrivilegeEscalation: false`
- `capabilities.drop: [ALL]`
- `seccompProfile.type: RuntimeDefault`

#### 4. Utility Container File System Operations

Pod utilities write to mounted volumes at runtime:
- **clonerefs**: writes clone logs (`os.WriteFile` at `pkg/clonerefs/run.go:169`), creates SSH dirs (`os.MkdirAll` at `:220`)
- **entrypoint**: creates dirs (`os.MkdirAll` at `cmd/entrypoint/main.go:48-51`)
- **sidecar**: creates output dirs (`os.MkdirAll` at `pkg/sidecar/censor.go:296`)

These operations write to emptyDir/volume mounts (`/logs`, `/tools`, `/home/prow/go`), not the root filesystem. This means `readOnlyRootFilesystem: true` should work IF all writable paths are covered by volume mounts.

#### 5. No Validation or Policy Integration

The admission webhook (`cmd/admission/admission.go`) only validates ProwJob spec immutability. There is no:
- Security validation of ProwJob pod specs
- OPA/Gatekeeper integration
- Pod Security Standards enforcement
- Warning when users submit ProwJobs with privileged settings

### Root Cause Analysis

**Primary Cause**: Prow was developed before Pod Security Standards became the norm (PSP was deprecated in K8s 1.21, removed in 1.25). The codebase never adopted security hardening defaults — not because it needs privileges, but because no one added the boilerplate.

**Contributing Factors**:
1. DecorationConfig only exposes 3 of the ~8 SecurityContext fields needed for PSS "restricted" compliance
2. Container-level SecurityContext (needed for PSS "restricted") is not configurable at all for utility containers
3. Starter manifests serve as de facto documentation and don't demonstrate security best practices
4. No e2e tests run in PSS-restricted namespaces, so compliance regressions are undetected

### Proposed Solutions

#### Approach 1: Hardened Defaults in Code and Manifests

**Description**: Add PSS "restricted"-compatible SecurityContext defaults to both control plane manifests and utility container creation code. Make Prow "secure by default" without requiring configuration.

**Changes**:
- Update starter manifests and integration test deployments to include SecurityContext with `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]`, `seccompProfile.type: RuntimeDefault`
- Update `podspec.go` to set container-level SecurityContext on utility containers (clonerefs, initupload, entrypoint, sidecar) with the same restricted defaults
- Extend DecorationConfig to expose additional SecurityContext fields (RunAsNonRoot, AllowPrivilegeEscalation, ReadOnlyRootFilesystem, SeccompProfile) so operators can tune per-job
- Ensure all writable paths in utility containers are volume-mounted (emptyDir)

**Pros**:
- Secure by default — works out of the box in PSS-restricted environments
- Aligns with K8s security best practices
- Benefits all users, not just those with OPA

**Cons**:
- Breaking change for users whose ProwJobs depend on running as root or with privileges
- Requires careful audit of all file system operations in utility containers
- May break existing deployments that don't expect these restrictions

**Complexity**: Medium
**Backwards Compatibility**: Potentially breaking — existing ProwJobs that run as root would fail

#### Approach 2: Opt-in SecurityContext via Extended DecorationConfig

**Description**: Extend DecorationConfig with additional SecurityContext fields and document how operators can opt-in to PSS compliance. Don't change defaults.

**Changes**:
- Add new fields to DecorationConfig: `RunAsNonRoot`, `AllowPrivilegeEscalation`, `ReadOnlyRootFilesystem`, `SeccompProfile`, `Capabilities`
- Apply these to both pod-level and container-level SecurityContext in `decorateSpec()`
- Update documentation with a "Running Prow in PSS-restricted environments" guide
- Provide example configs for PSS "baseline" and "restricted" profiles

**Pros**:
- No breaking changes
- Operators can gradually adopt stricter security
- Flexible per-org/repo/job configuration via existing filter mechanism

**Cons**:
- Not secure by default — new deployments remain permissive
- Doesn't address control plane manifests
- Puts burden on operators to discover and configure

**Complexity**: Low-Medium
**Backwards Compatibility**: Fully backwards compatible

#### Approach 3: Combined — Hardened Defaults + Opt-out

**Description**: Set secure defaults everywhere but allow operators to relax them via configuration. This is the standard K8s approach (secure defaults, explicit opt-out).

**Changes**:
- Implement all changes from Approach 1 (hardened defaults)
- Extend DecorationConfig (as in Approach 2) to allow relaxing restrictions
- Add a global config flag like `security_profile: restricted|baseline|privileged` for easy tuning
- Provide migration documentation

**Pros**:
- Best of both approaches
- Follows K8s security patterns
- Gradual migration path via config

**Cons**:
- Most complex to implement
- Still potentially breaking for existing users
- Requires thorough testing

**Complexity**: Medium-High
**Backwards Compatibility**: Breaking, but with opt-out mechanism

#### Recommendation

**Preferred Approach**: Approach 2 (Opt-in via Extended DecorationConfig) for code changes, combined with updating starter manifests to include hardened SecurityContext (from Approach 1). This gives:
- No breaking changes for existing users
- Immediate benefit for new deployments following starter manifests
- Clear path for operators to opt in to stricter security
- The manifests changes are low-risk since they're templates, not running configs

**Key Implementation Considerations**:
1. Extend DecorationConfig with container-level SecurityContext fields
2. Apply container-level SecurityContext to all utility containers in `decorateSpec()`
3. Update starter manifests with PSS "restricted" compatible SecurityContext
4. Audit utility container file operations for readOnlyRootFilesystem compatibility
5. Add integration tests running in a PSS-restricted namespace
6. Document the configuration in Prow's site docs

## Next Steps

- Assess effort level for the recommended approach
- Augment the issue with technical details and solution proposal
