VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

.PHONY: build run stop logs shell admin test test-web lint lint-web ptagent-build clean

# --- container image ---
build:
	docker build --build-arg VERSION=$(VERSION) \
		-t portfolio-tracker:$(VERSION) \
		-t portfolio-tracker:latest .

# --- dev/prod stack ---
# Workflow: symlink compose.override.yaml → compose.override.yaml-dev
# (or -prod) once; then plain `docker compose` picks it up.
run:
	docker compose up -d

stop:
	docker compose down

logs:
	docker compose logs -f

shell:
	docker compose exec ptd sh

# Run ptadmin inside the running container, e.g.:
#   make admin ARGS="user add --email you@x --name You --base-currency EUR"
admin:
	docker compose exec ptd ptadmin $(ARGS)

# --- CI / test (ephemeral containers) ---
test:
	docker run --rm -v $(PWD):/src:ro -w /src \
		-e GOFLAGS=-buildvcs=false \
		golang:1.26.3 \
		sh -c "go test -race -cover ./..."

lint:
	docker run --rm -v $(PWD):/src:ro -w /src \
		golangci/golangci-lint:latest-alpine \
		golangci-lint run ./...

lint-web:
	docker run --rm -v $(PWD)/web:/web -w /web node:24-alpine \
		sh -c "npm ci --silent && echo 'no lint script configured yet'"

test-web:
	docker run --rm -v $(PWD)/web:/web -w /web node:24-alpine \
		sh -c "npm ci --silent && npm test --silent"

# --- local ptagent build for development (standalone; normally via goreleaser) ---
ptagent-build:
	docker run --rm -v $(PWD):/src -w /src \
		-e CGO_ENABLED=0 -e GOFLAGS=-buildvcs=false \
		golang:1.26.3 \
		go build -ldflags "-s -w -X github.com/mtzanidakis/portfolio-tracker/internal/version.Version=$(VERSION)" \
		-o bin/ptagent ./cmd/ptagent
	@echo "Built: ./bin/ptagent"

clean:
	docker compose down -v 2>/dev/null || true
	rm -rf bin/ web/dist/ internal/web/dist/*
	touch internal/web/dist/.gitkeep
	docker image rm portfolio-tracker:$(VERSION) portfolio-tracker:latest 2>/dev/null || true
