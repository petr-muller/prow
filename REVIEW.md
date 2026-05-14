---
pr: kubernetes-sigs/prow#712
title: "feat(peribolos): clean up failed org invitations before re-inviting"
head_sha: 3a2c9732796cda893fe7f09fc50420ca80e59ae5
base: main
reviewed_at: "2026-05-14T12:27:07Z"
verdict: APPROVE_WITH_SUGGESTIONS
refresh_log:
  - from: 3a2c9732796cda893fe7f09fc50420ca80e59ae5
    to: 3a2c9732796cda893fe7f09fc50420ca80e59ae5
    summary: "No code changes. PR merged after LGTM (petr-muller) and /hold cancel (cblecker)."
---

# PR #712 — feat(peribolos): clean up failed org invitations before re-inviting

**Author**: cblecker (Christoph Blecker) | **Size**: +276 / -28 | **Labels**: approved, area/peribolos, cncf-cla: yes, size/L, lgtm, do-not-merge/hold

## What this PR does

When a GitHub org invitation fails (e.g. 2FA not configured), the stale failed-invitation record blocks subsequent re-invites. This PR makes peribolos detect those records and delete them before issuing a fresh invite.

Since previous review: no code changes. The PR was LGTM'd by petr-muller (2026-05-11), approved by k8s-ci-robot, hold cancelled by cblecker (2026-05-13), and **merged**.

| Component | Change |
|-----------|--------|
| `pkg/github/types.go` | Adds `ID`, `FailedAt`, `FailedReason` to `OrgInvitation` |
| `pkg/github/client.go` | New `ListFailedOrgInvitations` and `DeleteOrgInvitation` on `OrganizationClient` |
| `cmd/peribolos/main.go` | New `orgFailedInvitations()` fetch; cleanup in the `adder` closure before re-invite |
| `cmd/peribolos/main_test.go` | 11 new test cases across `TestOrgFailedInvitations` (6) and `TestConfigureOrgMembers` (5) |

## Control flow

```
configureOrg(org)
  ├─ orgInvitations()            → invitees (pending invites, as before)
  ├─ orgFailedInvitations()      → failedInvites map[login][]invitationID  ← NEW
  └─ configureOrgMembers(…, invitees, failedInvites)
       └─ adder(user, super)
            ├─ if invitees.Has(user): return (pending invite exists, skip)
            ├─ for each failedInvites[user]:        ← NEW
            │    DeleteOrgInvitation (warn on error)
            └─ UpdateOrgMembership (re-invite)
```

## What looks good

- **Consistent API patterns**: `ListFailedOrgInvitations` mirrors `ListOrgInvitations` exactly — same pagination, same `acceptNone`, same `c.fake` guard.
- **Idempotent delete**: `DeleteOrgInvitation` accepts both 204 and 404 exit codes.
- **Correct ordering in the adder**: Pending invitations short-circuit first; failed-invite cleanup runs only when a re-invite is about to happen.
- **Soft-fail on delete**: Delete errors are warnings, don't abort sync, don't prevent the re-invite.
- **Nil-safe map access**: When `orgFailedInvitations` returns nil, `range failedInvites[user]` is a no-op.
- **Login normalization**: `github.NormLogin` applied before map insertion.
- **Gating logic**: `orgFailedInvitations` only runs when `fixOrgMembers && !ignoreInvitees`, with a comment explaining why `fixTeamMembers` is excluded.

## Items to discuss

- **`time.Time` and `omitempty`** (`types.go:1201`): Go's `encoding/json` does not treat zero `time.Time` as empty — `omitempty` is inert. Not functional since peribolos only reads from GitHub, but could mislead future readers.
- **Whitespace-only reformatting** (`main.go:47-64`, `main_test.go:1866-1870`): Alignment changes add diff noise.
- **`intSliceToString` uses lexicographic sort** (`main_test.go:849-855`): `sort.Strings` means `"10" < "2"`. Harmless for current values.
- **`OrgInvitation.ID` now populated for pending invitations too** (`types.go:1198`): Additive, backwards-compatible.

## Multi-perspective maintainer review

### Code Quality — APPROVE

No critical issues. Implementation follows existing patterns closely with good test coverage. Minor notes: inert `omitempty` tag on `FailedAt`, implicit normalization contract in adder closure, `failedInvites` double-use in tests.

### Maintainability — APPROVE (LOW burden)

Follows established patterns (interface segregation, warn-and-continue, normalization, table-driven tests). Minor notes: per-invitation Info logging could be noisy, `configureOrgMembers` parameter count (6) is a threshold signal.

### Deployment Risk — LOW

No breaking changes, no config changes, no new CLI flags. Behavioral change is strictly positive. GHE compatibility preserved via `--ignore-invitees`. Dry-run support respected. Rollback is safe.

### Converging concern

**`failedInvites` double-use in tests**: flagged by Code Quality and Maintainability. Same map passed as both `fakeClient` field and direct argument. Functionally correct but slightly confusing. Non-blocking.

### Advisor suggestions (non-blocking)

- Consider a single summary log line instead of per-invitation Info logging.
- Note GHE Server compatibility in PR description for operators not using `--ignore-invitees`.
- `time.Time` omitempty tag is inert — consider `*time.Time` or dropping the tag.

### Deployment notes

- `OrganizationClient` interface gains two methods; downstream implementors need stubs.
- GHE Server installations not using `--ignore-invitees` should verify `/failed_invitations` endpoint support.

## Gate decision: MERGE

No show-stoppers found. The implementation is clean, well-tested, follows existing patterns, and carries low deployment risk.

## Risks

- **Low**: Blast radius is minimal — cleanup only fires for users with failed invitations who are about to be re-invited.
- **Low**: API permissions already available — no new token scopes needed.
- **Low**: Rate limiting — one extra paginated GET per org, DELETE calls proportional to problem size.

## Test coverage

| Scenario | Test | File:Line |
|----------|------|-----------|
| Skip when `fixOrgMembers` false | TestOrgFailedInvitations | main_test.go:557 |
| Skip when `ignoreInvitees` set | TestOrgFailedInvitations | main_test.go:563 |
| Returns failed invitations | TestOrgFailedInvitations | main_test.go:571 |
| Multiple invitations for same user | TestOrgFailedInvitations | main_test.go:578 |
| Login case normalization | TestOrgFailedInvitations | main_test.go:583 |
| List API error | TestOrgFailedInvitations | main_test.go:590 |
| Delete failed invite then re-invite (member) | TestConfigureOrgMembers | main_test.go:770 |
| Delete all failed invites for same user | TestConfigureOrgMembers | main_test.go:778 |
| Delete failed invite (admin role) | TestConfigureOrgMembers | main_test.go:786 |
| Delete failure still re-invites | TestConfigureOrgMembers | main_test.go:793 |
| Pending invite takes precedence | TestConfigureOrgMembers | main_test.go:801 |

## Verdict

APPROVE WITH SUGGESTIONS. Well-structured, well-tested, low-risk change that solves a real operational problem. All suggestions are minor polish items, not blockers.
