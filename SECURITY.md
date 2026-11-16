# Security

Report vulnerabilities through [GitHub's private vulnerability reporting](https://github.com/GillMacedo/lostspans/security/advisories/new), not in a public issue.

A case where `lostspans` gets the OpenTelemetry spec or an SDK's behavior wrong is a normal bug. Report it privately if it prints a secret, such as an OTLP header value or a password in an endpoint URL, or if it could mislead someone into an unsafe setting. `lostspans` reads files only, never sends anything anywhere, and doesn't enforce anything.
