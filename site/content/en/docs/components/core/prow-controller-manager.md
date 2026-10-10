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
During the transition, jobs can still be assigned to a cluster for which PCM has no
build client and receive a terminal missing-client error. Restarting restores PCM's
ability to reconcile jobs on enabled clusters, but does not retry jobs already marked
errored. Operators must rerun affected jobs after the cluster configuration has propagated
and PCM has restarted; this mechanism does not guarantee a failure-free transition.

[Plank]: /docs/components/deprecated/plank/
[Sinker]: /docs/components/core/sinker/
[Crier]: /docs/components/core/crier/
