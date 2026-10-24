# Regenerates marketing screenshots of the biscuittin X11 window, on a private
# Xvfb display of a fixed size, with fictional config and settings. Run `make`
# here; Xvfb is stopped afterwards, even when a step fails.

ROOT := $(abspath ../../..)
HERE := $(abspath .)
TMP := $(HERE)/tmp

WIDTH := $(shell sed -n 's/^width *= *//p' screenshots.ini)
HEIGHT := $(shell sed -n 's/^height *= *//p' screenshots.ini)
FONT := $(shell sed -n 's/^font: *//p' config.yaml)

# A display number unlikely to be in use; override with `make XDISPLAY=:123`.
XDISPLAY ?= :97

# Keep the app away from your own config, settings and caches.
APP_ENV := DISPLAY=$(XDISPLAY) MKT_DIR=$(HERE) XDG_STATE_HOME=$(TMP)/state \
	XDG_CONFIG_HOME=$(TMP)/config XDG_CACHE_HOME=$(TMP)/cache

.PHONY: all check build up shoot manifest down clean

all:
	@$(MAKE) check build up shoot manifest; rc=$$?; $(MAKE) down; exit $$rc

check:
	@command -v Xvfb >/dev/null || { echo "Xvfb is missing: sudo dnf install xorg-x11-server-Xvfb (or apt install xvfb)"; exit 1; }
	@[ -f "$(FONT)" ] || { echo "Font $(FONT) is missing: sudo dnf install google-noto-sans-fonts"; exit 1; }
	@[ ! -e /tmp/.X11-unix/X$(subst :,,$(XDISPLAY)) ] || { echo "Display $(XDISPLAY) is in use; try make XDISPLAY=:123"; exit 1; }

# The web remote isn't captured, so the frontend isn't built.
build:
	mkdir -p $(TMP)
	cd $(ROOT)/desktop && CGO_ENABLED=1 go build -o $(TMP)/biscuittin ./cmd/biscuittin

up:
	rm -rf $(TMP)/state $(TMP)/cache $(TMP)/config
	mkdir -p $(TMP)/state/biscuittin
	cp settings.yaml $(TMP)/state/biscuittin/settings.yaml
	{ Xvfb $(XDISPLAY) -screen 0 $(WIDTH)x$(HEIGHT)x24 -nolisten tcp > $(TMP)/xvfb.log 2>&1 & echo $$! > $(TMP)/xvfb.pid; }
	@for i in $$(seq 50); do [ -e /tmp/.X11-unix/X$(subst :,,$(XDISPLAY)) ] && exit 0; sleep 0.1; done; \
		echo "Xvfb did not start; see $(TMP)/xvfb.log"; exit 1

# biscuittin runs the generated steps, saves each PNG from its own window
# (exactly WIDTHxHEIGHT, no decorations) and quits.
shoot:
	./generate-script.py > $(TMP)/steps
	$(APP_ENV) timeout 300 $(TMP)/biscuittin -config config.yaml -size $(WIDTH)x$(HEIGHT) \
		-script $(TMP)/steps > $(TMP)/biscuittin.log 2>&1 \
		|| { tail -n 20 $(TMP)/biscuittin.log; exit 1; }

manifest:
	./generate-manifest.py

down:
	-@[ -f $(TMP)/xvfb.pid ] && kill $$(cat $(TMP)/xvfb.pid) 2>/dev/null; rm -f $(TMP)/xvfb.pid

clean: down
	rm -rf $(TMP)
