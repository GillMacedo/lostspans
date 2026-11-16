# Changelog

All notable changes are documented here, in [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. This project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-11-16

### Added
- Check `OTEL_` variables for settings that send telemetry nowhere: OTLP endpoint paths, schemes, ports and `localhost` (`LS101` to `LS107`), protocol, exporter, sampler, propagator and compression names (`LS201` to `LS206`), sampling (`LS301` to `LS304`), durations and sizes (`LS401` to `LS404`), headers and resource attributes (`LS501` to `LS505`), and misspelled names, booleans and empty values (`LS601` to `LS603`).
- Kubernetes manifests (Pods, Deployments, StatefulSets, DaemonSets, ReplicaSets, Jobs, CronJobs and kinds with `spec.template.spec`), with `envFrom` ConfigMaps and kubelet's `$(VAR)` expansion.
- Docker Compose files, with `env_file` and `${VAR}` interpolation from the `.env` next to them, and `.env` files.
- `--fail-on`, `--format json`, directories and stdin.
- Exit 2, with nothing checked, for input that can't be trusted: empty, garbage, cut off, unrendered Helm templates, or the output of a failed command.
- Header values and URL passwords are never printed.
- `checking-opentelemetry-env` skill and plugin manifest.
