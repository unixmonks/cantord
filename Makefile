BACKEND_DIR  := backend
FRONTEND_DIR := frontend

.DEFAULT_GOAL := help
.PHONY: help install build build-backend build-frontend ctl freebsd \
        run dev test vet fmt lint clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

install: ## Install frontend deps (backend deps resolve via go.mod on build)
	cd $(FRONTEND_DIR) && npm install

build: build-backend ctl build-frontend ## Build backend binary + cantordctl + frontend production bundle

build-backend: ## Build the cantord daemon binary (backend/cantord)
	cd $(BACKEND_DIR) && go build -o cantord ./cmd/cantord

ctl: ## Build the cantordctl CLI client (backend/cantordctl)
	cd $(BACKEND_DIR) && go build -o cantordctl ./cmd/cantordctl

build-frontend: ## Build the frontend production bundle (frontend/dist)
	cd $(FRONTEND_DIR) && npm run build

freebsd: ## Cross-compile the daemon for FreeBSD/amd64
	cd $(BACKEND_DIR) && GOOS=freebsd GOARCH=amd64 go build -o cantord-freebsd ./cmd/cantord

run: build-backend ## Run the daemon (CONFIG=path/to/cantord.toml to override)
	cd $(BACKEND_DIR) && ./cantord $(if $(CONFIG),-config $(CONFIG),)

dev: ## Run the frontend dev server
	cd $(FRONTEND_DIR) && npm run dev

test: ## Run backend tests
	cd $(BACKEND_DIR) && go test ./...

vet: ## Vet backend sources
	cd $(BACKEND_DIR) && go vet ./...

fmt: ## Format backend sources with gofmt
	cd $(BACKEND_DIR) && gofmt -l -w .

lint: vet ## Lint backend (go vet) and frontend (oxlint)
	cd $(FRONTEND_DIR) && npm run lint

clean: ## Remove build artifacts
	rm -f $(BACKEND_DIR)/cantord $(BACKEND_DIR)/cantordctl $(BACKEND_DIR)/cantord-freebsd
	rm -rf $(FRONTEND_DIR)/dist
