---
pr: kubernetes-sigs/prow#727
title: "chore(deps): bump go.opentelemetry.io/otel/sdk from 1.40.0 to 1.43.0"
head_sha: e35fb0653f91ad07f04724b6ff04faafd879426b
base: main
reviewed_at: 2026-05-19T20:50:02Z
verdict: approve
---

## Summary

Dependabot bump of the OpenTelemetry Go SDK package family from inconsistent versions (1.40.0 for sdk/sdk/metric, 1.41.0 for otel/metric/trace) to a consistent 1.43.0 across all five packages. All are indirect dependencies; prow has no direct OTel API usage. Changes are purely mechanical: version strings in go.mod and SHA hashes in go.sum.

## Findings

No findings.

## Checked

- All five otel packages updated to consistent 1.43.0 (previously split across 1.40.0 and 1.41.0)
- go.sum has correct hash replacements for all 10 entries (h1 + go.mod hashes per package)
- All packages are `// indirect` in go.mod — prow has no direct OTel API calls (confirmed by grep)
- No usage of newly deprecated `attribute.INVALID` in the codebase
- v1.43.0 tag was created 2026-04-03T08:30:48Z in open-telemetry/opentelemetry-go
- Notable upstream fixes included: race condition in sdk/metric lastvalue aggregation, 4 MiB HTTP response body cap in OTLP HTTP exporters (security), missing GetBody in otlploghttp for HTTP2 GOAWAY recovery
- Breaking behavioral change in TraceIdRatioBased description (spec compliance) — no impact since prow has no direct OTel usage

## Open questions

None.
