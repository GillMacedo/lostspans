Add a rule to lostspans for: $ARGUMENTS

1. Find the OpenTelemetry spec text that decides the behavior, and link it in the PR. Find a real config or bug report where it matters, and say where it came from.
2. Add it in `plugin/scripts/rules.go` with the next unused `LSnnn` ID in its group, a message that says what the SDK will do, and a fix line. Never print header values.
3. Add a case to `TestRules` in `plugin/scripts/lostspans_test.go`, plus a case that must stay quiet. If the message says what an SDK does, add a case to `tests/sdk/sdk_test.go` that shows it. Check that `tests/testdata/real` stays quiet.
4. Add the rule to the table in `README.md` and to the skill's list in `plugin/skills/checking-opentelemetry-env/SKILL.md`.
5. Run `make test` and `make test-sdk`.
