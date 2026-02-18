# Triage for Issue #177

**Status**: In Progress
**Created**: 2026-02-18

## Issue Information

- **Issue Number**: #177
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/177

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue reports that the "Details" link on the Tide status check for GitHub PRs does not work correctly when the PR is authored by a bot user (e.g., `dependabot`). The Tide status URL includes `author:dependabot` in the query, but GitHub's search requires `author:app/dependabot` for GitHub App bot users. This causes the link to return no results.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: Tide (status check "Details" link)
- Exists in this repo: Yes
- Relevant code paths:
  - `pkg/tide/status.go` — `targetURL()` function (lines 379-405) constructs the PR query URL using `crc.AuthorLogin`
  - `pkg/tide/status_test.go` — test coverage for `targetURL()`
  - `pkg/config/tide.go` — configuration for `PRStatusBaseURL`

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue includes:
  - A concrete example PR (dependabot PR in cluster-api-provider-azure)
  - The broken URL with the incorrect `author:dependabot` query
  - The expected working URL with `author:app/dependabot` query
  - A screenshot showing the "Details" link

### Recommendation

This is a valid bug report for the Tide component. The `targetURL()` function in `pkg/tide/status.go` constructs a query using `crc.AuthorLogin` directly, but for GitHub App bot users, the login needs to be prefixed with `app/` for the GitHub search query to match correctly.

The issue was originally filed in kubernetes/test-infra and correctly migrated to this repository. It has existing labels `kind/bug` and `help wanted`, and was confirmed still active by the author in August 2024.

**Suggested Action**:
- Keep open and continue triage

## Code Research

### Current Implementation

**Primary Components**:
- `targetURL()`: `pkg/tide/status.go:379-405` — Constructs the "Details" link URL for the Tide GitHub status check
- `CodeReviewCommon`: `pkg/tide/codereview.go:83-107` — Shared struct that carries PR metadata between Tide subsystems
- `CodeReviewCommonFromPullRequest()`: `pkg/tide/codereview.go:144-169` — Populates `CodeReviewCommon` from a GraphQL `PullRequest`
- `PullRequest` (GraphQL model): `pkg/tide/tide.go:1914-1946` — GraphQL struct for fetching PR data

**Architecture Overview**:
Tide periodically queries GitHub's GraphQL API for open PRs. For each PR, it creates a `CodeReviewCommon` struct containing metadata including `AuthorLogin`. When setting GitHub status checks, `targetURL()` constructs a "Details" link pointing to the Prow PR dashboard with a pre-populated search query: `is:pr repo:ORG/REPO author:LOGIN head:BRANCH`.

**Key Code Paths**:
1. `pkg/tide/tide.go:1916-1918` — GraphQL `PullRequest.Author` struct only fetches `Login`, not user type
2. `pkg/tide/codereview.go:161` — `AuthorLogin` set from `string(pr.Author.Login)` with no type info
3. `pkg/tide/status.go:397` — Query constructed: `fmt.Sprintf("is:pr repo:%s author:%s head:%s", ..., crc.AuthorLogin, ...)`
4. `pkg/tide/status.go:462` — `TargetURL` field set on the GitHub status via `sc.ghc.CreateStatus()`

**Data Flow**:
1. Tide sync loop queries GitHub GraphQL for PRs → `PullRequest.Author.Login` returns `"dependabot"` (no `[bot]` suffix, no type info)
2. `CodeReviewCommonFromPullRequest()` copies `Author.Login` into `AuthorLogin` string field
3. `targetURL()` uses `crc.AuthorLogin` directly in the `author:` query parameter
4. For bot users, this produces `author:dependabot` instead of the required `author:app/dependabot`

### Related Code

**GitHub User Type System** (`pkg/github/types.go:147-163`):
- `User` struct has a `Type string` field
- Constants defined: `UserTypeUser = "User"`, `UserTypeBot = "Bot"`
- This is used for REST API responses, but is NOT available in the GraphQL path

**GraphQL Inline Fragments** (existing pattern in codebase):
- `pkg/plugins/bugzilla/bugzilla.go:536` uses `graphql:"... on User"` for type discrimination
- The shurcooL/githubv4 library supports inline fragments via struct tags
- This pattern can be used to detect `Bot` type on the `Actor` interface

### Test Coverage

**Existing Tests** (`pkg/tide/status_test.go:1033-1159`):
- `TestTargetUrl` has 7 test cases covering various configuration scenarios
- All test cases use `AuthorLogin: "author"` (a regular user)
- Tests verify URL construction with `author:author` in the query
- Coverage assessment: **Missing** — no test cases for bot/app users

**Test Gaps**:
- No test for GitHub App bot users (e.g., dependabot)
- No test for the `author:app/` prefix requirement

### Documentation Review

**Code Comments**:
- `codereview.go:98-99`: "AuthorLogin is the author login from the fork on GitHub, this will be the author login from Gerrit."
- No documentation about bot user handling

**Known Limitations**:
- The code assumes all PR authors are regular users
- No distinction between user types in the GraphQL query or data model

### Root Cause Analysis

**Primary Cause**:
The `PullRequest` GraphQL struct (`pkg/tide/tide.go:1916-1918`) only queries `Author { Login }` from the GitHub GraphQL API. The `Author` field returns a GitHub `Actor` interface, which can be a `User`, `Bot`, `Organization`, etc. Without type discrimination, there is no way to detect that an author is a GitHub App bot. For bot users, GitHub's search requires `author:app/<name>` instead of `author:<name>`, but `targetURL()` always uses the bare login.

**Contributing Factors**:
1. GitHub GraphQL API returns `"dependabot"` as the login for bot users (no `[bot]` suffix), making it indistinguishable from a regular user based on login string alone
2. The `CodeReviewCommon` struct carries only a plain `AuthorLogin` string with no type information
3. The existing `User.Type` field and `UserTypeBot` constant in `pkg/github/types.go` are only used for REST API responses, not in the Tide GraphQL path

**Reproduction Conditions**:
- A PR is authored by a GitHub App bot user (e.g., dependabot, renovate)
- Tide sets a status check on the PR
- The "Details" link uses the PR status dashboard (not the overview URL)
- Clicking the link produces a query with `author:botname` which returns no results

### Proposed Solutions

#### Approach 1: Add Bot Detection via GraphQL Inline Fragment

**Description**: Extend the `PullRequest.Author` GraphQL struct to include a `... on Bot` inline fragment that detects bot authors. Propagate this information through `CodeReviewCommon` and use it in `targetURL()` to prefix the author login with `app/` for bot users.

**Changes**:
- `pkg/tide/tide.go` — Add `Bot` fragment to `Author` struct: `Bot struct { Login githubql.String } \`graphql:"... on Bot"\``
- `pkg/tide/codereview.go` — Add `AuthorIsBot bool` field to `CodeReviewCommon`; set it from `pr.Author.Bot.Login != ""`
- `pkg/tide/status.go` — In `targetURL()`, use `author:app/<login>` when `crc.AuthorIsBot` is true

**Pros**:
- Addresses root cause directly using existing GraphQL query (no extra API calls)
- Uses an established pattern in the codebase (`... on Bot` fragments)
- Clean, minimal change footprint

**Cons**:
- Requires understanding of how shurcooL/githubv4 handles inline fragments
- Need to verify the `... on Bot` fragment works correctly with the Actor interface

**Affected Components**:
- `pkg/tide/tide.go` — PullRequest struct (GraphQL model)
- `pkg/tide/codereview.go` — CodeReviewCommon struct and factory function
- `pkg/tide/status.go` — targetURL() function

**Complexity**: Low

**Backwards Compatibility**: No impact — this only changes the "Details" link URL for bot-authored PRs

#### Approach 2: Construct Author Query from PR Number Instead

**Description**: Instead of constructing a search query with `author:`, use a more direct query that avoids the author field entirely, relying on `repo:` and `head:` or potentially the PR number.

**Pros**:
- Avoids the bot detection problem entirely
- Simpler conceptually

**Cons**:
- May not uniquely identify PRs (multiple forks could use the same branch name)
- Changes the query semantics, potentially affecting dashboard behavior
- Less informative query for the user

**Complexity**: Low

**Backwards Compatibility**: Could affect PR dashboard filtering behavior for all users

#### Recommendation

**Preferred Approach**: Approach 1 (Bot Detection via GraphQL Inline Fragment)

This approach directly addresses the root cause, uses an existing codebase pattern, and has minimal blast radius. The fix is self-contained to three files and doesn't change behavior for regular users.

**Key Implementation Considerations**:
1. Verify the `... on Bot` inline fragment returns data from the GitHub GraphQL API for the `Actor` interface
2. Consider handling other bot-like types (`Mannequin`, `EnterpriseUserAccount`) if they face the same issue
3. The `[bot]` suffix convention is not reliable for detection since GraphQL returns bare login names

**Testing Requirements**:
- Add test case in `TestTargetUrl` for bot-authored PRs verifying `author:app/botname` format
- Add test for regular users to ensure no regression (already covered)

## Effort Assessment

**Effort Level**: 1 - Easy (good-first-issue)

### Summary

A well-defined, small-scope bug fix touching 3 files with clear solution approach. The fix adds bot type detection via a GraphQL inline fragment and adjusts the URL query construction. Existing test patterns can be followed directly.

### Factor Analysis

#### Scope of Changes
- **Assessment**: Small
- **Details**: 3 files modified (`pkg/tide/tide.go`, `pkg/tide/codereview.go`, `pkg/tide/status.go`) plus test updates in `status_test.go`. Estimated ~30 lines of code changes.
- **Level Indication**: 1-2

#### Complexity
- **Assessment**: Simple
- **Details**: The fix is straightforward: add a field to the GraphQL struct, propagate a boolean, and conditionally prefix `app/` in a format string. No concurrency, no algorithmic challenges, one edge case (bot vs. non-bot).
- **Level Indication**: 1-2

#### Required Expertise
- **Assessment**: Minimal
- **Details**: Requires basic Go knowledge and understanding of struct tags for the GraphQL library. The existing `... on User` pattern in `pkg/plugins/bugzilla/bugzilla.go` serves as a direct example. No Prow-specific architectural knowledge needed beyond following the data flow.
- **Level Indication**: 1-2

#### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**: The problem is clearly described with concrete examples. The root cause is identified (missing type info in GraphQL query). The solution approach (inline fragment + conditional prefix) is unambiguous. The only minor uncertainty is verifying the `... on Bot` fragment works with the `Actor` interface, which can be validated quickly.
- **Level Indication**: 1-2

#### Testing Requirements
- **Assessment**: Simple
- **Details**: Add 1-2 test cases to the existing `TestTargetUrl` function following the established pattern. Set `AuthorIsBot: true` and verify the URL contains `author:app/botname`. Existing test infrastructure is sufficient.
- **Level Indication**: 1-2

#### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**: Only changes the "Details" link URL for bot-authored PRs (which is currently broken anyway). No behavior change for regular users. No configuration changes needed.
- **Level Indication**: 1-2

#### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**: Uses existing patterns (GraphQL inline fragments, boolean fields in `CodeReviewCommon`). Follows the established data flow. No new patterns introduced.
- **Level Indication**: 1-2

#### External Dependencies
- **Assessment**: Well-supported
- **Details**: GitHub GraphQL API supports the `Bot` type on the `Actor` interface. The shurcooL/githubv4 library supports inline fragments. The `author:app/` search syntax is documented GitHub behavior.
- **Level Indication**: 1-3

### Recommended Labels

- [x] `good-first-issue`: Clear, well-defined, small scope, existing patterns to follow
- [x] `kind/bug`: Fixing broken "Details" link for bot-authored PRs
- [x] `area/tide`: Tide component
- [ ] `help-needed`: Already has this label, but `good-first-issue` is more appropriate given the low effort

### Guidance for Contributors

- Good starting point for new Prow contributors
- Suggested prerequisite knowledge: Basic Go, understanding of Go struct tags
- Key files to review:
  - `pkg/tide/status.go:379-405` — the function to fix
  - `pkg/tide/codereview.go:83-169` — the data structure and factory function
  - `pkg/tide/tide.go:1914-1946` — the GraphQL model to extend
  - `pkg/plugins/bugzilla/bugzilla.go:531-537` — example of `... on` fragment pattern
  - `pkg/tide/status_test.go:1033-1159` — existing tests to extend

### Caveats and Considerations

- The existing `help wanted` label should be replaced with `good-first-issue` since this is a Level 1 issue
- The `Mannequin` and `EnterpriseUserAccount` Actor types may also need the `app/` prefix, but this is a rare edge case and can be addressed separately if needed
- The fix should be verified against a real GitHub App bot PR to confirm the `... on Bot` fragment populates correctly

## Proposed Issue Augmentation

### Title Change

- **No change needed**: Current title ("Details" link for "tide" check on GitHub PRs doesn't work for PRs authored by bot users) is specific, mentions the affected component (Tide), describes the symptom clearly, and identifies the trigger condition (bot users).

### Proposed GitHub Comment

```
The root cause is in `pkg/tide/status.go`, in the `targetURL()` function that constructs the "Details" link URL. It builds a query string using `author:<login>`, where the login comes from the GraphQL `PullRequest.Author.Login` field. For GitHub App bot users like dependabot, the GraphQL API returns just `"dependabot"` as the login. However, GitHub's search syntax requires `author:app/dependabot` for App bot users. The code has no way to distinguish bot authors from regular users because the GraphQL query (defined in `pkg/tide/tide.go`) only fetches `Author { Login }` without any type discrimination on the `Actor` interface.

The fix would involve adding a `... on Bot` inline fragment to the GraphQL `PullRequest.Author` struct (the shurcooL/githubv4 library supports this pattern, and there's an existing example in `pkg/plugins/bugzilla/bugzilla.go`), propagating the bot type info through `CodeReviewCommon`, and conditionally prefixing `app/` in the author query parameter when the author is a Bot. This touches three files (`pkg/tide/tide.go`, `pkg/tide/codereview.go`, `pkg/tide/status.go`) with roughly 30 lines of changes.

/area tide
/good-first-issue
```

### Rationale

**What's being added**:
- Root cause explanation: The issue author correctly identified the symptom and the fix needed in the URL, but the underlying code path and reason why the login lacks the `app/` prefix was not explained. The augmentation identifies exactly where the bug is, why the GraphQL query is insufficient, and what the fix entails.
- Implementation guidance: Specific file paths, existing pattern reference (`bugzilla.go`), and scope of changes to help a contributor get started quickly.

**Why these labels**:
- `/area tide`: The bug is in Tide's status URL generation code
- `/good-first-issue`: Level 1 effort assessment — small scope (3 files, ~30 LOC), clear solution using an existing codebase pattern, no architectural concerns

**What's NOT included**:
- `/kind bug`: Already present on the issue
- `/help-wanted`: Already present; `good-first-issue` is more appropriate for a Level 1 issue. Both can coexist, so not removing it.
- `/retitle`: Current title is already clear and specific
- `/priority`: This is a broken link, not a critical or blocking issue

## Next Steps

- Brief the maintainer on findings
- Wrap up and post the comment
