---
pr: kubernetes-sigs/prow#724
title: "chore(deps): bump github.com/go-jose/go-jose/v4 from 4.1.3 to 4.1.4 in /hack/tools"
head_sha: 8c389ad77f606482845293ce72cf1c531475a7bc
base: main
reviewed_at: 2026-05-18T18:14:27Z
verdict: approve
---

## Summary

Dependabot security bump of `go-jose/go-jose/v4` from v4.1.3 to v4.1.4 in `hack/tools/` only. Fixes CVE-2026-34986 (GHSA-78h2-9frx-2jm8), a High-severity (CVSS 7.5) DoS via panic in JWE decryption. v4.1.4 was published 2026-04-04, ~6 weeks before this review.

## Findings

### [question] root module still on v4.1.3
- where: `go.mod:1`
- concern: The root `go.mod` references `github.com/go-jose/go-jose/v4 v4.1.3 // indirect` — the vulnerable version. This PR only fixes `hack/tools`. If prow exercises the JWE decrypt path (`ParseEncrypted` + `Decrypt`) on user-controlled input at runtime, the main binary carries the vulnerability.
- excerpt: |
    github.com/go-jose/go-jose/v4 v4.1.3 // indirect

## Checked
- `hack/tools/go.mod` version pin is correct (4.1.3 → 4.1.4)
- `hack/tools/go.sum` hashes updated for both `.mod` and source entries
- No source code changes; dependency-only diff
- Security advisory: panic triggered by JWE with empty `encrypted_key` and `KW`-type algorithm — no authentication required, remote DoS

## Open questions
- Does prow's runtime use the JWE decrypt path (`ParseEncrypted` + `Decrypt`) on user-controlled input? If so, the root `go.mod` bump should follow in a separate PR.
