build: node_modules
	npm run build
	touch ../desktop/internal/webcontrol/webdist/.gitkeep

node_modules: package.json
	npm install --no-audit --no-fund
	touch node_modules

.PHONY: build
