# Build info, injected the same way as OliveTin (main.version/commit/date).
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo nocommit)
DATE    ?= $(shell git log -1 --format=%cI 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: all build frontend protocol test lint flatpak screenshots docs

all: build

# The Go app is in desktop/. The web remote (frontend/) is embedded in the
# binary, so build it first.
build: frontend
	cd desktop && CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o biscuittin ./cmd/biscuittin

frontend:
	$(MAKE) -wC frontend

# Regenerates desktop/gen/ (Go) and frontend/src/gen/ (ES) from protocol/.
protocol:
	$(MAKE) -wC protocol

test:
	cd desktop && go test ./...

# gofmt and go vet. Run by pre-commit and CI.
lint:
	@out=$$(cd desktop && gofmt -l $$(find . -name '*.go' -not -path './gen/*')); \
		if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	cd desktop && go vet ./...

# A local Flatpak bundle in dist/, compiled inside the Flatpak SDK like a
# release (see .goreleaser.yaml), without publishing anything.
flatpak: frontend
	cd desktop && CGO_ENABLED=1 ../var/flatpak/go-in-sdk build -trimpath -ldflags "$(LDFLAGS)" -o ../dist/flatpak/biscuittin ./cmd/biscuittin
	$(MAKE) -wC var/flatpak bundle BINARY=$(CURDIR)/dist/flatpak/biscuittin

# The docs site (Antora), into build/site.
docs:
	$(MAKE) -wC docs

# Marketing screenshots of the X11 window; needs Xvfb. Not part of `all`.
screenshots:
	$(MAKE) -wC var/marketing/screenshots
