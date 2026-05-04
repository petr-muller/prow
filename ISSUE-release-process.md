# Establish Prow release process and publish images to registry.k8s.io

## Problem

Prow has no formal release process. Images are built after every merge with
date-SHA versions (`v20250503-abc1234`) and pushed to a staging registry
(`us-docker.pkg.dev/k8s-infra-prow/images/`). This staging registry is not
intended for end-user consumption (see #113), provides no stability guarantees,
no changelogs, and no way for consumers to track what changed between versions.
The old `gcr.io/k8s-prow` registry is defunct since the Google-to-community
infra migration.

## Goal

Publish semver-tagged Prow images to `registry.k8s.io/prow/` via the standard
k8s image promotion process, with GitHub Releases that include changelogs.

## How it works

The existing staging build (Cloud Build postsubmit) continues unchanged — images
are built on every merge and pushed to staging with date-SHA tags. "Releasing"
means finding the sha256 digests of those staging images and writing them into an
`images.yaml` promotion manifest in `kubernetes/k8s.io`, mapped to a semver tag.
A promoter postsubmit then copies them to `registry.k8s.io/prow/` server-side.
No rebuild needed.

## Versioning

- Semantic versioning starting at `v0.1.0` (pre-1.0)
- Tags directly on `main`, no release branches
- No fixed cadence; release when meaningful changes accumulate
- Only the latest release supported initially

## Work Items

### Phase 1 — Release Process

- [ ] Resolve staging project naming with sig-k8s-infra: GCP project is
  `k8s-infra-prow` but promotion convention uses `k8s-staging-*` directory names
- [ ] Create `k8s-staging-prow` promotion directory in `kubernetes/k8s.io` with
  `OWNERS` and `images.yaml`
- [ ] Create `hack/check-release-images.sh` script for collecting staging image
  digests by commit SHA (based on [boskos#233](https://github.com/kubernetes-sigs/boskos/pull/233) template)
- [ ] Update `VersionTimestamp()` in `pkg/version/doc.go` to handle semver
  format (currently assumes `v${date}-${sha}` and will error on `v0.1.0`)
- [ ] Replace placeholder `RELEASE.md` with actual release process documentation
- [ ] Tag `v0.1.0` on `main`, verify staging images, file first promotion PR
- [ ] Create GitHub Release for `v0.1.0`

### Phase 2 — Consumer Migration

- [ ] Update starter configs to use `registry.k8s.io/prow/` (4 files in
  `config/prow/cluster/starter/`)
- [ ] Update documentation to reference `registry.k8s.io/prow/` (7+ doc files)
- [ ] Update `generic-autobumper` default registry from `gcr.io/k8s-prow`
- [ ] Update `cmd/branchprotector/oneshot-job.yaml`
- [ ] Announce migration on Slack (#prow, #sig-testing) and kubernetes-dev@
- [ ] Decide prow.k8s.io strategy: stay on staging as canary, or consume promoted

### Phase 3 — Cleanup

- [ ] Remove defunct `pkg/cip-manifest.yaml` (old gcr.io promoter config)
- [ ] Remove remaining `gcr.io/k8s-prow` references (~20 files)
- [ ] Update `Makefile` default REGISTRY from `gcr.io/k8s-prow`

## Release workflow summary

1. Pick a commit on `main`, tag it: `git tag -a v0.2.0 -m "Release v0.2.0"`
2. Run `hack/check-release-images.sh <7-char-sha>` to verify staging images and
   collect digests
3. File PR to `kubernetes/k8s.io` updating `registry.k8s.io/images/k8s-staging-prow/images.yaml`
4. After merge, promoter copies images to `registry.k8s.io/prow/`
5. Create GitHub Release with curated changelog

## Consumer impact

- **Non-breaking:** Staging images continue to be built on every merge. Promoted
  images at `registry.k8s.io` are additive.
- **Registry change:** `us-docker.pkg.dev/k8s-infra-prow/images/<component>` →
  `registry.k8s.io/prow/<component>`
- **Version format change:** `v20250503-abc1234` → `v0.1.0`
- Component image names stay the same (`hook`, `deck`, `clonerefs`, etc.)
- No hard cutoff for staging; it continues but is not advertised for external use

## Communication

- **GitHub Releases** as primary changelog (auto-generated from PR labels, curated)
- **Announcements page** (`docs.prow.k8s.io/docs/announcements/`) for breaking changes
- **Slack / kubernetes-dev@** for notable releases

## Related

- #113
- #559
- https://github.com/kubernetes-sigs/boskos/pull/233
