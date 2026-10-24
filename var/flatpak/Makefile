# Wraps a biscuittin binary into a Flatpak bundle. GoReleaser runs this after
# building (see .goreleaser.yaml); `make flatpak` at the repo root runs it
# for a local build.

APP_ID := io.github.jamesread.biscuittin
ROOT := $(abspath ../..)
HERE := $(abspath .)
TMP := $(HERE)/tmp

BINARY ?= $(ROOT)/desktop/biscuittin
VERSION ?= dev
ARCH ?= x86_64
OUT ?= $(ROOT)/dist/biscuittin_$(VERSION)_$(ARCH).flatpak
RUNTIME_REPO := https://dl.flathub.org/repo/flathub.flatpakrepo

.PHONY: bundle stage lint clean

bundle: stage
	flatpak-builder --user --install-deps-from=flathub --disable-rofiles-fuse \
		--force-clean --arch=$(ARCH) --default-branch=stable \
		--state-dir=$(TMP)/state --repo=$(TMP)/repo \
		$(TMP)/app $(APP_ID).yml
	mkdir -p $(dir $(OUT))
	flatpak build-bundle --arch=$(ARCH) --runtime-repo=$(RUNTIME_REPO) \
		$(TMP)/repo $(OUT) $(APP_ID) stable
	@echo "Wrote $(OUT)"

# The manifest's `dir` source: the binary plus the files installed with it.
# Releases get a <release> entry in the metainfo.
stage:
	rm -rf build && mkdir -p build
	cp $(BINARY) build/biscuittin
	cp $(APP_ID).desktop $(ROOT)/logo.svg build/
	if [ "$(VERSION)" = dev ]; then \
		cp $(APP_ID).metainfo.xml build/; \
	else \
		sed 's|<releases/>|<releases><release version="$(VERSION)" date="'$$(date -u +%F)'"/></releases>|' \
			$(APP_ID).metainfo.xml > build/$(APP_ID).metainfo.xml; \
	fi

# Validates the desktop entry and metainfo (needs desktop-file-utils and
# appstream).
lint:
	desktop-file-validate $(APP_ID).desktop
	appstreamcli validate --no-net $(APP_ID).metainfo.xml

clean:
	rm -rf build $(TMP) .flatpak-builder
