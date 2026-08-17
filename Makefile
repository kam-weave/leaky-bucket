.PHONY: setup diagrams
.PHONY: build-go run-go seed-go reset-go bench-go test-go test-go-all clean-go
.PHONY: build-java run-java seed-java reset-java test-java clean-java
.PHONY: clean

# One-liner dev setup: install the VS Code extension that renders the
# inline Mermaid diagrams in docs/. Safe to re-run; no-op if `code` is absent.
setup:
	@command -v code >/dev/null 2>&1 \
		&& code --install-extension bierner.markdown-mermaid --force \
		&& echo "Ready. Open any docs/*.md and press Cmd+Shift+V to preview diagrams." \
		|| echo "VS Code 'code' CLI not found — diagrams also render on GitHub. Skipping."

# Render every Mermaid diagram in docs/ to SVG and open a browser gallery.
# Extension-free fallback for viewing diagrams locally (needs Node/npx).
diagrams:
	@bash scripts/render-diagrams.sh

# Go targets
build-go:
	$(MAKE) -C go build

run-go:
	$(MAKE) -C go run

seed-go:
	$(MAKE) -C go seed

reset-go:
	$(MAKE) -C go reset

bench-go:
	$(MAKE) -C go bench

test-go:
	$(MAKE) -C go test

# Every Go test level: unit + integration (race), e2e, and mutation.
test-go-all:
	$(MAKE) -C go test-all

clean-go:
	$(MAKE) -C go clean

# Java targets
build-java:
	$(MAKE) -C java build

run-java:
	$(MAKE) -C java run

seed-java:
	$(MAKE) -C java seed

reset-java:
	$(MAKE) -C java reset

test-java:
	$(MAKE) -C java test

clean-java:
	$(MAKE) -C java clean

# Combined targets
clean: clean-go clean-java
