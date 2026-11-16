# 3. Rules come from the spec and are checked on the Go SDK

Date: 2026-11-16

## Decision
Each rule follows a sentence in the OpenTelemetry specification: `specification/configuration/sdk-environment-variables.md`, `specification/configuration/common.md`, `specification/protocol/exporter.md` and `specification/resource/sdk.md`. When a message says what an SDK does, it names Go, and `tests/sdk` runs the real Go SDK (autoexport v0.72.0, otel v1.47.0) to show it.

## Why
The spec leaves room: enum case is a SHOULD, gRPC endpoints may accept host:port, and malformed resource attributes SHOULD discard the whole variable. SDKs differ there. Saying what Go does, and testing it, is honest; guessing for every language isn't.

## Consequences
Severities follow the Go results. A rule can be wrong for another SDK, and the README says so. Tests for Java, Python and Node.js are on the roadmap.
