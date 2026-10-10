---
title: "override"
weight: 10
description: >
  Override status contexts and check runs on pull requests.
---

The `override` plugin lets authorized users mark failed or pending checks as
successful on an open pull request. It supports GitHub status contexts, Prow
presubmit job names, and GitHub check run names, including queued and in-progress
check runs.

## Configuration

Enable the plugin for the desired organization or repository in `plugins.yaml`:

```yaml
plugins:
  org/repo:
    plugins:
    - override

override:
  allow_top_level_owners: true
  allowed_github_teams:
    org:
    - release-team
    org/repo:
    - repo-maintainers
```

Repository administrators can use all three commands by default. The optional
`allow_top_level_owners` setting also allows approvers in the top-level `OWNERS`
file on the pull request's base branch. `allowed_github_teams` allows members of
the specified GitHub teams; use team **slugs**, with organization or
`org/repo` keys. Organization and repository entries both apply to that
repository. These settings grant the same access to creating and cancelling
overrides.

Overriding and cancelling GitHub check runs requires Prow to authenticate as a
[GitHub App](/docs/github-api-access/).

## Commands

Post commands on their own lines in a new comment on an open pull request.
`/override` and `/override-sticky` each require at least one context or job name.
Separate multiple names with spaces and put names containing spaces in double
quotes:

```text
/override ci/prow/unit-tests
/override pull-repo-integration "test / Unit Tests"
/override-sticky ci/prow/soak-gate "test / Integration Tests"
```

Use the exact status context or check run name. For a Prow presubmit with a
nonpassing status, its configured job name is also accepted. Required contexts
from branch protection can be overridden even if they have not reported yet.
An already passing or already overridden check needs no new override. If a
request includes an unknown name or a name rejected as already passing, Prow
reports it and applies none of the requested overrides; retry with only the
names that can be overridden.

### Status contexts and Prow jobs

`/override` writes a successful status on the current HEAD commit. For a
configured Prow presubmit, it also creates a completed successful ProwJob.
Both use the base branch tip at the time of the command, even if the original
test ran against an older base commit. Existing unfinished ProwJobs for that
presubmit on the pull request are aborted and have reporting disabled so their
results cannot replace the override.

Tide can use the regular override result while the base SHA matches. Once the
base branch moves, that result no longer satisfies Tide's requirement for
testing against the current base, so Tide can retest the job. The successful
GitHub status itself is not automatically deleted when the base moves.

`/override-sticky` additionally marks the status so Tide accepts it across base
branch changes on the **same HEAD commit**, including after the original
ProwJob has been cleaned up. A new HEAD commit has its own checks and does not
inherit either kind of override.

A plain `/retest` selects failed or missing status contexts and therefore
normally leaves a successful overridden context alone. An explicit
`/test job-name`, a matching custom job trigger, or `/test all` can start a new
job for an overridden context. That job can report a new status and replace
the override, including its sticky marker. Sticky overrides do not prevent
explicit reruns or guarantee that every later status update preserves them.

### GitHub check runs

For a check run, Prow creates a separate completed successful check run with
the same name on the current HEAD commit. It does not change the original
check run. Tide selects the best result among check runs with that name, so
the successful override can take precedence over the original failure or
pending result, including after the original check is rerun.

`/override` and `/override-sticky` behave the same for check runs: the override
run has no base SHA or sticky status marker. Base branch movement does not
expire it. It applies to the current HEAD commit until cancelled; a new HEAD
commit does not inherit it. Rerunning the original check does not itself
cancel Prow's separate successful override run.

### Cancelling overrides

Cancel a named status context or check run, quoting names with spaces, or omit
arguments to cancel all overrides on the pull request's current HEAD commit:

```text
/override-cancel ci/prow/unit-tests
/override-cancel "test / Unit Tests"
/override-cancel
```

Cancellation applies to both regular and sticky overrides. For statuses, Prow
writes a failure status and removes the sticky marker. Use the status context
name when cancelling; a Prow job name that differs from its context is not
resolved by this command. Cancellation does not automatically start a test;
use `/retest` or `/test job-name` to obtain a new result.

For check runs, Prow marks its successful override runs as `cancelled`. It
only updates override runs created by the GitHub App used by this Prow
instance. Tide then evaluates the remaining results with the same name,
including the original check run; another successful run can still make the
check pass. Prow reports any overrides it cannot cancel, such as override runs
created by a different GitHub App.
