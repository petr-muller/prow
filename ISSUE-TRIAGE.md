# Triage for Issue #494

**Status**: In Progress
**Created**: 2026-04-10

## Issue Information

- **Issue Number**: #494
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/494

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue requests adding a `Flaky` boolean field to the `ProwJobSpec` struct, inspired by Bazel's flaky test attribute. The author envisions this field controlling service logic, retries, and logic forks for jobs known to be flaky.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Component mentioned: `ProwJobSpec`
- Exists in this repo: Yes (`pkg/apis/prowjobs/v1/types.go:141`)
- Relevant code paths: ProwJob API types, Prow controller/plank, tide, crier

**Information Completeness**:
- Sufficient detail provided: Partially — the request is clear but lacks specifics about which Prow components would consume this field and how
- Missing information: Concrete use cases, expected behavior changes in specific components (plank, tide, crier), interaction with existing retry mechanisms

**Maintainer Discussion**:
- BenTheElder (member): Skeptical — jobs are either required or not, retries already possible via robot commenter. Suggests annotations/labels for separate components (like testgrid's approach).
- petr-muller (member): Agrees this shouldn't be a core ProwJob property but is open to optional/separate component-provided behaviors.

### Recommendation

This is a legitimate feature request targeting the ProwJob API in this repository. However, the maintainer consensus leans toward using annotations/labels rather than a first-class spec field. The issue is worth keeping as a record of this design discussion and as a potential enhancement if a compelling use case emerges.

**Suggested Action**:
- Keep open and continue triage
- Research how annotations/labels could address this vs. a spec field
- Assess effort for both approaches

## Code Research

### Current Implementation

**Primary Components**:
- ProwJobSpec: `pkg/apis/prowjobs/v1/types.go:141-236` — CRD API struct with behavior-control fields
- Job config layer: `pkg/config/jobs.go:104-231` — User-facing job configuration structs (Presubmit, Postsubmit, Periodic)
- ProwJob labels/annotations: `pkg/kube/prowjob.go:19-88` — Standard label/annotation constants
- Spec conversion: `pkg/pjutil/pjutil.go` — Converts config-layer jobs to ProwJobSpec

**Architecture Overview**:
Prow has a two-layer job definition architecture. Users define jobs in config (Presubmit/Postsubmit/Periodic structs in `pkg/config/jobs.go`). When triggered, these are converted to ProwJob CRDs via `pkg/pjutil/pjutil.go`. The ProwJobSpec carries the runtime configuration, consumed by plank (execution), tide (merge gating), and crier (reporting).

### Existing Job Behavior Controls

Several fields already control how Prow treats jobs differently:

| Field | Where | Purpose |
|-------|-------|---------|
| `Optional` bool | `config/jobs.go:202` (Presubmit only) | Marks job as non-blocking for merge |
| `IsOptionalLabel` | `kube/prowjob.go:71` | Label propagated from `Optional` field |
| `Report` bool | `types.go:167` | Controls whether results are reported |
| `SkipReport` bool | `config/jobs.go:392` | Config-layer equivalent (inverted) |
| `MaxConcurrency` int | `types.go:179` | Limits parallel job instances |
| `ErrorOnEviction` bool | `types.go:184` | Controls eviction behavior |
| `Hidden` bool | `types.go:222` | Hides from Deck UI |
| `Retry` struct | `config/jobs.go:283-295` | Retry config for Periodic jobs only |

### How Tide Uses Optional/Required

- `pkg/tide/tide.go:814-825`: `requiredContextsMap()` builds a map of required job contexts, excluding optional jobs
- `pkg/tide/status.go:768-774`: Uses `RequiredContexts` policy to check for incomplete required jobs
- `pkg/tide/tide.go:871`: `contextChecker.IsOptional()` filters contexts during merge decisions

A `Flaky` field would interact most naturally with this optional/required distinction.

### How Retries Work Today

- **Periodic jobs**: Have a `Retry` struct (`config/jobs.go:283-295`) with `Attempts`, `RunAll`, and `Interval` fields, implemented in horologium (`cmd/horologium/main.go:245-289`)
- **Presubmit/Postsubmit**: No built-in retry. Users re-trigger via `/retest` commands handled by the trigger plugin (`pkg/plugins/trigger/generic-comment.go`)
- **No automatic flaky retry**: Prow has no concept of automatically retrying failed jobs that might be flaky

### TestGrid Annotation Precedent

TestGrid integration works entirely through annotations on ProwJobs, not through spec fields. Job config provides `Annotations map[string]string` (`config/jobs.go:134-135`) which propagates to ProwJob ObjectMeta via `pjutil.go:169`. This is the model BenTheElder referenced as precedent.

### Crier/Reporter Flow

- `pkg/crier/controller.go:38-47`: Reporter interface receives full ProwJob
- `pkg/crier/controller.go:119-128`: `ShouldReport()` filtering happens before reporters
- `pkg/github/report/report.go:297-315`: GitHub reporter uses `IsOptionalLabel` to annotate status in PR comments

### Root Cause Analysis

**Primary Cause**:
This is a feature gap, not a bug. Prow has no way to declare a job as "known flaky" and have components adjust behavior accordingly. The issue is well-motivated — flaky tests are a universal CI pain point — but the request for a ProwJobSpec field is architecturally problematic because:

1. The ProwJobSpec is a CRD API — adding fields has API versioning implications
2. "Flaky" semantics are ambiguous: different components would interpret it differently
3. The existing `Optional` field already covers the "don't block merge" use case
4. Annotations/labels provide the same extensibility without API surface commitment

**Contributing Factors**:
1. No clear definition of what "flaky" means across components (retry? ignore failure? change reporting?)
2. Existing mechanisms (Optional, SkipReport, /retest) already cover most concrete use cases
3. The gap is more about user experience (no single "flaky" knob) than missing functionality

### Proposed Solutions

#### Approach 1: Annotation-Based Flaky Signal (Maintainer-Preferred)

**Description**: Define a standard annotation (e.g., `prow.k8s.io/flaky: "true"`) that components can independently opt into consuming. No API changes required.

**Pros**:
- No CRD schema changes or API versioning concerns
- Each component can independently decide what "flaky" means for its context
- Follows established testgrid annotation pattern
- Backwards compatible by default
- Can be adopted incrementally

**Cons**:
- No compile-time guarantees on annotation values
- Requires each component to independently implement support
- Semantics may drift across components without coordination

**Affected Components**:
- `pkg/kube/prowjob.go`: Add annotation constant
- Individual components (tide, crier, plank): Opt-in to reading annotation

**Complexity**: Low per-component, Medium total

**Backwards Compatibility**: Fully compatible — annotation ignored until components add support

#### Approach 2: ProwJobSpec Field (Author's Request)

**Description**: Add `Flaky bool` to ProwJobSpec, with well-defined semantics for each consuming component.

**Pros**:
- Single clear API surface
- Compile-time type safety
- Centralized documentation of behavior
- Discoverable via API schema

**Cons**:
- CRD schema change required
- Must define complete semantics upfront for all components
- API surface commitment — hard to change later
- Overlap with existing `Optional` field unclear
- Maintainers have expressed opposition to this approach

**Affected Components**:
- `pkg/apis/prowjobs/v1/types.go`: New spec field
- `pkg/config/jobs.go`: New config field
- `pkg/pjutil/pjutil.go`: Conversion logic
- CRD YAML: Schema update
- Generated deepcopy files
- All consuming components

**Complexity**: High

**Backwards Compatibility**: Additive field, compatible but locks in API

#### Recommendation

**Preferred Approach**: Approach 1 (Annotation-Based)

This aligns with maintainer consensus and established patterns (testgrid). The key insight is that "flaky" is not a single behavior — it means different things to different components. An annotation lets each component define its own semantics without locking in a one-size-fits-all definition in the API.

**Key Implementation Considerations**:
1. Define the annotation constant and document expected values
2. Start with one component (likely tide or crier) as proof of concept
3. Document per-component behavior in annotation godoc
4. Consider whether `Optional` already covers the most important use case (merge gating)

**Testing Requirements**:
- Unit tests for annotation parsing in each consuming component
- Integration test showing annotation affects behavior end-to-end

## Next Steps

(Action items will be added here)
