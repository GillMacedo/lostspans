---
name: checking-opentelemetry-env
description: Checks OpenTelemetry SDK settings (OTEL_ environment variables) in Kubernetes manifests, helm template output, Docker Compose files and .env files for values that make traces, metrics or logs go nowhere without an error. Finds OTEL_EXPORTER_OTLP_ENDPOINT with a /v1/traces path (sent to /v1/traces/v1/traces), signal endpoints without a path, endpoints without http://, invalid or upper-case protocols, the removed jaeger exporter, timeouts written in seconds, malformed OTLP headers and resource attributes, sampler arguments that are ignored, localhost in a pod with no collector, certificate paths not covered by a volumeMount, and misspelled variable names. Use when traces or metrics don't show up in the backend, the collector logs 404s, after changing OTEL_ settings, or before merging deploy config.
---

# Checking OpenTelemetry env settings

lostspans reads files only. It needs Go 1.22 or newer to build, and never connects to a cluster, a collector or a backend. Build it once:

    go build -o "${TMPDIR:-/tmp}/lostspans" $(ls "${CLAUDE_PLUGIN_ROOT}"/scripts/*.go | grep -v _test.go)

## Get the files

Ask before running anything that touches a cluster. Any of these works:

- Manifests, Compose files or `.env` files in the repository: pass the files or a directory.
- A Helm chart: `helm template RELEASE CHART -f values.yaml > rendered.yaml`. Check that helm exited 0 first.
- Kustomize: `kubectl kustomize DIR > rendered.yaml`.
- What runs now: `kubectl get deploy,sts,ds,job,cronjob,configmap -n NS -o yaml > live.yaml`.

Pass ConfigMaps that containers load with `envFrom`, and the `.env` next to a Compose file, so their values are checked in context. Secrets are never read.

## Run

    "${TMPDIR:-/tmp}/lostspans" rendered.yaml

Exit 0 means no findings at or above `--fail-on` (default `high`), 1 means findings, and 2 means an input couldn't be trusted and nothing was checked. On exit 2, show the user the reason and fix the input. Never report exit 2 as clean. Add `--format json` for machine-readable output, including every value that wasn't checked.

## Read the findings

- `LS101` to `LS107`, endpoints: a base URL with a `/v1/<signal>` path, a signal URL without one, a URL without `http://`, a protocol on the other protocol's default port, no port, or `localhost` in a pod with no collector.
- `LS108`, certificate paths: an absolute `OTEL_EXPORTER_OTLP_CERTIFICATE`, `OTEL_EXPORTER_OTLP_CLIENT_KEY` or `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` path that no `volumeMount` in the same Kubernetes container covers.
- `LS201` to `LS206`, names of protocols, exporters, samplers, propagators and compression that SDKs don't accept. Go's autoexport also rejects upper case.
- `LS301` to `LS304`, sampling: ratio 0, a ratio outside 0 to 1, an argument with no sampler set, or a sampler that records nothing.
- `LS401` to `LS404`, numbers: timeouts under 100 ms (usually meant as seconds), values with units, a metric interval under a second, a batch larger than the queue.
- `LS501` to `LS505`, headers and resource attributes that aren't `key=value,key=value`.
- `LS601` to `LS603`, a misspelled variable name, a boolean other than true or false, an empty value.

Each finding says what the SDK will do and gives a fix. Several say what Go's SDK does, because that's the SDK the rules were tested against; say so when the service uses another language.

## Report

1. Group findings by workload and container, worst first, with the file and line, what happens to the telemetry, and the smallest fix.
2. Say what wasn't checked: values from `valueFrom`, Secrets, ConfigMaps not in the inputs, or the shell that runs Compose. The summary line and `--format json` list them.

## Don't

Don't edit manifests unless the user asks. Don't print header values, even from the files: they often hold API keys. Don't call telemetry working because lostspans is quiet. It checks settings, not whether the collector is up, reachable or configured to receive them, and it doesn't read settings made in code.
