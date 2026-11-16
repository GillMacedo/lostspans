# lostspans: find OpenTelemetry OTEL_ settings that send your traces nowhere

[![CI](https://github.com/GillMacedo/lostspans/actions/workflows/ci.yml/badge.svg)](https://github.com/GillMacedo/lostspans/actions/workflows/ci.yml) [![MIT license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**A Claude Code skill and Go CLI for people who deploy services instrumented with OpenTelemetry. It reads the `OTEL_` environment variables in Kubernetes manifests, `helm template` output, Docker Compose files and `.env` files, and reports the settings that make traces, metrics or logs go nowhere: an `OTEL_EXPORTER_OTLP_ENDPOINT` ending in `/v1/traces`, a signal endpoint with no path, an endpoint with no `http://`, a protocol of `http`, a timeout of `10` (milliseconds), headers written `Authorization: Bearer ...`, a sampler ratio with no sampler, certificate paths that no Kubernetes volumeMount covers, and misspelled variable names.**

None of these fail at startup. The SDK logs one line, if any, and the service runs with no telemetry, or with telemetry the collector answers with 404. You find out when a dashboard is empty during an incident. lostspans checks the files before they ship, so it runs in CI next to your manifests, and it never connects to a cluster or a collector.

## What you can ask Claude

- "Why don't my traces show up in Jaeger? Check the OTEL settings in our manifests."
- "The collector logs 404 for /v1/traces/v1/traces. Where does that come from?"
- "Review the OpenTelemetry env vars in this docker-compose file."
- "Is OTEL_EXPORTER_OTLP_TIMEOUT in seconds or milliseconds?"
- "Check the rendered Helm chart for OTLP endpoint mistakes before I merge."

## Install

As a Claude Code plugin:

    claude plugin marketplace add GillMacedo/lostspans
    claude plugin install lostspans@lostspans-marketplace

Or run it with Go 1.22 or newer. It has no dependencies outside this repository:

    helm template my-release ./chart > rendered.yaml
    go run github.com/GillMacedo/lostspans/plugin/scripts@v0.1.0 rendered.yaml

## Example

`examples/broken/checkout.yaml` is a made-up checkout Deployment and CronJob with settings people really write. `--fail-on never` keeps the exit code at 0 for this demo; without it, the run exits 1:

    $ go run ./plugin/scripts --fail-on never examples/broken/checkout.yaml
    HIGH   LS101 Deployment checkout, container checkout (examples/broken/checkout.yaml:8)
      OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector.observability:4318/v1/traces
      SDKs add /v1/<signal> to this base URL, so they send traces to http://otel-collector.observability:4318/v1/traces/v1/traces, metrics to http://otel-collector.observability:4318/v1/traces/v1/metrics and logs to http://otel-collector.observability:4318/v1/traces/v1/logs. The collector answers 404.
      fix: remove /v1/traces from the URL, or set OTEL_EXPORTER_OTLP_TRACES_ENDPOINT to this URL instead

    HIGH   LS501 Deployment checkout, container checkout (examples/broken/checkout.yaml:42)
      OTEL_EXPORTER_OTLP_HEADERS=(values hidden; keys: x-api-key)
      The x-api-key entry uses : instead of =, so SDKs can't read it. Go's SDK skips it and logs an error, so the request goes out without that header, and a backend that needs it rejects the data.
      fix: write each header as key=value, for example Authorization=Bearer%20TOKEN

    MEDIUM LS401 Deployment checkout, container checkout (examples/broken/checkout.yaml:10)
      OTEL_EXPORTER_OTLP_TIMEOUT=10
      This is 10 milliseconds, not seconds. Any export slower than that is cancelled and counted as failed, so data is retried or dropped whenever the collector is slow or far away.
      fix: use 10000 for 10 seconds

    MEDIUM LS303 Deployment checkout, container checkout (examples/broken/checkout.yaml:11)
      OTEL_TRACES_SAMPLER_ARG=0.25
      OTEL_TRACES_SAMPLER isn't set, so SDKs ignore this argument and sample every new trace.
      fix: also set OTEL_TRACES_SAMPLER=parentbased_traceidratio

    MEDIUM LS504 Deployment checkout, container checkout (examples/broken/checkout.yaml:35)
      OTEL_RESOURCE_ATTRIBUTES=deployment.environment=prod;team=payments
      Entries are separated by commas. With ;, the text after it becomes part of the previous resource attribute's value, and the resource attributes after it are lost.
      fix: separate entries with commas: key1=value1,key2=value2

    HIGH   LS102 CronJob invoice-sync, container sync (examples/broken/checkout.yaml:62)
      OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:4318
      SDKs use OTEL_EXPORTER_OTLP_TRACES_ENDPOINT as written, with no path added, so traces go to http://localhost:4318/. The collector answers 404 there.
      fix: add the path: http://localhost:4318/v1/traces, or set OTEL_EXPORTER_OTLP_ENDPOINT instead, which gets the path added

    MEDIUM LS107 CronJob invoice-sync, container sync (examples/broken/checkout.yaml:62)
      OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:4318
      localhost here means this pod itself, and nothing else in it looks like a collector, so exports have nowhere to go.
      fix: use the collector's service name, such as http://otel-collector:4318

    MEDIUM LS601 CronJob invoice-sync, container sync (examples/broken/checkout.yaml:64)
      OTEL_EXPORTER_OTLP_METRIC_ENDPOINT=http://otel-collector.observability:4318/v1/metrics
      OTEL_EXPORTER_OTLP_METRIC_ENDPOINT isn't an OpenTelemetry SDK variable, so SDKs ignore it. It looks like a typo of OTEL_EXPORTER_OTLP_METRICS_ENDPOINT.
      fix: rename it to OTEL_EXPORTER_OTLP_METRICS_ENDPOINT

    3 high, 5 medium, 0 low: checked 10 OTEL_ variables in 2 containers, from 1 file.
    Not checked, because the value isn't in the files: 1 built from a variable whose value isn't in the files. --format json lists them.

The same file with each mistake fixed stays quiet:

    $ go run ./plugin/scripts examples/fixed/checkout.yaml
    No problems: checked 11 OTEL_ variables in 2 containers, from 1 file.
    Not checked, because the value isn't in the files: 1 built from a variable whose value isn't in the files. --format json lists them.

A Compose service reads its settings from `environment` and from an `env_file`, and lostspans points at the file and line where each one is set:

    $ go run ./plugin/scripts --fail-on never examples/compose/compose.yaml
    HIGH   LS104 Compose service api (examples/compose/otel.env:1)
      OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318
      This isn't a URL that starts with http:// or https://. Go's SDK can't parse it and sends to its default, https://localhost:4318, instead.
      fix: write the full URL, such as http://otel-collector:4318

    HIGH   LS103 Compose service api (examples/compose/compose.yaml:9)
      OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://otel-collector:4318/v1/metrics
      This sends traces to the metrics path. The collector rejects them there.
      fix: use the /v1/traces path

    MEDIUM LS202 Compose service api (examples/compose/otel.env:2)
      OTEL_EXPORTER_OTLP_PROTOCOL=HTTP/PROTOBUF
      The spec says SDKs should accept any case, but Go's autoexport rejects "HTTP/PROTOBUF" and creates no exporter.
      fix: write it in lower case: http/protobuf

    MEDIUM LS403 Compose service api (examples/compose/otel.env:3)
      OTEL_METRIC_EXPORT_INTERVAL=60
      This is 60 milliseconds, so metrics are exported about 17 times per second.
      fix: use 60000 for 60 seconds

    2 high, 2 medium, 0 low: checked 5 OTEL_ variables in 1 Compose service, from 1 file.

The exit code tells CI what happened:

    $ go build -o /tmp/lostspans ./plugin/scripts && /tmp/lostspans examples/broken/checkout.yaml > /dev/null; echo "exit $?"
    exit 1

The real configs in `tests/testdata/real`, from the OpenTelemetry Demo and Jaeger, stay quiet:

    $ go run ./plugin/scripts tests/testdata/real
    No problems: checked 280 OTEL_ variables in 23 containers, 19 Compose services and 1 env file, from 4 files.
    Not checked, because the value isn't in the files: 23 set from valueFrom. --format json lists them.

Add `--format json` for machine-readable output, with the file, line, scope, variable, message and fix of each finding, and every value that wasn't checked.

## What it reports

| Rule | Severity | Finds |
|---|---|---|
| `LS101` | High | `OTEL_EXPORTER_OTLP_ENDPOINT` ending in `/v1/traces`, `/v1/metrics` or `/v1/logs`. SDKs add the signal path again |
| `LS102` | High | `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` (or metrics, logs) with no path. SDKs use it as written, so data goes to `/` |
| `LS103` | High | A signal endpoint with another signal's path, such as traces sent to `/v1/metrics` |
| `LS104` | High | An endpoint that isn't an `http://` or `https://` URL. Go's SDK sends to `https://localhost:4318` instead. Medium when the protocol is `grpc` |
| `LS105` | Medium | HTTP on port 4317 or gRPC on port 4318, the other protocol's default port |
| `LS106` | Low | An `http://` endpoint with no port, which means port 80 |
| `LS107` | Medium | `localhost` in a pod or Compose service where nothing else looks like a collector |
| `LS108` | Medium | An absolute certificate, client key or client certificate path that no `volumeMount` in the same Kubernetes container covers |
| `LS201` | High | A protocol that isn't `grpc`, `http/protobuf` or `http/json`, such as `http` |
| `LS202` | Medium | A protocol or exporter in upper case. Go's autoexport rejects it |
| `LS203` | High, Medium or Low | The `jaeger` exporter (removed from the spec, rejected by Go), an unknown exporter, or the deprecated `logging` |
| `LS204` | Medium | An unknown sampler. SDKs fall back to sampling every trace |
| `LS205` | Medium | An unknown propagator |
| `LS206` | Medium | Compression other than `gzip` or `none` |
| `LS301` | High | `traceidratio` with a ratio of 0, which samples nothing |
| `LS302` | Medium | A ratio outside 0 to 1, such as `10` for 10%. SDKs ignore it |
| `LS303` | Medium | `OTEL_TRACES_SAMPLER_ARG` with no `OTEL_TRACES_SAMPLER`. SDKs ignore the argument and sample every trace |
| `LS304` | Low | `always_off` or `parentbased_always_off` |
| `LS401` | Medium | A timeout under 100, which is milliseconds, usually meant as seconds |
| `LS402` | Medium | A duration or number that isn't a whole number, such as `10s`. SDKs ignore it |
| `LS403` | Medium | `OTEL_METRIC_EXPORT_INTERVAL` under 1000 milliseconds |
| `LS404` | Low | A batch size larger than the queue size |
| `LS501` | High | An OTLP header that isn't `key=value`, such as `Authorization: Bearer ...`. Go's SDK skips it, so the request goes out without it |
| `LS502` | Medium | Headers separated by `;` |
| `LS503` | Medium | A resource attribute that isn't `key=value` |
| `LS504` | Medium | Resource attributes separated by `;` |
| `LS505` | Low | A `%` that isn't valid percent-encoding in headers or resource attributes |
| `LS601` | Medium | A name one or two letters away from a spec variable, such as `OTEL_EXPORTER_OTLP_TRACE_ENDPOINT` |
| `LS602` | Medium | A boolean other than `true` or `false`, such as `1`, which the spec says is false |
| `LS603` | Medium or Low | An empty value, which counts as unset. Medium for endpoints, which then fall back to localhost |

Header values and passwords in endpoint URLs are never printed. Names with a language or project prefix (`OTEL_GO_`, `OTEL_JAVA_`, `OTEL_PYTHON_`, `OTEL_INSTRUMENTATION_` and others), and project variables that aren't close to a spec name, aren't reported.

## The rules it follows

Each rule follows the OpenTelemetry specification: `configuration/sdk-environment-variables.md` for names, types, enum values and empty values, `configuration/common.md` for durations and timeouts in milliseconds, `protocol/exporter.md` for how OTLP/HTTP builds URLs from base and signal endpoints, how headers are written, and which certificate and key file paths the exporter accepts, and `resource/sdk.md` for resource attributes.

Where the spec leaves room, the message says what Go's SDK does, and `tests/sdk` shows it. That test runs the real OpenTelemetry Go SDK (`go.opentelemetry.io/contrib/exporters/autoexport` v0.72.0, `go.opentelemetry.io/otel/sdk` v1.47.0) against a local OTLP/HTTP server for 20 configurations. For each broken one, the SDK does what the rule's message says, and lostspans reports the rule. For each healthy one, the span arrives and lostspans is quiet.

## What it reads

- **Kubernetes:** YAML or JSON with one or more objects, `kind: List` from `kubectl get -o yaml`, and `helm template` or `kubectl kustomize` output. Pod, Deployment, StatefulSet, DaemonSet, ReplicaSet, ReplicationController, Job, CronJob and PodTemplate, plus any kind with `spec.template.spec.containers`, such as an Argo Rollout. Containers and init containers. `envFrom` ConfigMaps in the inputs (with `prefix`), then `env`, with `$(VAR)` expanded from variables set earlier, as kubelet does. ConfigMaps no container loads are checked on their own.
- **Docker Compose:** `services` with `environment` (map or list) and `env_file`, with `${VAR}`, `${VAR:-default}` and similar read from the `.env` next to the Compose file. Never from your shell.
- **Env files:** `.env`, `.env.production`, `app.env` and similar, `KEY=VALUE` per line.
- Directories (walked for `.yaml`, `.yml`, `.json` and env files) and `-` for stdin.

Values from `valueFrom`, Secrets, ConfigMaps that aren't in the inputs, and the shell that runs Compose can't be checked. The summary counts them and `--format json` lists them. Headers built from a Secret are still checked for shape, as `x-api-key: $(VENDOR_KEY)` is in the example.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Nothing at or above `--fail-on`, which is `high` by default. Use `medium`, `low` or `never`. |
| 1 | Findings at or above `--fail-on`. |
| 2 | An input can't be trusted or the flags are wrong. Nothing was checked, and stdout is empty. |

## When it refuses

lostspans exits 2 and prints only the reason when it can't trust what it read, whatever `--fail-on` says. For example, when `helm template` failed and its error went into the file:

    $ go build -o /tmp/lostspans ./plugin/scripts && /tmp/lostspans tests/testdata/bad/helm-template-error.txt 2>&1; echo "exit $?"
    lostspans: can't trust the input, nothing was checked: tests/testdata/bad/helm-template-error.txt: this looks like the output of a failed command, not config: "Error: INSTALLATION FAILED: execution error at (shop/templates/deployment.yaml:41:22): otel.endpoint is required"
    exit 2

It does the same for an empty file, comments only, random text, binary data, the output of a failed `helm template`, `kubectl`, `kubectl kustomize`, `docker compose`, `go build`, `go test` or `curl`, an HTML error page, a Kubernetes API error (`kind: Status`), JSON or YAML that isn't a manifest or Compose file, an unrendered Helm template, an empty `kind: List`, a missing file, invalid YAML, a malformed env file, a Compose `env_file` that doesn't exist, and a file cut off partway. A file must end with a line break, every object needs `apiVersion`, `kind` and `metadata.name`, a workload needs a pod spec with containers that have a name and an image, and a Job needs a `restartPolicy`. One bad file among good ones fails the whole run, and a run with no `OTEL_` variables at all is refused as having nothing to check. In a directory, valid YAML that isn't config, such as Helm values or a workflow, and Helm templates are skipped and listed.

A file cut exactly on a line break can still be a valid, shorter file, and no tool can tell it apart from a complete one. Many such cuts leave an object without its pod spec, containers or image, or an env entry without a value, and are refused. The rest are read as the shorter file. Check the counts in the summary line against what you expect.

## How it compares

- **The collector's own checks** (`otelcol validate`, and collector config linters) check the collector's config file, not the SDK settings in your services.
- **Manifest linters** such as kube-linter, kubeconform and kube-score check Kubernetes fields and treat env values as plain text.
- **SDK doctors**, such as the Node.js `otel-doctor` package, check one running project's packages and setup from inside it.

lostspans checks the SDK settings themselves, across every service in your deploy files, before anything runs.

## Limits

- **Settings in code aren't seen.** Many services set the endpoint or exporter in code, which can override the environment or ignore it. lostspans reads only `OTEL_` variables.
- **SDKs differ.** The rules follow the spec, and messages about SDK behavior describe Go's SDK, which is the one tested. Java, Python, .NET and Node.js may accept or reject different values, for example upper-case enum values or `host:port` gRPC endpoints.
- **Defaults differ.** When no protocol is set, Go's autoexport sends OTLP/HTTP, and the spec lets other SDKs default to gRPC, so an endpoint on port 4317 with no protocol isn't reported.
- **It doesn't check the collector.** It can't tell whether a host resolves, a port is open, or the collector has an OTLP receiver.
- **`OTEL_CONFIG_FILE`** makes SDKs that support it ignore the other variables. lostspans still checks them.
- **The OpenTelemetry Operator's `Instrumentation` resource** isn't read yet. It's on the roadmap.

## FAQ

**Why don't my traces show up, with no error?** OTLP exporters log a failure once, if at all, and the service keeps running. Common causes are an endpoint with the wrong path, a protocol value SDKs don't accept, or headers they can't parse. lostspans names the variable and line.

**Should OTEL_EXPORTER_OTLP_ENDPOINT include /v1/traces?** No. SDKs add `/v1/traces`, `/v1/metrics` and `/v1/logs` to it. Only the signal variables, such as `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, take the full path, and they're used as written.

**Why does the collector log 404 for /v1/traces/v1/traces?** The base endpoint already ends in `/v1/traces`. That's `LS101`.

**Is OTEL_EXPORTER_OTLP_TIMEOUT in seconds?** No, milliseconds. So are `OTEL_BSP_SCHEDULE_DELAY`, `OTEL_BSP_EXPORT_TIMEOUT` and `OTEL_METRIC_EXPORT_INTERVAL`. A value with a unit, such as `10s`, is ignored.

**How do I write OTEL_EXPORTER_OTLP_HEADERS?** `key=value,key2=value2`, with spaces percent-encoded: `Authorization=Bearer%20TOKEN`. A colon instead of `=` makes the entry unreadable.

**Is OTEL_EXPORTER_OTLP_PROTOCOL=http valid?** No. The values are `grpc`, `http/protobuf` and `http/json`.

**Does it need a cluster or a collector?** No. It reads files: manifests, Compose files, env files, or `kubectl get -o yaml` output you saved.

**Is it an MCP server?** No. It's a skill with one Go program, so `go run` or `go build` is all it needs.

## Tested on

- **The real Go SDK:** `tests/sdk` runs 20 configurations through the OpenTelemetry Go SDK (autoexport v0.72.0, otel v1.47.0) against a local server, as described above. For every broken one, the SDK does what the rule says: it loses or misroutes the span, sends the request without a header, fails the export, or ignores the setting. Every healthy one delivers the span, and lostspans is quiet on it.
- **Real configs that must stay quiet:** `tests/testdata/real` holds the OpenTelemetry Demo's `compose.yaml` and `.env` (opentelemetry-demo at `b825c7c25726`), the demo's rendered Kubernetes manifest from opentelemetry-helm-charts (`c5297df1ff35`), and Jaeger's HotROD `docker-compose.yml` (`55808f12d225`). That's 280 `OTEL_` variables in 23 containers, 19 Compose services and 1 env file, and all are quiet. The run is at the end of the Example section.
  By hand, not in `make test`: Grafana's intro-to-mltp `docker-compose.yml` (6 variables in 3 services) is quiet too. It's AGPL-3.0, so it isn't in this repository.
- **A real bug:** `johnib/kusto-mcp` issue 298 reports that its docs show `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317/v1/traces` while its exporters add `/v1/traces` and speak HTTP. An env file with that line reports `LS101` and `LS105`.
- **Input it can't trust:** 35 bad files in `tests/testdata/bad` are each refused alone, under every `--fail-on` and both formats, with nothing on stdout. All but one are also refused next to a good file in either order; the exception is a valid Deployment with no `OTEL_` variables, refused alone only because there's nothing to check. The ones that aren't env files are refused on stdin too. In a directory next to a good file, 29 are refused, 5 are skipped and listed because they are valid YAML that isn't config (Helm values, a workflow, `package.json`, comments only) or a Helm template, and the Deployment with no `OTEL_` variables is read. A Compose project with a bad `env_file` or `.env`, each of six kinds, is refused. Other tests cut seven files at every line break and at regular byte offsets: every mid-line cut is refused, and line cuts are refused or read as shorter files.
- **Go versions:** `make test` passes with Go 1.22.12 and Go 1.27.3 on Linux arm64, offline. CI runs it on Linux amd64 with Go 1.22 and the current release.

Not tested: SDKs other than Go's, so messages about Java, Python, .NET and Node.js come from the spec, not from running them; a real collector or backend (the test server stands in for an OTLP/HTTP receiver); OTLP over gRPC, so `LS105` comes from the default ports in the spec; a live cluster; macOS and Windows; a fresh Claude Code session using the skill. The `go run ...@v0.1.0` line works only once the repository is published.

## Contributing

Contributions are welcome, from people and from people working with AI agents. See [CONTRIBUTING.md](CONTRIBUTING.md). Good first issues are [listed here](https://github.com/GillMacedo/lostspans/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

### How pull requests work

1. **Open a PR.** Fork the repo, work on a branch, and keep it to one change. Write a title that says what changes for users, because it becomes the changelog line: `feat:` goes under Added and `fix:` under Fixed.
2. **Automatic check.** Code changes need tests, and `make test` must pass. Changes over 30 lines need a linked open issue (`Closes #<number>`), and a PR can have at most 800 changed lines. If something is missing, a comment tells you what to fix.
3. **Review.** Claude reviews each PR automatically. Code PRs also get a security review and are tested in a sandbox. The review then merges the PR or asks for changes, and a small problem may become a follow-up issue instead. A PR with no new commit 14 days after a change request is closed, and you can reopen it. PRs that change CI, agent settings, npm or Python dependencies, vendored code or build files can't be merged this way, and a comment says so. The same goes for anything the security review doesn't clear.
4. **Release.** A merge that changes what users install becomes a new version within about two hours, with a GitHub Release and, if the project has one, its npm package. Docs and CI changes wait for the next release.

## AI disclosure

This project was largely generated with Claude Code and reviewed by its maintainer.

## About me

I'm Giulliano Macedo, and I build developer tools at GoTech Solutions. Follow along or get in touch:

- GitHub: [GillMacedo](https://github.com/GillMacedo)
- LinkedIn: [giulliano](https://www.linkedin.com/in/giulliano/)
- X: [@giumacedo](https://x.com/giumacedo)

If `lostspans` found where your spans went, a star helps others find it.

---

Built by [Giulliano Macedo](https://github.com/GillMacedo), GoTech Solutions · [LinkedIn](https://www.linkedin.com/in/giulliano/) · [X](https://x.com/giumacedo)
