# Prow Release Strategy

## Context

Prow currently has no formal release process. After each merge, a Cloud Build
postsubmit builds images tagged `vYYYYMMDD-<git-sha>` and pushes them to a
staging registry at `us-docker.pkg.dev/k8s-infra-prow/images/`. Users consume
these staging images directly, which violates k8s infra conventions (staging
registries are not intended for end-user consumption), provides no changelogs or
version stability guarantees, and uses the defunct `gcr.io/k8s-prow` in
documentation/examples. The old `gcr.io/k8s-prow` registry has been defunct
since the Google-to-community infra migration in August 2024.

The goal is to establish a proper semver-based release process that publishes
images to `registry.k8s.io/prow/` via the standard k8s image promotion
mechanism, provides release notes, and gives consumers a stable, supported
image source.

### How K8s Image Promotion Works

The process does NOT rebuild images. It's a registry-level copy by digest:

1. **Staging build** (already happening): Cloud Build postsubmit builds images
   after every merge and pushes to staging registry with date-sha tags.
2. **Promotion manifest**: A file `images.yaml` in the `kubernetes/k8s.io` repo
   maps sha256 digests to desired tags.
3. **Promoter job**: A Prow postsubmit watching `kubernetes/k8s.io` performs a
   server-side copy from staging to `registry.k8s.io`, applying the new tags.

So "releasing" is essentially: find the digests of the staging images you want,
write them into `images.yaml` mapped to a semver tag, and merge the PR.

### Current Build Infrastructure

| Component | Location | Notes |
|-----------|----------|-------|
| Image build tool | `hack/tools/prowimagebuilder/main.go` | Uses `ko`, builds 43 components |
| Image list | `.prow-images.yaml` | All component dirs and arch config |
| Ko config | `.ko.yaml` | Base images, ldflags for version injection |
| Cloud Build | `cloudbuild.yaml` | Postsubmit, pushes to staging |
| Version package | `pkg/version/doc.go` | `Version` and `Name` vars via ldflags |
| Version format | `gitTag()` in prowimagebuilder | `vYYYYMMDD-<git-describe>` |
| Staging registry | `us-docker.pkg.dev/k8s-infra-prow/images/` | GCP project `k8s-infra-prow` |
| Legacy registry | `gcr.io/k8s-prow` | Defunct since Aug 2024 |
| Legacy promoter | `pkg/cip-manifest.yaml` | Old gcr.io edge->prod config, defunct |
| RELEASE.md | Template placeholder | From kubernetes-template-project, not real |
| Announcements | `site/content/en/docs/announcements.md` | Manually curated Hugo page, last updated Apr 2024 |

---

## 1. High-Level Transition Strategy

### Three phases, each independently shippable:

**Phase 1: Establish release process and publish to registry.k8s.io**
- Define semver scheme (start at `v0.1.0`)
- Create release tooling (digest collection script)
- Set up promotion manifests in `kubernetes/k8s.io`
- Cut first release, file first promotion PR
- Publish GitHub Release with changelog
- Document the process in `RELEASE.md`

**Phase 2: Migrate consumers to registry.k8s.io**
- Update all docs, starter configs, and examples to `registry.k8s.io/prow/`
- Update `generic-autobumper` defaults
- Decide prow.k8s.io deployment strategy (canary on staging vs promoted)
- Announce migration path for external consumers

**Phase 3: Clean up legacy references**
- Remove defunct `pkg/cip-manifest.yaml`
- Remove `gcr.io/k8s-prow` references (20 files)
- Update `Makefile` default `REGISTRY` variable
- Add deprecation notice for direct staging consumption

---

## 2. GitHub Issue Proposal

### Title: "Establish Prow release process and publish images to registry.k8s.io"

### Body:

**Problem:**
Prow has no formal release process. Images are built after every merge with
date-SHA versions (`v20250503-abc1234`) and pushed to a staging registry
(`us-docker.pkg.dev/k8s-infra-prow/images/`). This staging registry is not
intended for end-user consumption (see #113), provides no stability guarantees,
no changelogs, and no way for consumers to track what changed between versions.
The old `gcr.io/k8s-prow` registry is defunct since the Google-to-community
infra migration.

**Goal:**
Publish semver-tagged Prow images to `registry.k8s.io/prow/` via the standard
k8s image promotion process, with GitHub Releases that include changelogs.

**Work Items:**

Phase 1 - Release Process:
- [ ] Resolve staging project naming with sig-k8s-infra: GCP project is
  `k8s-infra-prow` but promotion convention uses `k8s-staging-*` directory names
- [ ] Create `k8s-staging-prow` promotion directory in `kubernetes/k8s.io` with
  `OWNERS` and `images.yaml`
- [ ] Create `hack/check-release-images.sh` script for collecting staging image
  digests by commit SHA
- [ ] Update `VersionTimestamp()` in `pkg/version/doc.go` to handle semver
  format (currently assumes `v${date}-${sha}` and will error on `v0.1.0`)
- [ ] Replace placeholder `RELEASE.md` with actual release process documentation
- [ ] Tag `v0.1.0` on `main`, verify staging images, file first promotion PR
- [ ] Create GitHub Release for `v0.1.0`

Phase 2 - Consumer Migration:
- [ ] Update starter configs to use `registry.k8s.io/prow/` (4 files in
  `config/prow/cluster/starter/`)
- [ ] Update documentation to reference `registry.k8s.io/prow/` (7+ doc files)
- [ ] Update `generic-autobumper` default registry from `gcr.io/k8s-prow`
- [ ] Update `cmd/branchprotector/oneshot-job.yaml`
- [ ] Announce migration on Slack (#prow, #sig-testing) and kubernetes-dev@
- [ ] Decide prow.k8s.io strategy: stay on staging as canary, or consume promoted

Phase 3 - Cleanup:
- [ ] Remove defunct `pkg/cip-manifest.yaml` (old gcr.io promoter config)
- [ ] Remove remaining `gcr.io/k8s-prow` references (20 files)
- [ ] Update `Makefile` default REGISTRY from `gcr.io/k8s-prow`

**Related:** #113, #559

---

## 3. Release Process Proposal

### 3.1 Versioning

- Semantic versioning: `vMAJOR.MINOR.PATCH`
- Start at `v0.1.0` (pre-1.0 signals breaking changes possible in minor releases)
- No fixed cadence initially; release when meaningful changes accumulate
- Only the latest release is supported (no backport maintenance)

### 3.2 Branching

- Tags directly on `main`, no release branches
- If a patch is ever needed on an older release, a branch can be created
  retroactively from the tag, but this is not expected to be common

### 3.3 Release Workflow

**Step 1: Identify the release commit**

Pick a commit on `main` that you want to release. Verify its staging images
were built successfully by checking the Cloud Build postsubmit.

**Step 2: Tag the release**

```bash
git tag -a v0.2.0 -m "Release v0.2.0" <commit-sha>
git push upstream v0.2.0
```

**Step 3: Verify staging images and collect digests**

Run the digest collection script with the first 7 characters of the commit SHA:

```bash
./hack/check-release-images.sh <7-char-sha>
```

The script queries the staging registry for all 43 Prow component images
matching the commit SHA tag, verifies they all exist, and outputs their sha256
digests in the `images.yaml` format ready for the promotion PR.

On failure, investigate the postsubmit build job before proceeding.

**Step 4: File promotion PR to kubernetes/k8s.io**

Using the script output from Step 3, update
`registry.k8s.io/images/k8s-staging-prow/images.yaml` in the `kubernetes/k8s.io`
repository. Map the digests to the semver tag:

```yaml
- name: hook
  dmap:
    "sha256:abc123...": ["v0.2.0"]
- name: deck
  dmap:
    "sha256:def456...": ["v0.2.0"]
# ... all 43 components
```

Open a PR, get it reviewed by k8s.io OWNERS and Prow maintainers.

**Step 5: Wait for image promotion**

After the PR merges, the promoter postsubmit copies images from staging to
`registry.k8s.io/prow/`. Verify with:

```bash
crane digest registry.k8s.io/prow/hook:v0.2.0
```

**Step 6: Create GitHub Release**

Create a GitHub Release for the tag with curated changelog:

```markdown
## What's Changed

### Breaking Changes
- ...

### New Features
- ...

### Bug Fixes
- ...

## Container Images

Images available at `registry.k8s.io/prow/<component>:v0.2.0`

Components: hook, deck, tide, crier, sinker, horologium, prow-controller-manager,
gerrit, sub, gangway, webhook-server, exporter, moonraker, ...

## Full Changelog
https://github.com/kubernetes-sigs/prow/compare/v0.1.0...v0.2.0
```

**Step 7: Announce (for significant releases)**

- Update `site/content/en/docs/announcements.md` if there are breaking changes
- Post to Slack #prow / #sig-testing
- Email kubernetes-dev@ for major milestones

### 3.4 Digest Collection Script

Based on the Boskos PR (kubernetes-sigs/boskos#233) `hack/check_images.sh`
template, adapted for Prow:

- Registry: `us-docker.pkg.dev/k8s-infra-prow/images` (not `gcr.io/k8s-staging-*`)
- Uses `gcloud container images list-tags` or `crane` to find images by commit SHA
- Verifies all 43 component images exist for the given commit
- Outputs digests in `images.yaml` dmap format, ready to paste into the
  promotion manifest
- Should be idempotent and safe to re-run

### 3.5 Promotion Manifest Setup

**In `kubernetes/k8s.io` repository, create:**

`registry.k8s.io/images/k8s-staging-prow/OWNERS`:
```yaml
reviewers:
  - petr-muller
  - BenTheElder
  - cjwagner
approvers:
  - petr-muller
  - BenTheElder
  - cjwagner
```

`registry.k8s.io/images/k8s-staging-prow/images.yaml`:
```yaml
# Initially populated with v0.1.0 digests from check-release-images.sh
- name: hook
  dmap:
    "sha256:...": ["v0.1.0"]
# ... all components
```

**Open question to resolve first:** The staging GCP project is `k8s-infra-prow`
(at `us-docker.pkg.dev/k8s-infra-prow/images/`), not `k8s-staging-prow`. The
promoter manifest directory naming convention is `k8s-staging-*`. Need to confirm
with sig-k8s-infra whether:
- The promoter can be configured to pull from `k8s-infra-prow` while the
  directory is named `k8s-staging-prow`
- Or whether a new staging project following the standard naming is needed

This is the most critical prerequisite to resolve before the first release.

### 3.6 Changes Needed in Prow Codebase

| File | Change |
|------|--------|
| `RELEASE.md` | Replace template with real release process docs |
| `pkg/version/doc.go` | Update `VersionTimestamp()` to handle semver gracefully |
| `hack/check-release-images.sh` | New script for digest collection |
| `pkg/cip-manifest.yaml` | Delete (defunct legacy) |

The existing build infrastructure (`cloudbuild.yaml`, `prowimagebuilder`,
`.ko.yaml`, `.prow-images.yaml`) needs no changes. Staging builds continue
exactly as they do today. The release process is purely additive.

---

## 4. Consumer Impact Analysis

### 4.1 Current Consumers

| Consumer | Registry | Version Format | Impact |
|----------|----------|----------------|--------|
| prow.k8s.io | `us-docker.pkg.dev/k8s-infra-prow/images/` | `v20240802-66b115076` | Low - can migrate on own schedule |
| OpenShift CI | staging registry | date-sha | Low - Red Hat team manages |
| Users following old docs | `gcr.io/k8s-prow` | date-sha | Already broken (defunct) |
| Users on staging directly | `us-docker.pkg.dev/k8s-infra-prow/images/` | date-sha | Medium - need migration path |
| Users building from source | N/A | N/A | No impact |

### 4.2 What Changes for Consumers

**Registry prefix change:**
```
# Old (staging, deprecated)
us-docker.pkg.dev/k8s-infra-prow/images/<component>:<date-sha>
# New (promoted, supported)
registry.k8s.io/prow/<component>:v0.1.0
```

Component image names stay the same (`hook`, `deck`, `clonerefs`, etc.).

**Version format change:** `v20250503-abc1234` to `v0.1.0`
- `generic-autobumper` config needs updating (default registry + version detection)
- `VersionTimestamp()` in `pkg/version/doc.go` currently parses the date from
  `v${date}-${sha}` format and will error on semver. Callers need audit, function
  needs to handle semver gracefully (return build time from binary or zero value).

### 4.3 Migration Path

1. **Phase 1 is non-breaking:** Staging images continue to be built on every merge.
   The promoted images at `registry.k8s.io` are additive.

2. **Transition period:** Both staging (date-sha) and promoted (semver) images
   are available simultaneously. Staging continues indefinitely.

3. **Documentation updates (Phase 2):** All docs, starters, and examples switch
   to `registry.k8s.io/prow/`. This is when most consumers notice and migrate.

4. **No hard cutoff for staging:** Staging images keep being built (prow.k8s.io
   may continue consuming them as a canary). They're just not advertised or
   supported for external use.

### 4.4 Risks

- **Promotion latency:** Each release requires a PR to `kubernetes/k8s.io`. Time
  from tagging to images appearing at `registry.k8s.io` depends on review speed
  (hours to days).
- **Staging naming mismatch:** The `k8s-infra-prow` vs `k8s-staging-prow` naming
  difference needs resolution before the first release.
- **VersionTimestamp() breakage:** Code paths using this function will error on
  semver versions. Must be fixed before the first semver-tagged build is deployed.
- **43 images per release:** The promotion manifest will be large. The digest
  collection script must be reliable.

---

## 5. Release Communication Strategy

### 5.1 GitHub Releases (Primary)

GitHub Releases become the primary changelog. Structure:

```markdown
## What's Changed

### Breaking Changes
- Description (#PR)

### New Features  
- Description (#PR)

### Bug Fixes
- Description (#PR)

### Other Changes
- Description (#PR)

## Container Images
Images: `registry.k8s.io/prow/<component>:v0.2.0`

## Full Changelog
https://github.com/kubernetes-sigs/prow/compare/v0.1.0...v0.2.0
```

Use GitHub's auto-generated release notes as a starting point, grouped by PR
labels (`kind/bug`, `kind/feature`, `kind/api-change`, etc.).

### 5.2 Announcements Page

Keep `site/content/en/docs/announcements.md` for breaking changes and
deprecation notices that need long-term visibility. Add a "Releases" subsection
linking to GitHub Releases.

This page should NOT try to duplicate every release note — just document things
that operators need to be aware of when upgrading (config format changes,
removed flags, changed defaults, RBAC changes).

### 5.3 Communication Channels per Release

| Channel | When |
|---------|------|
| GitHub Release | Every release |
| Announcements page | Breaking changes or deprecations |
| Slack #prow, #sig-testing | Notable releases |
| kubernetes-dev@ email | Major milestones (v1.0), critical security fixes |

### 5.4 PR Labeling

Leverage existing Prow labels to auto-categorize release notes:
- `kind/bug` -> Bug Fixes
- `kind/feature` -> New Features
- `kind/api-change` -> Breaking Changes
- `kind/deprecation` -> Deprecations
- `kind/cleanup`, `kind/documentation` -> Other Changes

---

## 6. Immediate Next Steps

1. **Resolve staging naming** with sig-k8s-infra: Can the promoter pull from
   `us-docker.pkg.dev/k8s-infra-prow/images/` using a `k8s-staging-prow`
   manifest directory? File an issue or ask in #sig-k8s-infra Slack.
2. **Fix `VersionTimestamp()`** in `pkg/version/doc.go` to handle semver
3. **Write `hack/check-release-images.sh`** (based on Boskos template)
4. **Replace `RELEASE.md`** with the process documented above
5. **Create promotion directory PR** in `kubernetes/k8s.io`
6. **Tag `v0.1.0`** and file first promotion PR
7. **Create first GitHub Release**
