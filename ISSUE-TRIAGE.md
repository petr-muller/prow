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

## Findings

(Further findings from triage subcommands will be added below)

## Next Steps

(Action items will be added here)
