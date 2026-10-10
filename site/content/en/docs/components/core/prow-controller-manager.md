---
title: "Prow-Controller-Manager"
weight: 50
description: >
  
---

`prow-controller-manager` manages the job execution and lifecycle for jobs running in k8s.

It currently acts as a replacement for [Plank].

It is intended to eventually replace other components, such as [Sinker] and [Crier].
See the tracking issue [#17024](https://github.com/kubernetes/test-infra/issues/17024) for details.

### Advantages

- Eventbased rather than cronbased, hence reacting much faster to changes in prowjobs or pods
- Per-Prowjob retrying, meaning genuinely broken prowjobs will not be retried forever and transient errors will be retried much quicker
- Uses a cache for the build cluster rather than doing a LIST every 30 seconds, reducing the load on the build clusters api server

### Exclusion with other components

This is mutually exclusive with only [Plank].
Only one of them may have more than zero replicas at the same time.

### Usage
```bash
$ go run ./cmd/prow-controller-manager --help
```

### Configuration

* [Deployment and RBAC manifest](https://github.com/kubernetes/k8s.io/blob/main/kubernetes/gke-prow/prow/prow-controller-manager.yaml)

#### Changes to disabled clusters

The `disabled_clusters` configuration lists build cluster names whose matching kubeconfig
contexts are ignored when Prow components load their kubeconfig files.
[PR #999](https://github.com/kubernetes-sigs/prow/pull/999) adds automatic recovery to
`prow-controller-manager` (PCM): when a configuration reload changes membership in this
set, PCM initiates graceful shutdown. The container supervisor must restart PCM after
it exits so that startup rebuilds build cluster clients, caches, and pod watches using
the updated configuration.

Deploy PCM with a supervisor that restarts it even after a successful exit. For a
Kubernetes Deployment, use the Pod's `restartPolicy: Always`; restarting only on failure
is insufficient for this graceful shutdown. Expect an interruption in job reconciliation
while PCM shuts down, restarts, and initializes its clients, caches, and watches.

Adding, removing, or replacing a member of `disabled_clusters` triggers this shutdown.
Equivalent sets do not: reordering names, adding or removing duplicates, and changing
between an omitted (`nil`) list and an empty list do not trigger it. Unrelated configuration
changes also do not trigger this restart. This behavior does not mean that all Prow
components automatically reload their cluster clients.

Configuration propagation, job assignment, and PCM shutdown are not coordinated.
For a configured cluster that is enabled in PCM's current configuration but has no
usable build client, Plank retries reconciliation every ten seconds for up to five
minutes from its first observation of the missing client. This applies to triggered
jobs, pending jobs (including pod cleanup), and aborted jobs awaiting pod deletion.
The first observation is persisted in the ProwJob annotation
`prow.k8s.io/missing-build-client-since`, so restarting PCM does not renew the window.
A successful reconciliation with a usable client clears the annotation. If the client
is still absent when the window expires, an incomplete job is completed as errored
with an explicit recovery-window-expired description. An invalid or future recovery
timestamp is terminal rather than granting an unlimited wait.

Configured cluster identities are read from kubeconfig at startup without filtering
`disabled_clusters`; current enablement is checked against PCM's loaded configuration.
Unknown aliases and deliberately disabled clusters with missing clients remain terminal.
Retries wait before allocating a build ID or creating a pod. Once the client is available,
normal reconciliation looks for an existing pod first, including one created before a
ProwJob status update failed.

This policy does not replace clients, caches, or watches at runtime. Recovery still
requires the existing restart mechanism and a working supervisor. New kubeconfig
identities are recognized after restart; the policy covers absent clients, not API
errors from existing clients. Existing pod timeouts still apply after recovery. Slow
restarts, configuration propagation delays, and jobs reaching PCM while it still sees
their cluster as disabled can still cause terminal failures. Restarting does not retry
jobs already marked errored. Operators must rerun affected jobs after configuration
has propagated and PCM has restarted; this mechanism does not guarantee a failure-free
transition.

[Plank]: /docs/components/deprecated/plank/
[Sinker]: /docs/components/core/sinker/
[Crier]: /docs/components/core/crier/
