---
title: "Managing GitHub API Access"
weight: 104
description: >
  
---

Prow components communicate with GitHub to handle webhooks, manage pull requests, update commit statuses, and more. This guide covers operational aspects of authenticating to GitHub, configuring components that access it, and managing its usage through the ghproxy caching proxy.

## GitHub Authentication

Prow supports two authentication methods: personal access tokens and GitHub Apps. Both methods provide API credentials for Prow components to interact with GitHub.

### GitHub Token Authentication

The first authentication method is generating a [personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens) for a GitHub account. This method is suitable for personal and ad-hoc testing deployments, but not recommended for production use.

Components accept the `--github-token-path` flag pointing to a file containing the token. In a cluster deployment, the token would typically be stored in a Kubernetes Secret and mounted as a volume to make the file available to the component.

### GitHub App Authentication

The second authentication method uses [GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/about-creating-github-apps). See the [Deploying Prow](/docs/getting-started-deploy/#github-app) guide for how to set up a GitHub App for Prow.

GitHub Apps require two secrets: the App ID and a private key. Components accept the `--github-app-id` flag (typically passed as an environment variable) and the `--github-app-private-key-path` flag pointing to a file containing the private key. In a cluster deployment, both would typically be stored in a Kubernetes Secret and made available to the component.

GitHub Apps are the recommended approach for production deployments because they provide:

- **Installation-specific rate limits:** Installation access tokens have their own [REST API rate limit](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#primary-rate-limit-for-github-app-installations), which can scale with the installation's repositories and organization users
- **Granular permissions:** More fine-grained control over what Prow can access
- **Audit trail:** Actions appear as the App rather than a user account
- **Organization-wide:** Easier to manage access across multiple repositories

## Managing API Rate Limits

GitHub enforces rate limits on API requests. REST and GraphQL have separate hourly budgets, while REST search has a separate per-minute limit:

- **REST API (v3):** [Personal access tokens and GitHub App installation tokens](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api) start at 5,000 requests/hour. For installations outside GitHub Enterprise Cloud, the limit scales with repositories and organization users above 20 of each, up to 12,500 requests/hour. Installations on GitHub Enterprise Cloud organizations have a 15,000 requests/hour limit.
- **GraphQL API (v4):** [A separate point-based limit](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api#primary-rate-limit) starts at 5,000 points/hour for users and App installations. Installation limits outside GitHub Enterprise Cloud can scale up to 12,500 points/hour; installations on GitHub Enterprise Cloud organizations have a 10,000 points/hour limit.
- **REST Search API:** [Most authenticated search endpoints](https://docs.github.com/en/rest/search/search#rate-limit) allow 30 requests/minute in a separate REST search resource; code search is limited to 10 requests/minute. This resource is separate from GraphQL's rate limit.

When a limit is exhausted, GitHub may return HTTP 403 or 429 for REST requests, or a GraphQL error. The GitHub client library tracks remaining tokens from response headers and can throttle requests to avoid hitting the limit.

Prow provides two mechanisms to manage rate limits:

### Client-side Throttling

Components accept flags to configure how aggressively they consume the API token budget:

- `--github-hourly-tokens`: Maximum GitHub tokens to consume per hour (default: matches GitHub's limit)
- `--github-allowed-burst`: Maximum burst of tokens that can be consumed at once (default: 100)

These flags allow operators to reserve part of the token budget for other tools or configure more conservative throttling. The client library automatically throttles requests when approaching the configured limit.

### ghproxy Caching Proxy

[ghproxy](/docs/ghproxy/) is a reverse proxy HTTP cache designed for the GitHub API. It caches responses and reduces redundant requests, helping Prow stay within rate limits even under heavy load.

**Benefits:**

- Reduces API token usage by serving cached responses
- Shares a single cache across all Prow components
- Optimized client throttling that doesn't count cached responses against the budget
- Automatic handling of conditional requests (ETags) for efficient cache revalidation

### Deploying ghproxy

ghproxy is deployed as a service in the Prow cluster. It requires a persistent volume for the cache and accepts flags to configure cache size and location. See the [ghproxy documentation](/docs/ghproxy/) for deployment details and configuration options.

### Configuring Components to Use ghproxy

Components that access the GitHub API configure REST and GraphQL endpoints separately:

- `--github-endpoint`: REST API (v3) endpoint. Repeat this flag to order endpoints for fallback. REST search requests use these endpoints too.
- `--github-graphql-endpoint`: One GraphQL API (v4) endpoint.

For REST requests, the client tries the endpoints in order and moves to the next if a connection fails. A typical configuration uses ghproxy first and the direct GitHub REST API as a fallback. GraphQL uses only the single configured endpoint:

```
--github-endpoint=http://ghproxy
--github-endpoint=https://api.github.com
--github-graphql-endpoint=http://ghproxy/graphql
```

The GitHub client library automatically tracks which requests were served from cache (ghproxy) versus direct API calls, and optimizes throttling accordingly—cached responses don't count against the token budget.

> **Note:** GitHub Enterprise deployments use different API endpoint URLs. See [Deploying with GitHub Enterprise](/docs/getting-started-deploy/#deploying-with-github-enterprise) for complete configuration details.

## See Also

- [Deploying Prow](/docs/getting-started-deploy/) - Complete deployment guide including GitHub App setup
- [ghproxy](/docs/ghproxy/) - Internal architecture and advanced configuration
- [GitHub API Library](/docs/github/) - Developer documentation for the Go client library
