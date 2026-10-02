# Tide retesting of manually triggered presubmits

Checked: 2026-10-02. This report describes the Prow code in this worktree and the public `openshift/release` configuration. It does not verify the configuration currently loaded by the running OpenShift CI instance.

## Scenario

A PR has two presubmits. `ci-job-auto` has `always_run: true`; `ci-job-manual` has `always_run: false`, `optional: false`, and neither `run_if_changed` nor `skip_if_only_changed`. Both report status contexts. The automatic job passes, and someone runs the manual job with `/test ci-job-manual`; it also passes. The base branch then advances while the PR head stays the same.

The question is whether Tide reruns each job against the new base, and whether a manual job can be required *only after* someone has chosen to run it.

## Findings

| Branch-protection setting | Before the manual job is triggered | After it passes and the base advances |
| --- | --- | --- |
| `require_manually_triggered_jobs` unset or `false` | Its status is required only if present; the PR may merge without it. | Tide retests `ci-job-auto`, but **does not retest `ci-job-manual`**. The old manual status can remain green. |
| `require_manually_triggered_jobs: true` | The manual job's status must be present and passing to merge. Someone must initiate its first run. | Tide retests **both** jobs against the new base. |

Why:

1. A manual-only presubmit has `NeedsExplicitTrigger() == true`. `optional: false` with status reporting makes `ContextRequired() == true`. See [the presubmit predicates](pkg/config/jobs.go#L537-L552).
2. With the setting off, Prow classifies its context as **required if present**. With the setting on, it classifies the context as **always required**. See [branch requirement classification](pkg/config/branch_protection.go#L608-L634).
3. Tide selects manual-only jobs for retesting only when `require_manually_triggered_jobs` is enabled for that repo and branch, or the job has `run_before_merge: true`. Otherwise `ShouldRun(..., forced=false, defaults=false)` excludes the job. See [Tide's presubmit selection](pkg/tide/tide.go#L1693-L1743) and [ShouldRun](pkg/config/jobs.go#L515-L533).
4. Tide compares selected jobs with results for the current base and PR head. A result against the previous base is missing for the new base, so Tide schedules that selected job again. See [result accumulation](pkg/tide/tide.go#L1091-L1152) and [retest scheduling](pkg/tide/tide.go#L1592-L1597).

With the setting on, a missing first-run status prevents Tide from starting a retest: the [PR filter](pkg/tide/tide.go#L770-L807) rejects the missing required context, and [retest eligibility](pkg/tide/tide.go#L1264-L1283) also requires its presence. A pending first run may enter the pool, but it still cannot merge until it passes.

## OpenShift CI configuration

The [public main Prow config](https://github.com/openshift/release/blob/6d5dd900a60b2d90a28d3480fc5c36e7290e9b28/core-services/prow/02_config/_config.yaml#L724-L727) sets Tide's `from-branch-protection: true` but does **not** set `require_manually_triggered_jobs` globally. A search of the public `openshift/release` repository found one explicit `true`: the [repo-level policy for `openshift/assisted-installer-agent`](https://github.com/openshift/release/blob/6d5dd900a60b2d90a28d3480fc5c36e7290e9b28/core-services/prow/02_config/openshift/assisted-installer-agent/_prowconfig.yaml#L1-L12), with the branch patterns shown there. No other explicit occurrence was found in that public repository. These findings do not establish whether the running instance has loaded exactly this revision.

## Configuration gap

The desired behavior—**allow the PR to merge without a never-triggered manual job, but require and automatically retest that job once it has been triggered**—cannot be expressed with the existing options. `required-if-present` governs whether a visible status blocks merging; it does not make Tide select the manual job for retesting. Conversely, `require_manually_triggered_jobs: true` makes Tide select it but also requires its status before the first run. `run_before_merge: true` makes Tide run the job regardless of whether someone previously triggered it.

Supporting the desired behavior requires a Tide code change that selects an applicable manual-only presubmit when its context is present on the PR head. A new configuration field could make that behavior opt-in, but a field without the selection logic would not suffice. Tide's [serial](pkg/tide/tide.go#L1693-L1743) and [batch](pkg/tide/tide.go#L1748-L1798) selection paths both need consideration.

## Sources

- [Prow job trigger and merge behavior](https://docs.prow.k8s.io/docs/jobs/)
- [Prow branch requirement classification](pkg/config/branch_protection.go#L608-L634)
- [Tide presubmit selection and retest logic](pkg/tide/tide.go#L1091-L1152)
- [OpenShift CI main Prow configuration](https://github.com/openshift/release/blob/6d5dd900a60b2d90a28d3480fc5c36e7290e9b28/core-services/prow/02_config/_config.yaml#L724-L727)
- [OpenShift CI assisted-installer-agent policy](https://github.com/openshift/release/blob/6d5dd900a60b2d90a28d3480fc5c36e7290e9b28/core-services/prow/02_config/openshift/assisted-installer-agent/_prowconfig.yaml#L1-L12)
