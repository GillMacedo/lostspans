.PHONY: test test-sdk lint demo

test:
	test -z "$$(gofmt -l plugin tests)"
	go vet ./...
	go test -race ./...

# Runs the real OpenTelemetry Go SDK. Needs network the first time.
test-sdk:
	cd tests/sdk && go vet ./... && go test ./...

lint:
	claude plugin validate ./plugin --strict
	claude plugin validate . --strict

# The broken examples exit 1 on purpose. The fixed one and the real configs
# must stay quiet.
demo:
	go run ./plugin/scripts examples/broken/checkout.yaml || test $$? -eq 1
	go run ./plugin/scripts examples/compose/compose.yaml || test $$? -eq 1
	go run ./plugin/scripts examples/fixed/checkout.yaml
	go run ./plugin/scripts tests/testdata/real
