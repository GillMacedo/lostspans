# 2. Refuse input it can't trust

Date: 2026-11-16

## Decision
lostspans exits 2, with nothing on stdout, when any input can't be read completely: empty files, binary data, random text, the output of a failed `helm template`, `kubectl`, `kustomize`, `docker compose`, `go build`, `go test` or `curl`, HTML pages, Kubernetes API errors, JSON or YAML that isn't a manifest or Compose file, unrendered Helm templates, files that don't end with a line break, objects without a name, workloads without a pod spec, containers or images, Jobs without a restart policy, an env entry with no value on the last line, malformed env files, missing Compose `env_file`s, and runs with no `OTEL_` variables at all. `--fail-on` doesn't change this.

## Why
A checker that prints "No problems" for a `helm template` error, or for a manifest cut in half, is worse than none. Exit 2 can't be mistaken for success in CI or by an agent.

## Limits
A file cut exactly on a line break can be a valid shorter file. Such a cut is refused when it leaves an object without its pod spec, containers or image, or an env entry without a value at the end; otherwise the shorter file is read. The summary says how many variables, containers, services and files were read, so a short read is visible. A complete file without a final line break is refused too, and the message says to add one.
