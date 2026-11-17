# App-pinned required checks: API and compatibility evidence

Research date: 2026-11-16. Sources below are GitHub's own documentation, OpenAPI descriptions, and GraphQL schema.

## REST exposes the required provider

`GET /repos/{owner}/{repo}/branches/{branch}/protection` returns `required_status_checks.contexts` and `required_status_checks.checks`. Each check has a context and a nullable integer `app_id`. A positive ID identifies the GitHub App required to supply that context. The write API documents `-1` as explicitly allowing any app; a null provider is also not a positive App pin. Detect pins with `app_id > 0`, retaining existing context handling for responses without `checks`.

Primary sources:

- [REST branch protection: Get branch protection](https://docs.github.com/en/rest/branches/branch-protection#get-branch-protection), including its example response.
- [REST branch protection: Update branch protection](https://docs.github.com/en/rest/branches/branch-protection#update-branch-protection), parameter `required_status_checks.checks[].app_id`: “The ID of the GitHub App that must provide this check”; `-1` explicitly allows any app.
- [Official GHES 3.4 OpenAPI description](https://github.com/github/rest-api-description/blob/main/descriptions/ghes-3.4/ghes-3.4.json), `components.schemas.status-check-policy.properties.checks`: array items contain `context` and nullable integer `app_id`. The `GET .../protection` response resolves through `branch-protection.required_status_checks` to `protected-branch-required-status-check`, which likewise contains `checks` with `context` and nullable integer `app_id`.

## Earliest GHES support is 3.4

GitHub Enterprise Server **3.4.0, released 2022-03-15**, introduced requiring a particular App to provide a status check. Release notes state that a different application's result, or a user's commit status, prevents merging.

This aligns exactly with official REST schemas: `status-check-policy` has only `contexts` and no `checks` in GHES 3.3, and adds `checks` with nullable `app_id` in GHES 3.4. The inspected GHES 3.0–3.2 schemas also lack this field. Therefore an additive JSON field in the existing REST response type preserves decoding on older GHES: absent `checks` decodes to an empty slice, and legacy behavior continues.

Primary sources:

- [GHES 3.4.0 release notes](https://docs.github.com/en/enterprise-server@3.4/admin/release-notes#3.4.0), repository-related changes: “When configuring protected branches, you can now enforce that a required status check is provided by a specific ... GitHub App.”
- [Official GHES 3.3 OpenAPI description](https://github.com/github/rest-api-description/blob/main/descriptions/ghes-3.3/ghes-3.3.json), `components.schemas.status-check-policy`, compared with the 3.4 description above.
- [GitHub's original 2021-12-01 announcement](https://github.blog/changelog/2021-12-01-ensure-required-status-checks-provided-by-the-intended-app/), linked by those release notes.

## GraphQL has richer data, but context-only fields lose the provider

`BranchProtectionRule.requiredStatusCheckContexts: [String]` returns names only. `BranchProtectionRule.requiredStatusChecks: [RequiredStatusCheckDescription!]` returns entries with `context: String!` and nullable `app: App`; the schema describes `app` as “The App that must provide this status in order for it to be accepted.” Accordingly, provider-specific detection cannot be implemented from `requiredStatusCheckContexts` alone.

Adding `requiredStatusChecks` to an unconditional GraphQL query would require establishing schema support/fallback for old GHES. Extending the already-used REST branch protection decoder avoids that extra compatibility problem. This note establishes the REST field's GHES introduction; it does not assert a separately verified historical introduction date for the GraphQL field.

Primary source: [GitHub's official Octokit GraphQL schema](https://github.com/octokit/graphql-schema/blob/master/schema.graphql), definitions of `BranchProtectionRule` and `RequiredStatusCheckDescription`.

## Prow's documented GHES compatibility

The checked-out repository documents [Deploying with GitHub Enterprise](site/content/en/docs/getting-started-deploy.md#deploying-with-github-enterprise), including REST/GraphQL endpoints and component flags. A repository-wide search of Markdown documentation found **no declared minimum GHES version or supported-version matrix**. The safe inference is to preserve older-server behavior, rather than infer that Prow requires GHES 3.4 or later.
