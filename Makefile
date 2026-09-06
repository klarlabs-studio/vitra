.PHONY: build build-native test vet fmt fmt-check lint cover security check install-hooks demo

GO       := go
LINT     := golangci-lint
NOX      := nox
COVERCTL := coverctl
COVERAGE := coverage.out

build:
	$(GO) build ./...

build-native:
	CGO_ENABLED=1 $(GO) build -tags vitra_native ./...

test:
	$(GO) test ./... -count=1

demo:
	CGO_ENABLED=1 VITRA_DEMO_SECONDS=3 xvfb-run -a $(GO) run -tags vitra_native ./example/competitive

vet:
	$(GO) vet ./...

fmt:
	$(LINT) fmt ./...

fmt-check:
	@test -z "$$($(LINT) fmt --diff ./...)" || (echo "Files need formatting. Run 'make fmt' to fix." && $(LINT) fmt --diff ./... && exit 1)

lint:
	$(LINT) run ./...

cover:
	$(GO) test ./... -coverprofile=$(COVERAGE) -count=1
	$(GO) tool cover -func=$(COVERAGE)
	$(COVERCTL) check --profile=$(COVERAGE) --from-profile --ratchet
	$(COVERCTL) record --profile=$(COVERAGE)

security:
	$(NOX) scan .

check: fmt lint test security
	@echo "All checks passed."

install-hooks:
	@echo "Installing git hooks..."
	@mkdir -p .git/hooks
	@cp scripts/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Done. Pre-commit hook installed."
