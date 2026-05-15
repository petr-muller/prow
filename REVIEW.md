---
pr: kubernetes-sigs/prow#715
title: "buildlog: fail gracefully on large build logs"
head_sha: a67ffd4f8766b65bf045f825a04d8351efc42463
base: main
reviewed_at: 2026-05-14T11:02:10Z
verdict: approve
refresh_log:
  - old_sha: dccb2344479f601c873df34072271f538d24ff90
    new_sha: a67ffd4f8766b65bf045f825a04d8351efc42463
    summary: "Author addressed Size() error handling, added CSS for .log-error and .log-warning, added UI warning banner, added TestBodySizeErrorDisablesHighlight"
---

## Findings

### Resolved

#### ~~[should-fix] Size() error silently allows highlighting~~
- where: `pkg/spyglass/lenses/buildlog/lens.go:240-243`
- resolution: Size() error now logs a `logrus.Warn` and disables highlighting conservatively. New test `TestBodySizeErrorDisablesHighlight` covers this path.

#### ~~[should-fix] Missing CSS for .log-error~~
- where: `pkg/spyglass/lenses/buildlog/buildlog.css:139-147`
- resolution: Added `.log-error` (red) and `.log-warning` (yellow) CSS rules.

### Open

### [should-fix] Error message stutter
- where: `pkg/spyglass/lenses/buildlog/lens.go:230`
- concern: Outer `fmt.Sprintf("Failed to read log: %v", err)` wraps inner `fmt.Errorf("failed to read log %q: %w", ...)` from `logLinesAll` (line 493). User sees "failed to read log" twice.
- excerpt: |
    av.Error = fmt.Sprintf("Failed to read log: %v", err)

### [nit] Callback path does not apply size guard
- where: `pkg/spyglass/lenses/buildlog/lens.go:452,469`
- concern: `loadLines` always passes `conf.highlightRegex` directly. Expanding collapsed sections in a large log still triggers regex matching. Lower practical risk (chunks, not full log) but inconsistent with `Body()` behavior.
- excerpt: |
    logLines := highlightLines(skipLines, skipRequest.StartLine, &request.Artifact, conf.highlightRegex, conf.highlightLengthMax)

### [nit] Test does not directly verify highlighting is skipped
- where: `pkg/spyglass/lenses/buildlog/lens_test.go:1186`
- concern: Asserts `"log-error"` is absent and warning is present, but does not check `"match-highlighted"` is absent. Does not directly test the intended behavior.

### [nit] No log line when size threshold triggers
- where: `pkg/spyglass/lenses/buildlog/lens.go:244-246`
- concern: The Size() error path now logs (resolved), but the normal `sz > highlightSizeThreshold` path at line 244 still has no server-side log. A `logrus.Info` with artifact name and size would help operators.

## Checked
- XSS safety: html/template auto-escapes {{.Error}} and {{.Warning}}
- Zero-value backwards compat: Error and Warning default to "", template {{if .Error}}/{{if .Warning}} skip for successful artifacts
- No config/API/deployment impact: LogArtifactView is internal template struct
- Test coverage: all new paths tested, errArtifact helper extended with sizeErr field
- Constant placement and naming: highlightSizeThreshold is clear
- Existing tests pass

## Open questions
- Is the Callback path omission intentional for this PR scope?
- The warning message hardcodes "10 MiB" — would it be better to derive from the constant?

## Since previous review

- Author force-pushed addressing review feedback from @elmiko and this review.
- Size() error path now disables highlighting conservatively and logs a warning (resolves main [should-fix]).
- Added `.log-error` and `.log-warning` CSS rules in `buildlog.css` (resolves missing CSS finding).
- Added a `Warning` field to `LogArtifactView` and renders a yellow warning banner in the UI when highlighting is skipped, with two messages: "log exceeds 10 MiB" or "unable to determine log size".
- New test `TestBodySizeErrorDisablesHighlight` and extended assertions in `TestBodyLargeLogSkipsHighlight`.
