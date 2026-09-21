APP := f
LEGACY_APP := aoo
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
GOFLAGS ?= -buildvcs=false
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || printf unknown)
LDFLAGS ?= -X aoo/internal/app.buildCommit=$(COMMIT)

.PHONY: build install update test validate tidy snapshot

build:
	@echo "[build] compiling $(APP)"
	@mkdir -p bin
	@go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/f
	@go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(LEGACY_APP) ./cmd/aoo
	@echo "[build] done: bin/$(APP), bin/$(LEGACY_APP)"

install: build
	@echo "[install] installing $(APP) to $(BINDIR)"
	@install -d "$(BINDIR)"
	@install -m 0755 bin/$(APP) "$(BINDIR)/$(APP)"
	@ln -sf "$(APP)" "$(BINDIR)/$(LEGACY_APP)"
	@echo "[install] done: $(BINDIR)/$(APP) (and $(BINDIR)/$(LEGACY_APP) symlink)"

update: install
	@echo "[update] done"

test:
	@echo "[test] running go test"
	@go test ./...
	@echo "[test] done"

validate: build
	@echo "[validate] binary built; run ./bin/$(APP) add to add hosts"
	@./bin/$(APP) version
	@echo "[validate] done"

tidy:
	@echo "[tidy] syncing go modules"
	@go mod tidy
	@echo "[tidy] done"

snapshot:
	@echo "[release] building snapshot"
	@goreleaser release --snapshot --clean
