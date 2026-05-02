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

## Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

### Summary

The recommended approach (opt-in SecurityContext extension + hardened starter manifests) is well-defined, follows existing patterns, and is fully backwards compatible. The work spans multiple files but each change is straightforward — adding struct fields, plumbing them through decoration, and updating YAML manifests. No algorithmic complexity or concurrency concerns.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Moderate
- **Details**: ~8-12 files affected, ~200-400 LOC. Key files: `types.go` (new DecorationConfig fields), `podspec.go` (container-level SecurityContext application), starter manifests (SecurityContext blocks), test files, documentation.
- **Level Indication**: 2-3

#### Complexity
- **Assessment**: Simple-Moderate
- **Details**: Adding new optional config fields and plumbing them into SecurityContext is mechanical. The only subtlety is ensuring `readOnlyRootFilesystem` works with utility container volume mounts — requires an audit of write paths, but the write operations already target volume mounts, not root filesystem.
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Moderate
- **Details**: Requires understanding of Kubernetes SecurityContext model, Prow's DecorationConfig mechanism, and `podspec.go` decoration flow. A contributor experienced with K8s security and Go can learn the Prow-specific patterns from existing code.
- **Level Indication**: 2-3

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: The Kubernetes SecurityContext API is standardized. The solution approach maps directly: add fields → apply in decoration → update manifests. No design ambiguity.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Moderate
- **Details**: Unit tests for new DecorationConfig fields in `podspec_test.go` (follow existing patterns for RunAsUser/RunAsGroup/FsGroup). Ideally add an integration test in a PSS-restricted namespace, but this is optional and can be a follow-up.
- **Level Indication**: 2-3

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: All new DecorationConfig fields are optional pointers — nil means "don't set". Existing configs work unchanged. Starter manifests are templates, not running configs.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Directly extends the existing DecorationConfig → SecurityContext pattern. The 3 existing fields (RunAsUser, RunAsGroup, FsGroup) establish the exact pattern to follow. Adding more SecurityContext fields is the natural evolution.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: None
- **Details**: All changes are internal to Prow. SecurityContext is a stable Kubernetes API with no compatibility concerns.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `help-wanted`: Well-defined, moderate scope, suitable for a skilled contributor
- [x] `kind/feature`: Adding new configuration capability
- [x] `area/pod-utils`: Primary changes in pod decoration
- [ ] `good-first-issue`: Requires understanding of Prow's decoration flow and K8s SecurityContext

### Guidance for Contributors

This is suitable for contributors familiar with Kubernetes SecurityContext and Go. The work can be split into independent PRs:

1. **PR 1 (smallest, good starting point)**: Update starter manifests with PSS "restricted" SecurityContext — pure YAML changes, immediately useful
2. **PR 2**: Extend DecorationConfig with new SecurityContext fields (RunAsNonRoot, AllowPrivilegeEscalation, ReadOnlyRootFilesystem, SeccompProfile, Capabilities)
3. **PR 3**: Apply container-level SecurityContext to utility containers in `decorateSpec()`
4. **PR 4**: Documentation for running Prow in PSS-restricted environments

Key code to review before starting:
- `pkg/pod-utils/decorate/podspec.go:831-844` — existing SecurityContext application
- `pkg/apis/prowjobs/v1/types.go:564-576` — existing DecorationConfig fields
- `pkg/pod-utils/decorate/podspec_test.go` — existing test patterns

### Caveats and Considerations

- The issue is very vague — the author may have specific OPA constraints beyond what PSS covers. The augmentation comment should invite them to share specifics.
- Level 2 assumes the recommended approach (opt-in). If the project decides to change defaults (Approach 1 or 3), the effort increases to Level 3 due to backwards compatibility concerns and migration work.
- The `readOnlyRootFilesystem` support requires auditing all utility container write paths — this is likely clean (all writes go to volumes), but needs verification.

## Proposed Issue Augmentation

### Title Change
- **Current**: "Restrict Prow for Users running in Environments with OPA constraints"
- **Proposed**: "Support Pod Security Standards (restricted profile) for ProwJob pods and control plane"
- **Rationale**: The current title is vague ("restrict Prow" is ambiguous — restrict what?). The new title names the specific Kubernetes standard (PSS), identifies what needs to change (ProwJob pods and control plane), and makes the issue discoverable by people searching for PSS/OPA/Gatekeeper compatibility.

### Proposed GitHub Comment

```
/retitle Support Pod Security Standards (restricted profile) for ProwJob pods and control plane

Prow's core code does not actually require elevated privileges — it doesn't hardcode privileged mode, doesn't mount `docker.sock`, and doesn't run as root. The DecorationConfig already supports configurable `RunAsUser`, `RunAsGroup`, and `FsGroup` via `pkg/apis/prowjobs/v1/types.go`. However, Prow currently cannot pass Pod Security Standards (PSS) "restricted" profile admission because of two gaps:

1. **ProwJob pods**: The utility containers injected by decoration (clonerefs, initupload, entrypoint, sidecar) in `pkg/pod-utils/decorate/podspec.go` are created without container-level SecurityContext. PSS "restricted" requires each container to explicitly set `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, and `seccompProfile.type: RuntimeDefault`. The DecorationConfig has no fields for these container-level settings.

2. **Control plane pods**: The starter deployment manifests (`config/prow/cluster/starter/`) ship without any SecurityContext — no `runAsNonRoot`, no capability drops, no seccomp profile. In a PSS-restricted namespace, these deployments would be rejected by admission.

The fix would extend DecorationConfig with additional SecurityContext fields (following the existing pattern for `RunAsUser`/`RunAsGroup`/`FsGroup`) and apply them to both pod-level and container-level contexts during decoration. Starter manifests should also be updated with PSS "restricted"-compatible defaults. This can be done in a backwards-compatible way since all new fields would be optional — nil means "don't set". The work splits naturally into independent PRs: manifest hardening, DecorationConfig extension, container-level SecurityContext plumbing, and documentation. If you have specific OPA constraints you're hitting beyond what PSS covers, sharing those details would help scope this further.

/area pod-utilities
/kind feature
/help-wanted
/remove-lifecycle stale
```

### Rationale

**What's being added**:
- Root cause explanation: Prow doesn't need privileges but lacks the SecurityContext boilerplate for PSS compliance
- Specific technical gaps: container-level SecurityContext on utility containers, and missing SecurityContext in starter manifests
- Code locations for each gap
- Implementation approach (extend existing DecorationConfig pattern)
- PR decomposition strategy
- Invitation for the author to share specific OPA constraints

**Why these labels**:
- `/area pod-utilities`: Primary changes affect pod decoration and utility containers
- `/kind feature`: This is a new configuration capability, not a bug
- `/help-wanted`: Level 2 effort — well-defined, moderate scope, suitable for a skilled contributor
- `/remove-lifecycle stale`: Issue is being actively triaged

**What's NOT included**:
- Priority label: not warranted — this is an enhancement, not blocking anyone
- Multiple area labels: while control plane manifests are also affected, the core code change is in pod-utilities
- Detailed solution approaches: kept it to one paragraph to avoid overwhelming the comment; the triage document has the full analysis

## Briefing Completed

Briefed maintainer on: 2026-05-02

Key questions asked:
- None — maintainer acknowledged all slides without questions

Maintainer decision:
Proceed with posting the augmentation comment.
