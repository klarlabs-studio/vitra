.PHONY: build build-native test test-native vet fmt fmt-check lint cover security check install-hooks demo e2e e2e-notes notes

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
	CGO_ENABLED=1 $(GO) build -tags vitra_native -o .bin/competitive ./example/competitive
	CGO_ENABLED=1 VITRA_DEMO_SECONDS=3 xvfb-run -a .bin/competitive

e2e:
	CGO_ENABLED=1 $(GO) build -tags vitra_native -o .bin/competitive ./example/competitive
	@out=$$(CGO_ENABLED=1 VITRA_DEMO_SECONDS=20 VITRA_E2E=1 timeout 90 xvfb-run -a .bin/competitive 2>&1); \
	echo "$$out"; \
	echo "$$out" | grep -q VITRA_E2E_OK

# Notes demo in the native WebView: saves a note and runs every attack in the UI.
# Linux needs xvfb; on macOS and Windows run: go test -tags vitra_native -run TestNotesE2E ./example/notes
e2e-notes:
	CGO_ENABLED=1 timeout 120 xvfb-run -a $(GO) test -tags vitra_native -count=1 -run TestNotesE2E -v ./example/notes/

notes:
	CGO_ENABLED=1 $(GO) run -tags vitra_native ./example/notes

# Native host tests, one process each (GTK is bound to one thread). The D-Bus
# tests run fake services on a private session bus: needs dbus + python3-gi.
test-native:
	CGO_ENABLED=1 xvfb-run -a $(GO) test -tags vitra_native -count=1 ./platform/linux/ -run TestNativeWindowChrome
	CGO_ENABLED=1 xvfb-run -a $(GO) test -tags vitra_native -count=1 ./platform/linux/ -run TestNativeHostOutlivesLastWindow
	CGO_ENABLED=1 xvfb-run -a $(GO) test -tags vitra_native -count=1 ./platform/linux/ -run TestNativeMenuAccelerator
	CGO_ENABLED=1 xvfb-run -a $(GO) test -tags vitra_native -count=1 ./platform/linux/ -run 'TestTray'
	CGO_ENABLED=1 xvfb-run -a $(GO) test -tags vitra_native -count=1 ./platform/linux/ -run TestNativePresentation
	CGO_ENABLED=1 xvfb-run -a $(GO) test -tags vitra_native -count=1 ./platform/linux/ -run TestNativeGlobalShortcut
	CGO_ENABLED=1 xvfb-run -a $(GO) test -tags vitra_native -count=1 ./platform/linux/ -run TestNativeProgramName
	CGO_ENABLED=1 xvfb-run -a $(GO) test -tags vitra_native -count=1 ./platform/linux/ -run 'TestPortal'
	CGO_ENABLED=1 xvfb-run -a $(GO) test -tags vitra_native -count=1 ./platform/linux/ -run 'TestDispatch'

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
