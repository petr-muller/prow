---
issue: kubernetes-sigs/prow#527
title: "Docs: Add subsection for running Prow outside GKE (e.g., local/Kind)"
state: open
labels: lifecycle/rotten, area/documentation
main_sha: a52d6ea9917b8de587b6858d7c28cf05e017f8e1
triaged_at: 2026-05-19T23:30:17Z
verdict: accepted
---

## Findings

### [reproducibility] Core ask addressed by local-dev.md (March 2026)
- detail: The original request (local/Kind setup docs) was fully addressed by `local-dev.md` and `local-dev-tilt.md` added 2026-03-26. These cover `make dev`, Kind cluster setup, in-cluster fakes for all external services, `hack/phony.sh` for fake webhooks, and a hook plugin dev workflow.
- evidence: `site/content/en/docs/local-dev.md` (commit 1797489fe, 2026-03-26), `site/content/en/docs/local-dev-tilt.md` (commit c1e6738396, 2026-03-26)

### [cause] getting-started-deploy.md lacks cross-reference to local-dev.md
- detail: The deploy guide opens with "focused on GKE but should work on any kubernetes distro" and never mentions `local-dev.md`. A developer arriving from search has no signal a local path exists. `getting-started-develop.md` does cross-reference `local-dev.md` correctly, proving the pattern works.
- evidence: `site/content/en/docs/getting-started-deploy.md:1-15` (no mention of local-dev); `site/content/en/docs/getting-started-develop.md:64-66` (working cross-reference example)

### [related-code] getting-started-deploy.md — GKE-centric opening, no local-dev mention
- where: `site/content/en/docs/getting-started-deploy.md:1-15`
- excerpt: |
    title: "Deploying Prow"
    The guide below is focused on Google Kubernetes Engine but should work on
    any kubernetes distro with no/minimal changes.

### [related-code] getting-started-develop.md — working cross-reference pattern to follow
- where: `site/content/en/docs/getting-started-develop.md:64-66`
- excerpt: |
    For local development without a cloud account, the Local Development
    Environment guide explains how to run a full Prow stack in a local kind
    cluster using in-cluster fakes...

### [related-code] local-dev.md — fully addresses original ask
- where: `site/content/en/docs/local-dev.md:1-7`
- excerpt: |
    title: "Local Development Environment"
    Run a complete Prow stack locally using kind, with fake replacements for
    all external services (GitHub, GCS, Gerrit, Pub/Sub).

### [related-issue] #283 — move starter config from test-infra
- ref: kubernetes-sigs/prow#283
- relevance: Identified by @BenTheElder as the main actionable item from this issue; closed 2025-11-13.

## Contribution path coverage
- Hook plugins: covered — `local-dev.md:170-181` + `getting-started-develop.md`
- Tide changes: not covered
- Deck/frontend: partial — `deck/_index.md` has `runlocal`, no full dev workflow
- ProwJob controllers: not covered
- Gerrit integration: not covered (full profile deploys it, no dev guide)

## Checked
- git log for `local-dev.md` and `local-dev-tilt.md` — confirmed added 2026-03-26
- full content of `getting-started-deploy.md` — confirmed no cross-reference to local-dev
- full content of `getting-started-develop.md` — confirmed it does cross-reference local-dev.md
- issue #283 state — confirmed closed
- Hugo frontmatter weights: local-dev.md=75, local-dev-tilt.md=76, getting-started-deploy.md=80
- hack/ scripts: dev-env.sh and phony.sh documented; tilt-apply-config.sh and tilt-build.sh undocumented
- full issue comment thread through 2026-04-23

## Next steps
- Add cross-reference note near top of `site/content/en/docs/getting-started-deploy.md` pointing to `/docs/local-dev/` — ~5 lines, Level 1 (good-first-issue)
- Post augmentation comment: /retitle, /remove-lifecycle stale, /kind cleanup, /good-first-issue (comment text in TRIAGE.html)
- Do not apply /help-wanted — too simple for that label
- Track broader contribution-path dev guides (Tide, Deck, ProwJob controllers) in a separate issue

## Open questions
- Should the cross-reference in `getting-started-deploy.md` use a Hugo/Docsy callout admonition shortcode, or plain prose like the pattern in `getting-started-develop.md:64-66`?
- Is the broader "Prow development guide" vision (@BenTheElder 2025-10-14, @petr-muller 2026-01-19) worth a dedicated tracking issue, or left for organic future work?
