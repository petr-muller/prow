---
title: "Plugins"
weight: 70
description: >
  
---

Plugins are sub-components of [`hook`](/docs/components/core/hook/) that consume [GitHub webhooks](https://developer.github.com/webhooks/) related to their function and can be individually enabled per repo or org.

Plugin configuration is loaded from a main file, usually [`plugins.yaml`](https://github.com/kubernetes/test-infra/blob/master/config/prow/plugins.yaml), selected with `--plugin-config`. A supported subset can also be loaded from supplemental files as described below.
The `Configuration` golang struct holds all the config fields organized into substructures by plugin. See its [GoDoc](https://godoc.org/sigs.k8s.io/prow/pkg/plugins#Configuration) for up-to-date descriptions of every config option.

## Help Information

Most plugins lack README's but instead generate `PluginHelp` structs on demand that include general explanations and help information in addition to details about the current configuration.

Please see <https://prow.k8s.io/plugins> for a list of all plugins deployed on the Kubernetes Prow instance, what they do, and what commands they offer.
For an alternate view, please see <https://prow.k8s.io/command-help> to see all of the commands offered by the deployed plugins.

## How to enable a plugin on a repo

Add an entry to [plugins.yaml](https://github.com/kubernetes/test-infra/blob/master/config/prow/plugins.yaml). If you misspell the name then a
unit test will fail. If you have [updateconfig](/docs/components/plugins/updateconfig/) plugin
deployed then the config will be automatically updated once the PR is merged,
else you will need to run `make update-plugins`. This does not require
redeploying the binaries, and will take effect within a minute.

## Supplemental plugin configuration

Hook can load supplemental configuration from a directory with
`--supplemental-plugin-config-dir=/etc/prow/plugin-configs`. This flag can be
repeated to load multiple directories. The loader recursively reads files whose
names end in `_pluginconfig.yaml`, such as `widget_pluginconfig.yaml`. Override
the suffix with `--supplemental-plugin-configs-filename-suffix`; an empty suffix
disables supplemental loading. The main configuration is loaded first, then
supplemental files are merged and the combined configuration is validated.

Supplemental files use the same YAML structure as the main configuration, but
only the following fields and subfields support merging:

| Field | Supported configuration and merge behavior |
| --- | --- |
| `plugins` | Organization or repository entries, including `plugins` and `excluded_repos`. Each key must occur in only one file. |
| `external_plugins` | Organization or repository entries with plugin `name`, `endpoint`, and `events`. Each key must occur in only one file. |
| `approve`, `lgtm`, `triggers`, `welcome` | Complete configuration entries, appended to the corresponding lists. The combined configuration must pass each plugin's validation. |
| `bugzilla` | `default`, `orgs.<org>.default`, and `orgs.<org>.repos.<repo>.branches`, including their branch options. Each global default, organization default, or repository configuration must occur in only one file. |
| `label.restricted_labels` | Restricted label entries. The same label cannot be configured more than once for the same scope key across files. |
| `milestone_applier` | Repository keys (`org/repo`) mapped to branch names and milestone titles. Each repository key must occur in only one file. |

Other configuration fields must stay in the main file. In particular,
`label.additional_labels`, `repo_milestone`, and `config_updater` are not
supported in supplemental files. See the `Configuration`
[GoDoc](https://godoc.org/sigs.k8s.io/prow/pkg/plugins#Configuration) for the
options within each supported field.

For example, `widget_pluginconfig.yaml` can contain:

```yaml
milestone_applier:
  acme/widget:
    main: "v2.0"
    release-1.9: "v1.9"
```

The milestone titles must exist in `acme/widget`. Enable the `milestoneapplier`
plugin for that repository or its organization using the `plugins` configuration
as described in [How to enable a plugin on a repo](#how-to-enable-a-plugin-on-a-repo).
Providing `milestone_applier` settings alone does not enable the plugin.

### Migrating a repository

For `milestone_applier`, duplicate detection applies to the entire repository
key across the main file and all supplemental files. For example, defining
`acme/widget` with only `main` in `plugins.yaml` and only `release-1.9` in a
supplemental file is rejected, even though the branch maps are disjoint. The
loader does not merge branches for the same repository or let a later file
override an earlier one.

Move the complete `milestone_applier.acme/widget` branch-to-milestone mapping
into one supplemental file and remove that repository entry from the main file
and any other supplemental files. Deliver these changes together so Hook sees
a configuration with exactly one mapping for the repository. Repository keys
in different fields are independent: its `plugins` entry can remain in the main
file while its `milestone_applier` entry moves to a supplemental file.

### Delivering supplemental files to Hook

The directory flag refers to a path inside the Hook container. Arrange for the
supplemental files to reach that directory, for example by updating a ConfigMap
and mounting it at `/etc/prow/plugin-configs`. Each mounted file name must match
the configured suffix. Committing a file to a repository does not by itself
make it available to Hook.

The [config-updater documentation](/docs/components/plugins/updateconfig/)
explains how to map repository files, including glob patterns, to ConfigMap
keys. Keep those `config_updater` settings in the main configuration and ensure
the target ConfigMap is mounted into Hook's supplemental directory. Hook
reloads plugin configuration once a minute. If a reload fails validation or
merging, Hook keeps the previous configuration; an invalid initial load fails
startup.

## External Plugins

External plugins offer an alternative to compiling a plugin into the `hook` binary. Any web endpoint that can properly handle GitHub webhooks can be configured as an external plugin that `hook` will forward webhooks to. External plugin endpoints are specified per org or org/repo in [`plugins.yaml`](https://github.com/kubernetes/test-infra/blob/master/config/prow/plugins.yaml) under the `external_plugins` field. Specific event types may be optionally specified to filter which events are forwarded to the endpoint.
External plugins are well suited for:

- Slow operations that would impact the performance of other plugins if run as part of `hook`.
- Components that need to be triggered or notified of events beside GitHub webhooks.
- Isolating a more or less privileged plugin or a plugin that executes PR code.
- Integrating existing GitHub services with Prow.

Examples of external plugins can be found in the [`prow/external-plugins`](https://github.com/kubernetes-sigs/prow/tree/main/cmd/external-plugins) directory. The following is an example external plugin configuration that would live in [`plugins.yaml`](https://github.com/kubernetes/test-infra/blob/master/config/prow/plugins.yaml).

```yaml
external_plugins:
  org-foo/repo-bar:
  - name: refresh-remote
    endpoint: https://my-refresh-plugin.com
    events:
    - issue_comment
  - name: needs-rebase
    # No endpoint specified implies "http://{{name}}".
    events:
    - pull_request
    # Dispatching issue_comment events to the needs-rebase plugin is optional. If enabled, this may cost up to two token per comment on a PR. If `ghproxy`
    # is in use, these two tokens are only needed if the PR or its mergeability changed.
    - issue_comment
  - name: cherrypick
    # No events specified implies all event types.
```

## How to test a plugin

See ["Building, Testing, and Updating Prow"](/docs/build-test-update/#how-to-test-a-plugin).
