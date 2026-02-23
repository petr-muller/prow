# Triage for Issue #482

**Status**: In Progress
**Created**: 2026-02-23

## Issue Information

- **Issue Number**: #482
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/482

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

This issue proposes exploring GitHub's improved Search API (nested queries, boolean operators) for opportunities to improve Prow. Filed by petr-muller (maintainer) and reopened by stmcginnis (maintainer) after automated lifecycle closure, demonstrating ongoing maintainer interest.

**Issue Category**: Feature Request

**Repository Scope Check**:
- Components mentioned: GitHub client (`pkg/github/client.go`), Tide (`pkg/tide/github.go`, `pkg/tide/blockers/blockers.go`), needs-rebase plugin (`cmd/external-plugins/needs-rebase/plugin/plugin.go`)
- Exists in this repo: Yes - all five referenced code locations are in this repository
- Relevant code paths:
  - `pkg/github/client.go` (Search API client)
  - `pkg/tide/github.go` (Tide search queries)
  - `pkg/tide/blockers/blockers.go` (Blocker search queries)
  - `cmd/external-plugins/needs-rebase/plugin/plugin.go` (needs-rebase search)

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue provides:
  - Links to GitHub's API improvement announcements
  - Five specific code locations that use the Search API
  - Two concrete improvement directions: (1) new configuration language leveraging boolean operators, (2) internal query merging to reduce API calls
  - Context about Tide's configuration language being essentially a GH search query through YAML

### Recommendation

Keep open and continue triage. This is a well-constructed feature request filed and maintained by project maintainers. It identifies specific code locations and proposes concrete improvement directions. The issue is exploratory in nature ("we should explore whether these improvements offer opportunities") which is appropriate for a feature request.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Critical Constraint: API vs Web UI Availability

**The issue's core premise requires clarification.** GitHub's improved search with nested queries and boolean operators was announced for the **web UI Issues search** (built on a new AST-based parser and Elasticsearch backend). However, Prow does not use the web UI — it uses GitHub's **REST Search API** (`/search/issues`) and **GraphQL API** (`search` connection).

**REST API constraints** (per GitHub docs):
- Supports AND, OR, NOT operators — but limited to **max 5 per query**
- Query length limited to **256 characters** (excluding operators/qualifiers)
- **No documented support for parentheses/nested queries**
- 1000 result limit per query (hard cap)

**GraphQL API**: Uses the same search query syntax as REST, so the same limitations apply.

**Web UI Issues search** (the feature announced in the blog posts):
- Supports AND, OR, NOT and nested queries with parentheses (up to 5 levels deep)
- Not clearly available through the REST/GraphQL APIs

This means the "nested queries and boolean operators" opportunity is more constrained than the issue implies when it comes to programmatic API usage.

### Current Implementation

**Primary Components**:

| Component | File | Query Method | API Type | Key Limitation |
|-----------|------|--------------|----------|----------------|
| GitHub Client | `pkg/github/client.go:3472-3528` | `FindIssues`/`FindIssuesWithOrg` | REST | Basic query strings |
| Tide Core | `pkg/tide/github.go:101-212` | GraphQL `search()` | GraphQL | Date-range sharding |
| Tide Config | `pkg/config/tide.go:502-633` | `TideQuery.Query()`/`OrgQueries()` | String builder | No boolean operators |
| Blockers | `pkg/tide/blockers/blockers.go:97-238` | GraphQL `search()` | GraphQL | 1000 result cap |
| needs-rebase | `cmd/external-plugins/needs-rebase/plugin/plugin.go:143-394` | GraphQL `search()` | GraphQL | Requires sharding |
| Blunderbuss | `pkg/plugins/blunderbuss/blunderbuss.go:241-320` | `FindIssuesWithOrg` | REST | Simple SHA lookup |
| Welcome | `pkg/plugins/welcome/welcome.go:109-169` | `FindIssuesWithOrg` | REST | Simple author lookup |
| CLA | `pkg/plugins/cla/cla.go:83-172` | `FindIssues` | REST | Requires retries |

**Architecture Overview**:

Search in Prow flows through two paths:
1. **Tide/Blockers/needs-rebase**: Configuration → TideQuery struct → query string → GraphQL `search` connection → paginated results
2. **Plugins (blunderbuss/welcome/cla)**: Event → simple query string → REST `/search/issues` → results

The TideQuery struct (`pkg/config/tide.go:504-520`) maps YAML configuration fields to GitHub search qualifiers:
- `Orgs/Repos/ExcludedRepos` → `org:"x"` / `repo:"x/y"` / `-repo:"x/y"`
- `Labels` → `label:"x"` (supports CSV for OR: `label:"lgtm","approved"`)
- `MissingLabels` → `-label:"x"`
- `IncludedBranches/ExcludedBranches` → `base:"x"` / `-base:"x"`
- `Author` → `author:"x"`
- `Milestone` → `milestone:"x"`
- Always includes: `is:pr state:open archived:false`

**Key Code Paths**:

1. **Tide query construction**: `pkg/config/tide.go:554-602` — `constructQuery()` builds org-scoped query parts from YAML config
2. **Tide GraphQL search**: `pkg/tide/github.go:165-212` — `search()` executes paginated GraphQL search with date ranges
3. **Org query sharding**: `pkg/config/tide.go:615-633` — `OrgQueries()` splits queries by org for GitHub Apps auth
4. **needs-rebase sharding**: `cmd/external-plugins/needs-rebase/plugin/plugin.go:344-394` — special-cases kubernetes org into 3 queries to work around 1000-result cap
5. **Blocker search**: `pkg/tide/blockers/blockers.go:190-196` — `blockerQuery()` builds `is:issue state:open label:"blocker"` queries

### Related Code

**Helpers**:
- `pkg/tide/tide.go:2039-2057` — `orgRepoQueryStrings()` builds org/repo filter tokens for queries
- `pkg/tide/github.go:214-230` — `datedQuery()`/`dateToken()` adds date range filtering

**Callers of FindIssues/FindIssuesWithOrg**:
- Blunderbuss plugin (SHA-based PR lookup)
- Welcome plugin (author's prior PRs check)
- CLA plugin (SHA-based PR lookup with retries)

### Test Coverage

**Existing Tests**:
- `pkg/config/tide_test.go` — Tests TideQuery.Query() and OrgQueries() string generation
- `pkg/tide/github_test.go` — Tests search() and datedQuery() with mocked GraphQL
- `cmd/external-plugins/needs-rebase/plugin/plugin_test.go` — Tests query construction and sharding
- Coverage assessment: Good for query construction, partial for search execution

### Root Cause Analysis

**Primary Finding**: The issue's exploration opportunity is constrained by API limitations.

The GitHub Search REST and GraphQL APIs support basic AND/OR/NOT operators (max 5 per query), but **not** the nested parenthetical queries that the blog posts announced. Those nested queries are a web UI feature built on GitHub's new internal search infrastructure (AST parser + Elasticsearch), and it is unclear whether they are exposed through the programmatic APIs.

**Practical Constraint**: Even if the API were to gain full nested query support, the 5-operator limit and 256-character query limit would significantly constrain how much query merging is practical.

**Contributing Factors**:
1. GitHub's 1000-result-per-query cap forces result sharding (especially for kubernetes org)
2. GitHub Apps auth requires org-scoped queries (can't merge across orgs)
3. TideQuery's YAML-to-search-query mapping is already quite efficient within current API constraints

### Proposed Solutions

#### Approach 1: Leverage Existing AND/OR/NOT Support (Limited)

**Description**: Use the existing boolean operator support in the REST/GraphQL API to merge some queries. For example, multiple TideQuery objects targeting the same org with different label requirements could be combined with OR (within the 5-operator limit).

**Pros**:
- Uses documented, existing API capabilities
- Could reduce API calls in some configurations
- No configuration language changes needed

**Cons**:
- 5-operator limit severely restricts merging potential
- 256-character query limit may prevent combining complex queries
- Unclear if parentheses work in the API (needed for correct OR semantics)
- Marginal improvement for most real-world configurations

**Affected Components**:
- `pkg/config/tide.go` — TideQuery merging logic
- `pkg/tide/github.go` — Query execution

**Complexity**: Medium
**Backwards Compatibility**: Full — internal optimization only

#### Approach 2: New Configuration Language (Future-Facing)

**Description**: Design a new Tide merge criteria configuration language that uses boolean expressions natively, independent of GitHub Search API syntax. The configuration would be compiled into optimized API queries internally.

**Pros**:
- Better user experience for complex merge criteria
- Decouples configuration from API syntax
- Ready to leverage future API improvements

**Cons**:
- Significant design and implementation effort
- Two configuration languages to maintain (old + new)
- Limited immediate benefit if the API doesn't support nested queries
- Migration complexity for existing users

**Affected Components**:
- `pkg/config/tide.go` — New config types and parsing
- `pkg/tide/github.go` — Query compilation from new config
- Documentation and migration tooling

**Complexity**: High
**Backwards Compatibility**: Additive (old config still works)

#### Approach 3: Internal Query Optimization

**Description**: Optimize how Prow constructs and executes search queries without changing configuration. Focus on reducing redundant API calls by merging queries that share common filters, improving date-range sharding, and batching related searches.

**Pros**:
- No user-facing changes
- Reduces API rate limit consumption
- Can be done incrementally

**Cons**:
- Limited by current API constraints
- Optimization opportunities depend on specific configurations
- Complexity of query merging logic

**Affected Components**:
- `pkg/tide/github.go` — Query batching logic
- `pkg/tide/tide.go` — orgRepoQueryStrings optimization

**Complexity**: Medium
**Backwards Compatibility**: Full — internal optimization only

#### Recommendation

**Preferred Approach**: Approach 3 (Internal Query Optimization) as the immediate focus, with Approach 1 as a secondary investigation.

Before investing in significant changes, the critical first step is to **verify** whether the REST/GraphQL APIs actually support the new boolean and nested query syntax. This can be done with simple manual API calls. If the APIs do support it, Approach 1 becomes more viable. If not, Approach 3 provides value within current constraints.

Approach 2 is premature until the API situation is clarified and there's demonstrated user demand for more expressive merge criteria.

**Key Implementation Considerations**:
1. Manually verify API support for AND/OR/NOT and parentheses via `gh api`
2. Profile actual API usage patterns to identify highest-value optimization targets
3. The needs-rebase kubernetes sharding workaround is the most obvious optimization candidate
4. Any changes must preserve org-scoped query isolation for GitHub Apps auth

**Testing Requirements**:
- Unit tests for query merging logic
- Integration tests verifying merged queries return same results as individual queries
- API rate limit impact measurement

## Next Steps

(Action items will be added here)
