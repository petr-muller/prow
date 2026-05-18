---
pr: kubernetes-sigs/prow#714
title: "spyglass: validate bucket in ResolveSymlink to prevent SSRF"
head_sha: 11bcd0f3212c5554dee5383ac9a7ff9d7bd43a58
base: main
reviewed_at: 2026-05-18T00:32:34Z
verdict: approve
---

## Findings

### [should-fix] No test exercises the new validation
- where: `pkg/spyglass/spyglass_test.go:1504-1513`
- concern: `TestResolveSymlink` config does not set `SkipStoragePathValidation` or `AllKnownStorageBuckets`. Default nil means `shouldValidateStorageBuckets()` returns false, making the new check a no-op in all test cases. Security fix has zero regression protection. Need at least: validation enabled + disallowed bucket -> error, and validation enabled + allowed bucket -> success.

### [nit] sg.cfg() vs sg.config() inconsistency
- where: `pkg/spyglass/spyglass.go:195`
- concern: New code uses `sg.cfg()` (promoted from embedded `*StorageArtifactFetcher`). Every other `Spyglass` method in this file uses `sg.config()` (direct field, line 61). Both return the same `config.Getter` from the same constructor param. Should use `sg.config()` for consistency.

### [question] Rejected requests surface as HTTP 500 instead of 400
- where: `cmd/deck/main.go` (renderSpyglass -> httpStatusForError)
- concern: Error from `ResolveSymlink` is wrapped in plain `fmt.Errorf` by `renderSpyglass`. `httpStatusForError` falls back to 500 when no `httpError` wrapper found. Existing `validateStoragePath` wraps bucket errors in `httpError{statusCode: http.StatusBadRequest}` returning 400. Disallowed-bucket rejections will show as 500 in monitoring. May be acceptable as follow-up.

## Checked
- Placement after alias resolution (line 191-192) and before `opener.Reader()` (line 198) is correct
- Bucket extraction via `strings.Split(key, "/")[0]` matches alias extraction pattern on line 190
- Double alias resolution (explicit line 192, then inside `ValidateStorageBucket` config.go:1334) is idempotent
- Error wrapping uses `%w`, message includes bucket name
- No deployment risk for default installations: gated by `SkipStoragePathValidation` (nil = off)
- `ResolveSymlink` was the only unguarded path to `opener.Reader()` in spyglass; `StorageArtifactFetcher` validates at line 82
- Safely rollbackable, no config migration

## Open questions
- Add test case with validation enabled (`SkipStoragePathValidation: ptr(false)`, `AllKnownStorageBuckets` populated) verifying disallowed bucket is rejected
- Change `sg.cfg()` to `sg.config()` on line 195 for file-level consistency
- HTTP 500 vs 400 for rejected requests: address here or follow-up?
