---
pr: kubernetes-sigs/prow#722
title: "chore: upgrade github.com/golang-jwt/jwt to v5"
head_sha: d1e585e80caa4e55ac8eebe11ca81dee87cfe61d
base: main
reviewed_at: 2026-05-18T15:57:18Z
verdict: approve
---

## What this PR does

- Upgrades `github.com/golang-jwt/jwt` from v4 to v5 (import path change: `jwt/v4` → `jwt/v5`).
- Updates three files: `pkg/github/app_auth_roundtripper.go`, `pkg/flagutil/github.go`, `pkg/clonerefs/run.go`.
- Reclassifies `aws-sdk-go` v1 from direct to indirect in `go.mod` (side effect of `go mod tidy`).
- Keeps `jwt/v4` as an indirect dep because something in the transitive graph still requires it.

## Findings

### [question] image build test was pending at review time
- where: CI
- concern: `pull-prow-image-build-test` was pending when this review was written. Confirm it lands green before approving merge.
- excerpt: |
    pull-prow-image-build-test	pending	...	Job triggered.

## Checked

- All three jwt import sites updated; no remaining `jwt/v4` imports in production or test code.
- API surface used (`ParseRSAPrivateKeyFromPEM`, `NewWithClaims`, `SigningMethodRS256`, `RegisteredClaims`, `NewNumericDate`) is unchanged between v4 and v5.
- Code only signs tokens, never parses them — the larger v4→v5 breaking changes (Claims interface, `Valid()` removal, `Audience` type) do not apply.
- `aws-sdk-go` v1 has no direct imports in Prow source; reclassification to indirect is correct.
- `jwt/v4` remaining as indirect is expected and harmless (separate module path, no conflict).
- Unit tests, integration tests, lint all pass.

## Open questions

- Is the image build test green now?
