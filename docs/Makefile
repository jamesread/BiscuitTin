# Builds the docs site into build/site at the repo root. Run `make docs` from
# the repo root, or `make` here.

ROOT := $(abspath ..)

.PHONY: site deps screenshots

site: deps screenshots
	cd $(ROOT) && ./docs/node_modules/.bin/antora antora-playbook.yml

deps: node_modules

node_modules: package.json package-lock.json
	npm ci --no-audit --no-fund
	touch node_modules

# The marketing screenshots (`make screenshots`) are the docs' images too.
screenshots:
	mkdir -p modules/ROOT/images/screenshots
	cp $(ROOT)/var/marketing/screenshots/*.png modules/ROOT/images/screenshots/
