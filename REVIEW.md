---
pr: kubernetes-sigs/prow#720
title: "chore: upgrade golang-lru to v2"
head_sha: 14e201e46b1604d805f6c21827939410602a49a4
base: main
reviewed_at: 2026-05-15T15:14:59Z
verdict: approve
refresh_log:
  - previous_sha: 4b0cb59d9242d0b586a746e4ca6e3d7fbcc4a07a
    new_sha: 14e201e46b1604d805f6c21827939410602a49a4
    summary: "Author restored goldmark to v1.8.2, resolving the unrelated downgrade finding."
  - previous_sha: 14e201e46b1604d805f6c21827939410602a49a4
    new_sha: 14e201e46b1604d805f6c21827939410602a49a4
    summary: "No code changes. petr-muller /ok-to-test, stmcginnis /lgtm. PR now has lgtm label, awaiting /approve."
---

## Findings

### [nit] Unrelated formatting changes in function signatures
- where: `pkg/cache/cache.go:121-123`, `pkg/cache/cache.go:146-148`
- concern: The PR reformats multi-line function signatures to add trailing commas and move closing parens to their own line. This is valid Go style and arguably better, but it's unrelated to the golang-lru upgrade and adds noise to the diff. Minor — not worth blocking on.
- excerpt: |
    func NewLRUCache(size int,
    -	callbacks Callbacks) (*LRUCache, error) {
    +	callbacks Callbacks,
    +) (*LRUCache, error) {

## Checked
- Import path correctly changed from `golang-lru/simplelru` to `golang-lru/v2/simplelru`
- Generic type parameters `[any, any]` applied consistently to `simplelru.LRU`, `simplelru.EvictCallback`, and `simplelru.NewLRU` — correct since the cache stores heterogeneous types
- `golang-lru/v2 v2.0.7` is the latest available release; the library is mature (5.1k stars, 1261 importers), not archived or deprecated
- v1 is explicitly marked legacy by upstream ("please upgrade to v2")
- Old `golang-lru v1.0.2` entries remaining in `go.sum` are expected (transitive dependency checksums)
- Only one Go file in the repo imports golang-lru (`pkg/cache/cache.go`) — migration is complete
- `pkg/cache/cache_test.go` exists for the modified package
- No behavioral changes to the cache — `any` type params preserve the existing untyped interface

## Resolved

### [should-fix] Unrelated goldmark downgrade from v1.8.2 to v1.4.13
- where: `go.mod:59`, `go.sum:644-645`
- concern: The PR downgrades `github.com/yuin/goldmark` from v1.8.2 to v1.4.13.
- resolution: Author pushed commit `14e201e` restoring goldmark to v1.8.2.

## Open questions
- ~Were tests run against this change?~ Resolved: petr-muller issued `/ok-to-test`; CI should now be running.
