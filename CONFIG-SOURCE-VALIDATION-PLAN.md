# Config Source Validation Plan

Status: proposed design, no code changed

Repository: `kubernetes-sigs/prow`

Inspected revision: `358e07290` (merged default branch after PR #1010 and the
three followups: disjoint input paths, symlink-root rejection, independent
polling, and mixed-source sinker discovery).

## 1. Executive summary

The generic job-config reader currently enforces one global rule: every loaded
`*.yaml`/`*.yml` file must have a unique basename across all configured
`--job-config-path` roots. The rule is not a job-semantics rule. It exists
because `updateconfig` may publish a file into a ConfigMap under a key derived
from its basename, and two files with the same basename would then overwrite
one another.

The recommended split is:

* Generic loading keeps only input-integrity rules: configured paths must be
  disjoint, directory symlink roots are rejected, `.prowignore` and
  `..`-prefixed ConfigMap-mount files are handled as today. It does not know
  or care about ConfigMap keys.
* Publication gets a new, explicit validation layer that works on the actual
  file set and the actual `config_updater.maps` configuration. It rejects two
  distinct files that would write the same key into the same ConfigMap, while
  allowing identical basenames whenever the files target different
  ConfigMaps, different namespaces/clusters, or different keys.
* Job semantics remain enforced exactly where they are today, after all
  sources are merged: duplicate presets, duplicate presubmit/postsubmit jobs
  per repo/branch, duplicate reporting contexts, and duplicate periodics.

The rollout must be staged: land the validators in `checkconfig`,
`config-bootstrapper`, and the `updateconfig` update path first, keep the
runtime basename check for one release, and only then remove the check from
`ReadJobConfigs`. No new CLI flag or policy option is required.

Non-goals for the bounded implementation: deleting the basename check on its
own, replacing it with a per-path basename check, validating only changed
files, changing sinker metrics, or adding a runtime policy flag. This document
is a plan only; it does not change code or accepted configurations.

## 2. Confirmed current behavior

This section describes the tree as inspected, not the proposal.

### 2.1 Runtime job-config loading

`pkg/config/config.go`:

* `ReadJobConfigs` (`:1714`) delegates to `readJobConfigs` (`:1719`).
* `readJobConfigs` creates `jobConfigReader{uniqueBasenames: map[string]string{}}`
  (`:1724-1730`). The comment explicitly says the reason is that the
  `updateconfig` plugin uses the basename as a ConfigMap key.
* `jobConfigReader.readFile` (`:1900`) computes `filepath.Base(path)` and
  rejects a file if that basename was already loaded from any other path
  (`:1901-1907`). The error is `duplicated basename is not allowed: ...`.
* The check is global across every configured job-config path, not per path,
  even though PR #1010 made the paths independent sources.
* `validateJobConfigPaths` (`:1754`) separately rejects repeated or
  overlapping configured paths. This is an input-integrity rule: without it,
  the same file could be merged twice.
* `jobConfigReader.readPath` (`:1829`) rejects a configured path that is a
  symlink to a directory (`:1840-1850`), because `filepath.Walk` does not
  follow a symlink root. It also creates a per-root `.prowignore` parser.
* `jobConfigReader.visit` (`:1866`) skips `..`-prefixed ConfigMap-mount
  internals, non-YAML files, and `.prowignore`-ignored files.

Every component that uses `config.Agent`/`config.Load` inherits the basename
check, including plank, hook, sinker, horologium, tide, and `checkconfig`.

### 2.2 Job-semantics checks

These checks already operate on the merged result and therefore already work
across independent sources. They must remain untouched:

* `mergeJobConfigs` (`pkg/config/config.go:2153`) rejects a duplicate preset
  `label:value` pair (`:2159-2168`).
* `validatePresubmits` (`:2442`) rejects duplicate presubmit names whose
  branchers intersect on the same repo, and rejects two jobs reporting to the
  same GitHub context (`:2447-2475`).
* `validatePostsubmits` (`:2504`) does the same for postsubmits
  (`:2510-2538`).
* `validatePeriodics` (`:2545`) rejects duplicate periodic names globally
  (`:2547-2553`).
* `ValidateJobConfig` (`:2618`) is called from `loadWithYamlOpts` after
  `finalizeJobConfig` (`:1684-1690`).

Consequence: removing the basename check does not relax any duplicate-job
guarantee. Two sources with identical basenames that define the same job still
fail through `ValidateJobConfig`; two sources that define disjoint jobs still
load.

Summary of which check protects what:

| Check | Concern | Proposed disposition |
| --- | --- | --- |
| global basename uniqueness (`readFile`) | publication to a ConfigMap | remove from the reader; move to `ValidatePublication` |
| disjoint configured paths (`validateJobConfigPaths`) | input integrity/job semantics | keep unchanged |
| symlink-to-directory root rejection (`readPath`) | input integrity | keep unchanged |
| duplicate preset `label:value` (`mergeJobConfigs`) | job semantics | keep unchanged |
| duplicate presubmit/postsubmit names and contexts (`validatePresubmits`/`validatePostsubmits`) | job semantics | keep unchanged |
| duplicate periodic names (`validatePeriodics`) | job semantics | keep unchanged |
| explicit-key collision in `validateConfigUpdater` | publication | keep at config load; refine in Phase 3 |
| key derivation in `FilterChanges` | publication | centralize in shared helpers |
| duplicate keys inside one `Update` batch | publication/publishing atomicity | add defense-in-depth guard |
| stale-key deletion in bootstrap (`MarkStaleKeysForDeletion`) | publication | keep; require a complete source set |

### 2.3 `checkconfig`

`cmd/checkconfig/main.go`:

* `validate` (`:254`) loads the Prow config through `o.config.ConfigAgent()`
  (`:261`), so the runtime basename check runs here.
* It loads the plugin config through `o.pluginsConfig.PluginAgent()` (`:268`).
* The only `config-updater`-specific validation is the ACL warning at
  `:444-448`, implemented by `validateConfigUpdaterACLs` (`:1589`). That
  function checks ACLs, not key collisions.
* There is no walk of the whole checkout for publication purposes. A
  `config-updater` collision involving an unchanged file is therefore missed
  unless the runtime basename check happens to catch it.

`pkg/flagutil/config/config.go:60` also documents the global basename rule in
the `--job-config-path` flag help. `site/content/en/docs/scaling.md:65-78`
documents it as user-facing behavior.

### 2.4 `pkg/plugins/updateconfig`

`pkg/plugins/updateconfig/updateconfig.go`:

* `FilterChanges` (`:284`) finds the first matching `cfg.Maps` entry by
  iterating a Go map (`:294-307`), so when multiple globs match one file, the
  winner is nondeterministic.
* Key derivation is inline (`:341-355`):
  * explicit `spec.Key` wins;
  * otherwise `UseFullPathAsKey` replaces `/` with `-` in the repo-relative
    filename;
  * otherwise `path.Base(filename)` is used.
* Partition selection is `sha256(key)[0] % len(PartitionedNames)` (`:329-339`).
  Keys are therefore stable for a fixed ordered partition list.
* `handle` (`:401`) receives only the PR's changed files from
  `GetPullRequestChanges` (`:421`) and passes them to `FilterChanges`
  (`:440`). It never sees unchanged files that are already published.
* `Update` (`:104`) applies updates sequentially and assigns
  `cm.Data[upd.Key]`/`cm.BinaryData[upd.Key]` (`:194-198`). Two updates for
  the same key silently last-write-win.
* `MarkStaleKeysForDeletion` (`:232`) deletes ConfigMap keys not present in the
  bootstrap update set. It assumes the caller supplied the complete set for
  that ConfigMap.

`pkg/plugins/config.go` has a second, partial key check:

* `validateConfigUpdater` (`:1490`) runs at plugin-config load time. It calls
  `SetDefaults` (`:1259`), then for each map entry computes
  `key = config.Key`, or `path.Base(file)` where `file` is the *glob pattern*
  (`:1515-1518`), and rejects the same key in the same ConfigMap ID
  (`:1520-1526`).
* That check ignores `UseFullPathAsKey`, cannot see actual matched files, and
  uses the glob text as a pretend filename. For example, two patterns ending
  in `*.yaml` can be rejected even when their real files derive different
  keys, while one broad glob matching two `jobs.yaml` files is not rejected.
* For `PartitionedNames`, `validateConfigUpdater` groups entries under the
  joined partition-list string (`:1498-1501`) and cannot know which partition
  a key will select, so it cannot validate the actual partition targets.
* `ConfigUpdater.resolve` (`:779`) expands `cluster_groups`. Maps that have an
  explicit `clusters` list (or the default added by `SetDefaults`) are skipped
  (`:785-787`); maps using `cluster_groups` are rebuilt with only
  `Name`, `PartitionedNames`, `Key`, `GZIP`, and `Clusters` (`:797-802`),
  dropping `UseFullPathAsKey`, `AllowedRepos`, and `DeniedRepos`.
  This was verified with a local probe: `cluster_groups` plus
  `use_full_path_as_key: true` loads with `UseFullPathAsKey == false`.
* `validateConfigUpdater` iterates `config.Clusters`; for a not-yet-resolved
  `cluster_groups` map that map is empty, so no key-collision check runs for
  those entries at all.
* `validateConfigUpdater` does not normalize an empty namespace to the default
  namespace. Runtime `handleDefaultNamespace` (`updateconfig.go:390`) does.
  Two specs that differ only by empty-vs-explicit default namespace are
  therefore not caught at config load but collide at runtime.

### 2.5 `config-bootstrapper`

`cmd/config-bootstrapper/main.go`:

* `main` (`:169`) loads the config agent (`:179`) and plugin agent (`:184`),
  so the runtime basename rule applies only if `--job-config-path` was passed
  to the config agent. The publication path itself does not call
  `ReadJobConfigs`.
* `run` (`:106`) walks every `--source-path` root (`:121-141`) and pretends
  every file is newly added, producing a complete change set for the roots it
  was given.
* It passes the change set to `FilterChanges` (`:149`) and then writes each
  ConfigMap with `Update` (`:159`). There is no preflight collision check.
* `osFileGetter.GetFile` (`:90`) searches the roots in order and returns the
  first match. Its comment says two roots containing the same relative path
  are assumed not to happen because that configuration would be racy and
  unsupported.

So `config-bootstrapper` is the one publish path that already has the complete
input set (all files under all source roots), but it does not use that
completeness to validate key uniqueness.

### 2.6 Sinker

`cmd/sinker/main.go` has a size monitor (`jobConfigMapMonitor` at `:581`,
`getConfigMapDirs` at `:610`, `getConfigMapSize` at `:680`). It discovers
ConfigMap mounts among mixed sources and reports sizes per mount. It performs
no publication validation. It is intentionally out of scope for this plan and
must not be redesigned.

### 2.7 Confirmed sharp edges relevant to the change

These are current defects or ambiguities, not consequences of the proposal:

1. The config-load key check uses glob text, not files, and ignores
   `UseFullPathAsKey`.
2. `cluster_groups` maps silently lose `UseFullPathAsKey` and ACL fields during
   `resolve`.
3. `FilterChanges` chooses among multiple matching map entries
   nondeterministically.
4. Empty and explicit default namespaces are treated as different at config
   load but the same at runtime.
5. Duplicate keys in one `Update` call last-write-win.
6. Two `--source-path` roots containing the same relative path are documented
   as unsupported but are not rejected.

## 3. Proposed model

### 3.1 Publication domain and safety invariant

The unit of safety is a publication target, not a filesystem path:

```text
PublicationTarget = (ConfigMapID, Key)
ConfigMapID       = (ConfigMap name, namespace, cluster)
```

For partitioned ConfigMaps, the ConfigMap name is the partition selected by
the key. The namespace is normalized through the same default-namespace rule
as `handleDefaultNamespace` (`pkg/plugins/updateconfig/updateconfig.go:390`).

Invariant:

```text
For every PublicationTarget, at most one source file may be a writer.
```

A source file is identified by its source root plus its map-relative path, not
by its content. Two identical contents in two files are still two writers and
must be rejected if they target the same key.

### 3.2 What "independent sources" means

Two `--job-config-path` roots are independent for *loading*: both are merged
into one job config and neither has to know about the other's filenames.

They are independent for *publication* only when their files map to different
`PublicationTarget`s. This is not automatically true just because the runtime
paths are different:

* `/etc/job-config-a` mounted from ConfigMap `job-config-a` and
  `/etc/job-config-b` mounted from ConfigMap `job-config-b` may both contain
  `jobs.yaml`. Both derive key `jobs.yaml`, but the ConfigMap names differ, so
  the targets differ: safe.
* `config/jobs-a/**` and `config/jobs-b/**` both mapped to ConfigMap
  `job-config` with basename keys derive the same target `job-config/jobs.yaml`:
  unsafe, even if configured as two runtime paths.
* The same two directories mapped with `use_full_path_as_key: true` derive
  `config-jobs-a-jobs.yaml` and `config-jobs-b-jobs.yaml`: safe.
* Explicit `key: jobs.yaml` on both patterns always derives the same target:
  unsafe regardless of directories or `UseFullPathAsKey`.
* Partitioned ConfigMaps do not create a new namespace per file. Two files
  with the same derived key hash to the same partition and collide; two
  different keys that happen to hash to the same partition do not collide.

The plan therefore validates the mapping, not the configured path boundary.
In particular, replacing the global basename check with a per-path basename
check would still be wrong: one configured path can be a partition holding
several ConfigMap mounts (the layout sinker's `getConfigMapDirs` discovers),
where the same basename in different mounts is safe.

### 3.3 Single key-derivation implementation

One helper must be the only place that derives keys. Proposed API, either in
`pkg/plugins/config.go` as methods on `ConfigMapSpec` or in a small new
`pkg/plugins/updateconfig` publication file:

```go
// KeyForFile returns the ConfigMap key for a map-relative slash path.
func (s ConfigMapSpec) KeyForFile(filename string) string {
    if s.Key != "" {
        return s.Key
    }
    if s.UseFullPathAsKey {
        return strings.ReplaceAll(filename, "/", "-")
    }
    return path.Base(filename)
}

// PartitionFor returns the selected partition name, or s.Name when not
// partitioned.
func (s ConfigMapSpec) PartitionFor(key string) string

// Targets returns every ConfigMapID a file writes to after cluster,
// namespace-default, and partition expansion.
func (s ConfigMapSpec) Targets(key, defaultNamespace string) []ConfigMapID
```

`ConfigMapSpec.Targets` must apply the default namespace to an empty
namespace before constructing `ConfigMapID`.

Both `FilterChanges` and every validator must call these helpers. This removes
the current drift between `validateConfigUpdater`, `FilterChanges`, and future
callers.

### 3.4 Publication validator

Proposed pure function in `pkg/plugins/updateconfig`:

```go
type SourceFile struct {
    Root string // source root or repo root, used for diagnostics and duplicate detection
    Path string // slash-separated path matched by config_updater.maps
}

// ValidatePublication rejects two distinct source files that write the same
// key to the same ConfigMapID.
func ValidatePublication(cfg plugins.ConfigUpdater, files []SourceFile, defaultNamespace string) error
```

Algorithm:

1. Reject duplicate `(Root, Path)` records. In `config-bootstrapper`, also
   reject the same `Path` appearing under two different roots because
   `osFileGetter` would resolve it ambiguously.
2. For each file, find the matching `config_updater.maps` entry with a
   deterministic matcher. If no entry matches, skip the file.
3. If more than one entry matches:
   * if all matches derive the same `(ConfigMapID, Key)` for that file, use it;
   * otherwise return an ambiguity error naming the file and the competing
     patterns. Do not silently pick a random one.
4. Derive `key` with `KeyForFile`.
5. Expand targets with `Targets` (clusters, namespaces, partition).
6. Record `target -> file`. If the target already has a different file,
   return an error containing the key, ConfigMap name/namespace/cluster, and
   both source files.

The validator must not read file contents. This is a namespace-collision
check, not a content comparison.

### 3.5 Where complete validation can run

| Component | File set it can see | Completeness |
| --- | --- | --- |
| `pkg/config.ReadJobConfigs` | all configured job paths | sees only job files; cannot know ConfigMap specs, partitions, or non-job published files |
| `updateconfig` plugin `handle` | one PR's changed files in one repo | incomplete by construction; cannot see unchanged published keys or other repos |
| `checkconfig` | whole checkout plus plugin config | complete for the publication sources present in that checkout; ideal CI gate |
| `config-bootstrapper` | all files under all `--source-path` roots plus plugin config | complete for every source root passed to that invocation; ideal pre-write gate |

`checkconfig` is the CI safeguard. It must walk the complete checkout, not a
diff, and must run after the plugin config is loaded. `config-bootstrapper` is
the publication safeguard: it must validate the complete change set before
the first `Update` call, so a failure leaves every ConfigMap untouched.

The `updateconfig` merge handler remains a defense-in-depth check only. It can
reject collisions inside one PR's change set, but it cannot prove global
uniqueness. Complete plugin-time validation across repositories that share a
ConfigMap would require an ownership/provenance record; that is an explicit
open question in section 8, not part of the bounded implementation.

## 4. Exact implementation plan

### Phase 0: preparation, no behavior relaxation

1. `pkg/plugins/config.go`
   * Add `ConfigMapSpec.KeyForFile`, `PartitionFor`, and `Targets` (or the
     equivalent shared helpers).
   * Normalize namespaces only in the publication validator, because
     `validateConfigUpdater` has no default namespace.

2. `pkg/plugins/updateconfig/updateconfig.go`
   * Refactor the inline key and partition derivation in `FilterChanges`
     (`:284`) to call the shared helpers. This must be behavior-preserving:
     keep ACL, rename, delete, and bootstrap partition bookkeeping unchanged.
   * Add `ValidatePublication` and its unit tests. Do not call it from a
     production path yet.

3. `pkg/config/config.go`
   * Do not remove `uniqueBasenames` yet.
   * Add the new validator package/function and unit tests first, so the
     release contains the safeguards while still rejecting duplicate
     basenames.

At the end of Phase 0 the only new executable behavior is the exported helper
code and its tests. The existing runtime basename check and the coarse
`validateConfigUpdater` check are both still in force.

### Phase 1: CI and publishing safeguards

1. `pkg/plugins/config.go`
   * Fix `ConfigUpdater.resolve` (`:779`) to preserve
     `UseFullPathAsKey`, `AllowedRepos`, and `DeniedRepos` when rebuilding a
     `cluster_groups` map. Without this, validation would use different key
     semantics than the user configured, and runtime already has the bug.
   * Add the deterministic `MatchConfigMapSpec` matcher used by
     `ValidatePublication`. It sorts map keys before choosing among multiple
     matches and returns an ambiguity error when matching specs disagree.
     `FilterChanges` can adopt it in this phase; the old random first-match
     behavior is not worth preserving.
   * Leave the coarse `path.Base(pattern)` check in `validateConfigUpdater`
     (`:1490`) in place for now. It is conservative and still protects config
     load while the new validator is being rolled out. It will be removed in
     Phase 3 together with the runtime basename check.

2. `cmd/checkconfig/main.go`
   * Add `validateConfigMapPublication(pcfg *plugins.Configuration, defaultNamespace string, fsys fs.FS) error`.
     It walks the checkout (the existing supplemental-config validation already
     uses `fs.WalkDir`, so this is consistent), collects every file matched by
     `ConfigUpdater.Maps`, and calls `ValidatePublication`. Skip `.git` and
     other non-source directories; do not apply `.prowignore`, because
     publication is independent of what the job loader ignores.
   * Call it from `validate` (`:254`) after `pcfg` is loaded (`:268`) and before
     the warning aggregate is returned.
   * Make a collision a hard error, not a warning. A collision silently
     corrupts a published ConfigMap and cannot be repaired at runtime, so it
     has the same severity as a config-load error. If maintainers insist on
     the warning mechanism, it must be enabled by default and CI must run
     `--strict`; that is weaker than a hard error.
     Implementation note: `validate` currently returns
     `utilerrors.Aggregate` and `main` reports aggregates through
     `reportWarning`. A hard publication error must be returned directly as a
     non-aggregate error (or `main` must distinguish fatal validation errors),
     otherwise it would only produce a warning unless `--strict` is set.
   * Keep the existing ACL warning separate.

3. `cmd/config-bootstrapper/main.go`
   * In `run` (`:106`), after collecting all `changes` and before the
     `FilterChanges` loop (`:149`), call `ValidatePublication` with the
     `(root, relative path)` records.
   * Reject the same relative path under two roots explicitly, replacing the
     `osFileGetter` comment's unsupported assumption with an error.
   * Return a non-zero count without calling `Update` for any ConfigMap if
     validation fails. This makes the preflight atomic.
   * Keep `--dry-run` default behavior; validation must run in dry-run too.

4. Documentation
   * `pkg/flagutil/config/config.go:60`: remove the "basenames must be unique"
     sentence and explain that publication keys are validated by
     `checkconfig`/`config-bootstrapper`; independent paths may reuse
     basenames when they target different ConfigMap keys. Until Phase 3,
     state explicitly that runtime still rejects duplicate basenames.
   * `site/content/en/docs/scaling.md:65-78`: replace the global basename
     requirement with the publication-target rule and examples.
   * `site/content/en/docs/components/cli-tools/checkconfig.md`: state that
     `checkconfig` validates the whole checkout, not only changed files, and
     requires the plugin config for publication checks.
   * `site/content/en/docs/components/cli-tools/config-bootstrapper.md`: state
     that all source roots feeding a ConfigMap must be passed to one
     invocation, and collisions abort before any write.
   * `site/content/en/docs/components/plugins/updateconfig.md`: document that
     a glob with an explicit `key` must match at most one file, and that
     `use_full_path_as_key` is the supported way to reuse basenames in one
     ConfigMap.

5. Release note / migration note: duplicate basenames are still rejected at
   runtime in this release, but the publication validator is now authoritative
   and will name both conflicting files.

### Phase 2: updateconfig defense-in-depth

1. `pkg/plugins/updateconfig/updateconfig.go`
   * Add a small `validateUpdates(updates []ConfigMapUpdate) error` or perform
     the same check at the start of `Update` (`:104`): reject two non-empty
     `Filename` entries for the same `Key` in one ConfigMap update set before
     mutating the object.
   * In `handle` (`:401`), call `ValidatePublication` on the PR's changed
     files after `FilterChanges` and before `Update`. This is incomplete
     globally, but it prevents an obvious same-PR collision from producing a
     partial write and a misleading success comment.
   * Do not change `MarkStaleKeysForDeletion` semantics. Bootstrap callers are
     now required to pass the complete source set.

2. Test that a failed validation produces no ConfigMap mutation and no
   "updated" comment in the plugin handler.

### Phase 3: relax runtime loading

Only after Phase 1 and Phase 2 are released and deployed:

1. `pkg/config/config.go`
   * Remove `uniqueBasenames` from `jobConfigReader` (`:1816-1827`).
   * Remove the basename check and error from `readFile` (`:1900-1907`).
   * Update the `ReadJobConfigs`/`readJobConfigs` doc comments (`:1700-1730`)
     so they no longer claim basename uniqueness.
   * Keep `validateJobConfigPaths`, the symlink-root rejection, `.prowignore`
     handling, `..` skipping, and all error aggregation exactly as they are.
2. `pkg/plugins/config.go`
   * Remove the coarse `path.Base(pattern)` inference from
     `validateConfigUpdater` (`:1490`). Keep its structural checks and its
     explicit-`Key` collision check; actual implicit-key collisions are now
     covered by `ValidatePublication`.
3. Update the flag help, site docs, and release note from "runtime rejects
   duplicate basenames" to "publication validates key collisions".
4. Update tests described in section 6.

No new feature flag is recommended. The release boundary is the switch: the
new checkconfig/bootstrapper/updateconfig safeguards are in release N, and the
runtime check is removed in release N+1. A flag would create a state where an
operator can relax the constraint before the safeguards are present.

## 5. Compatibility considerations

* Existing configurations with globally unique basenames load exactly as
  today; Phase 3 changes nothing for them.
* Configurations that currently fail only because independent sources reuse a
  basename will start loading in Phase 3. They are accepted only if
  `checkconfig`/`config-bootstrapper` prove distinct publication targets.
* Configurations that reuse a basename and target the same ConfigMap key
  remain rejected, but earlier and with a better error naming both files.
  They were already rejected by the runtime basename check. Some other
  configurations were accepted before but would silently collide at publish
  time (for example two explicit keys that differ only by empty versus
  explicit default namespace); those now fail earlier in `checkconfig`. They
  were never safely publishable.
* `config_updater` configs that currently fail `validateConfigUpdater` only
  because two glob patterns share a basename may now load under Phase 3. If
  their actual files collide, Phase 1 rejects them; if they use
  `use_full_path_as_key` or separate ConfigMaps, they become valid. This is an
  intentional improvement, but it is a config-load behavior change and must be
  called out in release notes.
* `cluster_groups` plus `use_full_path_as_key` currently silently loses the
  full-path setting. Phase 1's `resolve` fix makes the configured behavior
  real; this is a bug fix that may change published keys for users who
  believed the setting was active. Document it and test it.
* Ambiguous map matches are currently nondeterministic. Phase 1's matcher
  either makes them deterministic or rejects them. If the project prefers to
  keep permissive first-match behavior, the validator must still report the
  ambiguity, and the decision should be recorded as an open question. The
  recommended behavior is to reject specs that disagree.
* Empty namespace versus explicit default namespace is now a collision.
  Configurations that already collide at runtime will start failing
  `checkconfig`; they were never safe.
* Partition list order remains significant. The validator uses the same
  hash-and-index algorithm as the publisher, so it does not change key
  placement. Reordering `PartitionedNames` still remaps keys, as today.
* `config-bootstrapper` must be invoked with every root that publishes into a
  ConfigMap it bootstraps. Passing roots separately would make each invocation
  treat the other roots' keys as stale and delete them; the new preflight
  makes the same-relative-path ambiguity explicit but does not change stale
  deletion semantics.
* `checkconfig` now walks more files; this is bounded by the checkout size and
  the map globs. If it becomes expensive on very large repositories, an
  optimization is to walk only directories that are prefixes of map patterns,
  or to use a Git file list. That optimization is not required for the first
  implementation.

## 6. Focused test matrix

### 6.1 `pkg/config` generic reader

| Case | Expected |
| --- | --- |
| Same basename in two different configured paths | loads; both job sets merged (proposed Phase 3) |
| Same basename in two subdirectories of one path | loads (proposed Phase 3) |
| Same basename in a file path and a directory path | loads if both are valid and disjoint (proposed Phase 3) |
| Overlapping or repeated configured paths | still rejected by `validateJobConfigPaths` |
| Symlink-to-directory root | still rejected by `readPath` |
| Same job name in two sources with the same basename | rejected by `ValidateJobConfig` duplicate job check |
| Same preset `label:value` in two sources | rejected by `mergeJobConfigs` |
| Duplicate periodic name across sources | rejected by `validatePeriodics` |
| Presubmits with same name but disjoint branches | accepted, as today |

Existing tests to update or invert:

* `TestReadJobConfigsMultiplePaths` cases `"same basename in different paths is
  rejected"` and `"same basename in different directories of one path is
  rejected"` (`pkg/config/config_test.go:3846`, `:3854`).
* `TestReadJobConfigsFileAndDirectory` case `"the basename of a file is unique
  across the paths too"` (`:3932`).

### 6.2 Shared key derivation

| Case | Expected |
| --- | --- |
| `jobs.yaml` with no options | key `jobs.yaml` |
| `a/b/jobs.yaml` with `use_full_path_as_key: true` | key `a-b-jobs.yaml` |
| `a-b.yaml` and `a/b.yaml` with full-path keys | both key `a-b.yaml`, rejected as a collision |
| explicit `key: jobs.yaml` | key `jobs.yaml` regardless of path |
| explicit key with a glob matching two files | rejected |
| partitioned names with the same key | same partition, rejected |
| partitioned names with different keys that hash to the same partition | accepted; different keys |
| same key in different namespaces | accepted |
| empty namespace versus explicit default namespace | rejected as the same target |
| same key in different clusters | accepted |

### 6.3 Publication validator call sites

| Case | Expected |
| --- | --- |
| Two files, same basename, different ConfigMaps | accepted |
| Two files, same basename, same ConfigMap | rejected with both file paths and target |
| Two files, same basename, same ConfigMap, full-path keys | accepted |
| Two files, explicit same key | rejected |
| One glob matching multiple files with an explicit key | rejected |
| Glob matched by two `maps` entries with conflicting specs | rejected as ambiguous, or deterministic if the project chooses that behavior |
| `checkconfig` with a new file colliding with an unchanged existing file | rejected; proves validation is full-tree, not changed-files-only |
| `config-bootstrapper` with two roots containing the same relative path | rejected before any `Update` |
| `config-bootstrapper` with a collision in the complete root set | rejected before any `Update` |
| `config-bootstrapper` with distinct keys in one ConfigMap across roots | accepted |
| `updateconfig.Update` with two updates for the same key | rejected before mutating the ConfigMap |
| `updateconfig.handle` with a same-PR collision | no ConfigMap mutation, no success comment |

### 6.4 Existing behavior regression tests

* `TestValidateConfigUpdater`: explicit-key collision still rejected; structural
  checks unchanged; add cases for implicit keys deferred to publication and
  for `UseFullPathAsKey`.
* `TestConfigUpdaterResolve`: add a `cluster_groups` case asserting
  `UseFullPathAsKey`, `AllowedRepos`, and `DeniedRepos` survive resolution.
* `TestFilterChangesWithACLs`: unchanged ACL behavior.
* `TestUpdateConfigV2` and partition tests: unchanged publishing behavior.
* `TestReadJobConfigsRejectsNonDisjointPaths` and `TestReadJobConfigsSymlinkRoots`:
  unchanged.

## 7. Verification

For the implementation (not this plan-only deliverable), the focused commands
are:

```sh
GOEXPERIMENT=jsonv2 go test ./pkg/config
GOEXPERIMENT=jsonv2 go test ./pkg/plugins/...
GOEXPERIMENT=jsonv2 go test ./cmd/checkconfig
GOEXPERIMENT=jsonv2 go test ./cmd/config-bootstrapper
GOEXPERIMENT=jsonv2 go build ./...
```

On the inspected revision, the focused suites pass except for three
`pkg/plugins/updateconfig` gzip fixture assertions whose expected compressed
bytes differ from the local Go toolchain's gzip output. That failure is
unrelated to this plan; the non-gzip `updateconfig` behavior and the other
listed packages pass.

## 8. Open questions

1. **Cross-repository shared ConfigMaps.** If two separate repositories each
   publish `jobs.yaml` into the same ConfigMap, no single `checkconfig`
   checkout sees both, and the merge-time plugin sees only changed files. The
   bounded plan covers a single checkout and a single `config-bootstrapper`
   invocation with all roots. Making plugin-time validation complete across
   repositories requires a provenance/ownership record (for example, an
   annotation mapping key -> source repo/path) or a central validation job
   that checks out all sources. Which of those is acceptable?
2. **Ambiguous map matches.** Should `FilterChanges` reject a file matched by
   multiple `maps` entries with different specs, or keep a deterministic
   first-match rule? The recommendation is to reject, because today the
   winner is random.
3. **`resolve` field-loss fix.** The plan treats preserving
   `UseFullPathAsKey`/ACLs for `cluster_groups` as a prerequisite because
   validation and runtime otherwise disagree with the user's config. If the
   maintainers consider that a separate bug, the alternative is to document
   `cluster_groups` + `use_full_path_as_key` as unsupported and validate the
   post-resolve values, but that leaves the runtime bug in place.
4. **Error severity in `checkconfig`.** Hard error versus default warning plus
   `--strict` in CI. The recommendation is a hard error because a collision
   silently corrupts published configuration.
5. **ACL filtering in validation.** Should `checkconfig` validate files that
   the current repo's ACLs would deny? The recommendation is to validate all
   files matched by the maps (the map declares publication intent), and leave
   ACL filtering to runtime. This may reject a benign denied-file overlap; the
   alternative requires passing the target repo identity into `checkconfig`.
6. **Rename/delete interactions.** The first validator covers file-to-key
   mapping for existing files. A rename or deletion in one source can still
   delete a key that another source owns unless the update batch is examined
   as a sequence. Is that included in the updateconfig batch guard, or left as
   a follow-up?

## 9. Acceptance-criteria mapping

| Criterion | How the plan satisfies it |
| --- | --- |
| Independent sources can reuse basenames safely | The invariant is per `(ConfigMapID, key)`, not per basename or filesystem path. Section 3.2 gives the safe and unsafe cases; `ValidatePublication` accepts duplicate basenames when targets differ. |
| Distinct files targeting the same ConfigMap key are rejected | Shared `KeyForFile`/`Targets` helpers plus `ValidatePublication` group by target and reject multiple writers. It runs in `checkconfig` (full checkout), `config-bootstrapper` (all roots, before writes), and `updateconfig` (batch guard). |
| Duplicate-job and overlapping-input checks remain enforced | `validateJobConfigPaths` and `jobConfigReader.readPath` stay untouched. `mergeJobConfigs`, `validatePresubmits`, `validatePostsubmits`, `validatePeriodics`, and `ValidateJobConfig` stay untouched and run after merging all sources. |
| CI/publishing safeguards land before runtime relaxation | Phase 0 adds helpers and tests; Phase 1 adds full-tree `checkconfig` and pre-write `config-bootstrapper` validation; Phase 2 adds the updateconfig guard; Phase 3 removes `uniqueBasenames` only in the next release, with no feature flag that can relax early. |

