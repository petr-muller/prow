---
title: "Deck"
weight: 20
description: >
  Shows what jobs are running or have recently run in prow
---

## GitHub API Access

Deck uses the GitHub API to display pull request status information and to authorize rerun and abort actions based on GitHub organization membership. It requires GitHub authentication credentials and should be configured with ghproxy to manage rate limits. See [Managing GitHub API Access](/docs/github-api-access/) for details on authentication methods, endpoint configuration, and rate limit management.

## Appearance

The main Prow status dashboard (`/`) follows the system's light or dark theme
until a user chooses a theme with its header toggle. The choice persists in the
browser when local storage is available. Spyglass remains dark regardless of
that preference; other Deck pages retain their light appearance.

CSS selects the default logo for each page's theme, including before JavaScript
loads. Configured branding logos and colors keep their configured values.
Expanding dynamic theming to other pages and making Spyglass switchable are
tracked in [#944](https://github.com/kubernetes-sigs/prow/issues/944). These require
separate page and lens styling work and are deferred from logo consolidation.

## Running Deck locally

Deck can be run locally by executing `./cmd/deck/runlocal`. The scripts starts Deck via
Bazel using:

* pre-generated data (extracted from a running Prow instance)
* the local `config.yaml`
* the local static files, template files and lenses

Open your browser and go to: <http://localhost:8080>

## Debugging via Intellij / VSCode

This section describes how to debug Deck locally by running it inside 
VSCode or Intellij.

```bash
# Prepare assets
make build-tarball PROW_IMAGE=cmd/deck
mkdir -p /tmp/deck
tar -xvf ./_bin/deck.tar -C /tmp/deck 
cd /tmp/deck
# Expand all layers
for tar in *.tar.gz; do tar -xvf $tar; done

# Start Deck via go or in your IDE with the following arguments:
--config-path=./config/prow/config.yaml
--job-config-path=./config/jobs
--hook-url=http://prow.k8s.io
--spyglass
--template-files-location=/tmp/deck/var/run/ko/template
--static-files-location=/tmp/deck/var/run/ko/static
--spyglass-files-location=/tmp/deck/var/run/ko/lenses
```

## Rerun Prow Job via Prow UI

Rerun prow job can be done by visiting prow UI, locate prow job and rerun job by clicking on the ↻ button, selecting a configuration option, and then clicking `Rerun` button. For prow on github, the permission is controlled by github membership, and configured as part of deck configuration, see [`rerun_auth_configs`](https://github.com/kubernetes/test-infra/blob/0dfe42533307f9733f22d4a6abf08e1df2229fcb/config/prow/config.yaml#L92) for k8s prow.

See example below:
![Example](./prow_rerun.png)

Rerunning can also be done on Spyglass:
![Example](./spyglass_rerun.png)

This is also available for non github prow if the frontend is secured and [`allow_anyone`](https://github.com/kubernetes-sigs/prow/blob/db89760fea406dd2813e331c3d52b53b5bcbd140/pkg/apis/prowjobs/v1/types.go#L264-L265) is set to true for the job.

## Abort Prow Job via Prow UI

Aborting a prow job can be done by visiting the prow UI, locate the prow job and abort the job by clicking on the ✕ button, and then clicking `Confirm` button. For prow on github, the permission is controlled by github membership, and configured as part of deck configuration, see [`rerun_auth_configs`](https://github.com/kubernetes/test-infra/blob/0dfe42533307f9733f22d4a6abf08e1df2229fcb/config/prow/config.yaml#L92) for k8s prow. Note, the abort functionality uses the same field as rerun for permissions.

See example below:
![Example](./prow_abort.png)

Aborting can also be done on Spyglass:
![Example](./spyglass_abort.png)

This is also available for non github prow if the frontend is secured and [`allow_anyone`](https://github.com/kubernetes-sigs/prow/blob/db89760fea406dd2813e331c3d52b53b5bcbd140/pkg/apis/prowjobs/v1/types.go#L264-L265) is set to true for the job.