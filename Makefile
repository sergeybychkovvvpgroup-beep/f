APP := f
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
GOFLAGS ?= -buildvcs=false

.PHONY: build install update test validate tidy snapshot

build:
	@echo "[build] compiling $(APP)"
	@mkdir -p bin
	@go build $(GOFLAGS) -o bin/$(APP) ./cmd/f
	@echo "[build] done: bin/$(APP)"

install: build
	@echo "[install] installing $(APP) to $(BINDIR)"
	@install -d "$(BINDIR)"
	@install -m 0755 bin/$(APP) "$(BINDIR)/$(APP)"
	@echo "[install] done: $(BINDIR)/$(APP)"

update: install
	@echo "[update] done"

test:
	@echo "[test] running go test"
	@go test ./...
	@echo "[test] done"

validate: build
	@echo "[validate] checking notes in ./examples/notes"
	@./bin/$(APP) validate --dir ./examples/notes
	@echo "[validate] done"

tidy:
	@echo "[tidy] syncing go modules"
	@go mod tidy
	@echo "[tidy] done"

snapshot:
	@echo "[release] building snapshot"
	@goreleaser release --snapshot --clean
