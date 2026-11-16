# Architecture

lostspans is one Go `package main` in `plugin/scripts/`, so the skill can build it with `go build` and nothing else. It reads files, builds a list of scopes, checks each scope, and prints a report.

## Flow

1. **Read** (`input.go`, `readArgs`): files, directories (walked for `.yaml`, `.yml`, `.json` and env files, skipping hidden folders, `vendor` and `node_modules`) and `-` for stdin.
2. **Refuse** (`input.go`, `checkBytes`, and `sources.go`, `load`): an empty file, binary data, the output of a failed command, a file without a final line break, invalid YAML, prose, an unrendered Helm template, a Kubernetes object without `apiVersion`, `kind` or `metadata.name`, a workload without a pod spec, containers, names or images, a Job without a valid `restartPolicy`, an env entry with no value on the last line, a malformed env file, or a missing Compose `env_file` all stop the run with exit 2. In a directory walk, valid YAML that isn't a manifest or Compose file (Helm values, workflows, `package.json`) and Helm templates are skipped and listed instead.
3. **Scopes** (`sources.go`): one per Kubernetes container (init containers included), Compose service, env file, and ConfigMap that no container in the inputs loads. Each scope holds its `OTEL_` variables in the order they're set, so later ones win.
   - Kubernetes: `envFrom` ConfigMaps from the inputs (with `prefix`), then `env`, with `$(VAR)` expanded from earlier variables the way kubelet does. `valueFrom`, Secrets, and ConfigMaps not in the inputs are listed as not checked.
   - Compose: `env_file`, then `environment`, with `${VAR}` interpolated from the `.env` next to the Compose file, never from the shell running lostspans.
4. **Check** (`rules.go`, `check`): name checks, value checks per variable, then rules that combine variables: which endpoint each signal uses and with which protocol, the sampler and its argument, and batch sizes.
5. **Report** (`lostspans.go`): text or JSON, and the exit code from `--fail-on`.

## Why the endpoint rules look at protocol and port

The OTLP/HTTP exporter adds `v1/traces`, `v1/metrics` or `v1/logs` to `OTEL_EXPORTER_OTLP_ENDPOINT`, and uses signal endpoints as written. gRPC exporters don't use the path. So the path rules apply only when the endpoint is used over HTTP: the protocol says so, or no protocol is set and either the port isn't 4317 or the URL has an OTLP/HTTP path.

## Tests

- `plugin/scripts/lostspans_test.go`: each rule, with cases that must stay quiet, and the parsers.
- `tests/`: the built binary on the examples, on real configs that must stay quiet, on 35 bad inputs, and on files cut at every line break and at byte offsets.
- `tests/sdk/`: a separate module that runs the real OpenTelemetry Go SDK with each configuration and checks that lostspans reports exactly the ones where the SDK loses or misroutes telemetry.
