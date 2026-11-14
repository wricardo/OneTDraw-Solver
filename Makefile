# One T Draw — development tooling

.PHONY: help build build-frontend test run dev validate fmt vet generate clean verify check-frontend

PORT ?= 8090

help:
	@echo "one-t-draw - available targets:"
	@echo "  build           - Build the server binary (./one-t-draw)"
	@echo "  build-frontend  - Build the SvelteKit UI and copy it into static/"
	@echo "  test            - Run all Go tests with the race detector"
	@echo "  run             - Build and run the server on PORT (default 8090)"
	@echo "  dev             - Run the server from source with debug logging"
	@echo "  validate        - Validate every map and its reference solution"
	@echo "  fmt             - gofmt all Go files"
	@echo "  vet             - go vet ./..."
	@echo "  generate        - Regenerate gqlgen code from graph/schema.graphqls"
	@echo "  check-frontend  - svelte-check + frontend unit tests"
	@echo "  clean           - Remove build artifacts"
	@echo "  verify          - fmt check, vet, test, check-frontend"

build:
	go build -o one-t-draw .

build-frontend:
	@test -d frontend/node_modules || (cd frontend && npm ci)
	cd frontend && npm run build:static

test:
	go test -race ./...

run: build
	./one-t-draw -port $(PORT)

dev:
	go run . -port $(PORT) -debug

validate:
	go run ./cmd/validate -maps-dir maps -solutions-dir solutions

# gqlgen must be generated with a Go toolchain it supports. Go 1.26+ breaks
# gqlgen 0.17.73 ("package without types was imported"), so use the pinned 1.24
# toolchain when it is present and fall back to the default otherwise.
GQLGEN_GO := $(shell ls -d $(shell go env GOMODCACHE)/golang.org/toolchain@v0.0.1-go1.24.*/bin 2>/dev/null | head -1)

generate:
ifdef GQLGEN_GO
	GOTOOLCHAIN=local PATH="$(GQLGEN_GO):$$PATH" go run github.com/99designs/gqlgen generate
else
	@echo "warning: no Go 1.24 toolchain found; gqlgen may fail on newer Go"
	go run github.com/99designs/gqlgen generate
endif

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './frontend/*' -not -path './graph/generated/*')

vet:
	go vet ./...

check-frontend:
	cd frontend && npx svelte-check --tsconfig ./tsconfig.json
	cd frontend && npx vitest run

clean:
	rm -f one-t-draw coverage.out

verify:
	@out=$$(gofmt -l $$(find . -name '*.go' -not -path './frontend/*' -not -path './graph/generated/*')); \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	$(MAKE) vet
	$(MAKE) test
	$(MAKE) check-frontend
