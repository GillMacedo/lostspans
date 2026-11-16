# Test data

## real/
Unchanged files from public projects, each with its project's license next to it. lostspans must stay quiet on all of them.

| File | Source | License |
|---|---|---|
| `opentelemetry-demo/compose.yaml`, `.env` | open-telemetry/opentelemetry-demo at `b825c7c25726` | Apache-2.0 |
| `opentelemetry-helm-charts/opentelemetry-demo-rendered.yaml` | open-telemetry/opentelemetry-helm-charts at `c5297df1ff35`, `charts/opentelemetry-demo/examples/default/rendered/component.yaml` | Apache-2.0 |
| `jaeger-hotrod/docker-compose.yml` | jaegertracing/jaeger at `55808f12d225`, `examples/hotrod/docker-compose.yml` | Apache-2.0 |

The demo's `.env` holds the demo's own published default passwords, unchanged.

## bad/
Inputs lostspans must refuse with exit 2. `go-build-error.txt` and `go-test-fail.txt` are real output of `go build` and `go test` on a broken package, and `github-404.yaml` is what `curl -s` saved for a missing raw GitHub file. The Helm, kubectl, kustomize and Compose error texts were written to match those tools' messages. The rest were written for these tests.
