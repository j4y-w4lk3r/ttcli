# Local build/install with git-derived version metadata.
# Release builds use goreleaser (.goreleaser.yaml) on git tag push.

MODULE   := github.com/j4y-w4lk3r/ttcli/cmd/ttcli
BINARY   := ttcli

# Nearest tag + commits since (e.g. v0.1.0-1-gd15b7ec); strip leading v.
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  := -s -w \
	-X github.com/j4y-w4lk3r/ttcli/internal/version.Version=$(VERSION) \
	-X github.com/j4y-w4lk3r/ttcli/internal/version.Commit=$(COMMIT) \
	-X github.com/j4y-w4lk3r/ttcli/internal/version.Date=$(DATE)

# Where `make install` puts the binary. Override: make install INSTALL_DIR=/usr/local/bin
INSTALL_DIR ?= $(HOME)/.local/bin

.PHONY: build install version test vet release-snapshot clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/ttcli

install:
	@mkdir -p "$(INSTALL_DIR)"
	GOBIN="$(INSTALL_DIR)" go install -ldflags "$(LDFLAGS)" ./cmd/ttcli
	@rm -f "$$(go env GOPATH)/bin/ttcli" 2>/dev/null || true
	@echo "→ $(INSTALL_DIR)/ttcli ($$( "$(INSTALL_DIR)/ttcli" version ))"
	@echo ""
	@echo "If plain 'ttcli' still runs an old build, refresh the shell hash table:"
	@echo "  rehash          # zsh"
	@echo "  hash -r         # bash"

version:
	@echo "version=$(VERSION) commit=$(COMMIT) date=$(DATE)"

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

# Dry-run a goreleaser release (binaries land in dist/, nothing published).
release-snapshot:
	goreleaser release --snapshot --clean

install-completions:
	@mkdir -p "$(HOME)/.config/zsh/completions"
	cp completions/_ttcli "$(HOME)/.config/zsh/completions/_ttcli"
	@echo "→ installed ~/.config/zsh/completions/_ttcli"
	@echo "  ensure fpath includes ~/.config/zsh/completions (before compinit)"
