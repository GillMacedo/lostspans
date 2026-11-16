# 1. One Go program in a skill

Date: 2026-11-16

## Decision
lostspans is a Claude Code skill with one Go program in `plugin/scripts/`, standard library only, plus a copy of go.yaml.in/yaml/v3 renamed to `package main`. It isn't an MCP server.

## Why
An agent needs to run a check on files and read the result. A skill with a program does that, and the same program runs in CI. An MCP server would add a process and a protocol for no new ability. Copying the YAML parser lets the skill build with `go build` from the plugin folder, offline, with no module download.

## Consequences
The YAML copy has to be updated by hand. Its licenses are in `plugin/scripts/YAML_LICENSE` and `YAML_NOTICE`.
