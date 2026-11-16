# lostspans

Claude Code skill and Go CLI that reads Kubernetes manifests, Docker Compose files and `.env` files and reports OpenTelemetry `OTEL_` settings that send traces, metrics or logs nowhere: wrong OTLP endpoint paths, invalid protocols and exporters, timeouts in seconds, malformed headers and resource attributes, ignored sampler arguments and misspelled names.

## Architecture
`plugin/scripts/` is one `package main` with no dependencies outside it. `lostspans.go` reads flags and prints the report. `input.go` reads files and refuses bad input. `sources.go` turns manifests, Compose files and env files into scopes (one per container, service or env file), expanding `$(VAR)` like kubelet and `${VAR}` like Compose. `rules.go` checks each scope. `names.go` lists the variables and values from the spec. The `yaml_*.go` files are a copy of go.yaml.in/yaml/v3 renamed to `package main`. `plugin/skills/checking-opentelemetry-env/SKILL.md` tells Claude how to use it. See `docs/architecture.md`.

## Commands
- `make test`: gofmt, vet, unit tests and the binary tests in `tests/`. No network.
- `make test-sdk`: runs the real OpenTelemetry Go SDK against each rule (`tests/sdk`, its own module). Needs network once to download the SDK.
- `make lint`: plugin manifest validation (needs Claude Code)
- `make demo`: run the examples

## Conventions
- Standard library only, plus the vendored YAML copy. No network access, ever: lostspans reads files.
- An input that can't be read completely exits 2 with nothing on stdout, under every `--fail-on`. Never skip a bad file silently. See `docs/decisions/0002-refuse-untrusted-input.md`.
- Each rule has a stable `LSnnn` ID, a message that says what the SDK will do, and a fix. Don't reuse IDs.
- Every rule cites the spec text it follows. A claim about what an SDK does needs a case in `tests/sdk` that shows it.
- Never print OTLP header values or URL passwords.
- README examples are real output. `tests/readme_test.go` runs every `$ ` line and compares.

## Do and don't
- Do run a new rule on `tests/testdata/real` and read what it reports before adding it. Those files must stay quiet.
- Don't claim behavior for an SDK other than Go's without a test or a source.

## Definition of done
`make test`, `make test-sdk` and `make lint` pass, and `README.md`, `ROADMAP.md` and the skill are updated when relevant.
