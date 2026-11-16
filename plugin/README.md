# lostspans plugin

This directory is the Claude Code plugin for lostspans, which checks OpenTelemetry `OTEL_` environment variables in Kubernetes manifests, Docker Compose files and `.env` files for settings that send telemetry nowhere. `skills/checking-opentelemetry-env/SKILL.md` is the skill, and `scripts/` is the Go program it builds. See the [repository README](https://github.com/GillMacedo/lostspans#readme) for examples and limits.

Install it with:

    claude plugin marketplace add GillMacedo/lostspans
    claude plugin install lostspans@lostspans-marketplace
