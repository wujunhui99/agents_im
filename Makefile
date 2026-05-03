SHELL := /usr/bin/env bash

export PATH := /tmp/go/bin:$(HOME)/go/bin:$(PATH)

FRONTEND_HOST ?= 0.0.0.0
FRONTEND_PORT ?= 5173
FRONTEND_STRICT_PORT ?= true
FRONTEND_PID := .dev/pids/frontend.pid
FRONTEND_LOG := .dev/logs/frontend.log
FRONTEND_URL := http://127.0.0.1:$(FRONTEND_PORT)

.PHONY: help start stop restart backend-start backend-stop backend-restart frontend-start frontend-stop frontend-restart status test verify

help: ## Show available make targets.
	@awk 'BEGIN {FS = ":.*## "; printf "agents_im local commands:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

start: backend-start frontend-start ## Start backend services and frontend dev server.

stop: frontend-stop backend-stop ## Stop frontend dev server and backend services.

restart: stop start ## Restart backend services and frontend dev server.

backend-start: ## Start Docker middleware, migrations, and backend APIs/WebSocket gateway.
	@scripts/dev-up.sh

backend-stop: ## Stop backend host services started by scripts/dev-up.sh.
	@scripts/dev-up.sh --stop

backend-restart: backend-stop backend-start ## Restart backend services.

frontend-start: ## Start Vite frontend dev server in the background.
	@mkdir -p .dev/pids .dev/logs
	@if [[ -f "$(FRONTEND_PID)" ]] && kill -0 "$$(cat "$(FRONTEND_PID)")" >/dev/null 2>&1; then \
		echo "frontend already running pid=$$(cat "$(FRONTEND_PID)")"; \
		echo "frontend url: $(FRONTEND_URL)"; \
		exit 0; \
	fi
	@echo "starting frontend on $(FRONTEND_URL); log=$(FRONTEND_LOG)"
	@nohup npm --prefix web run dev -- --host $(FRONTEND_HOST) --port $(FRONTEND_PORT) --strictPort=$(FRONTEND_STRICT_PORT) </dev/null > "$(FRONTEND_LOG)" 2>&1 & echo $$! > "$(FRONTEND_PID)"
	@for attempt in $$(seq 1 60); do \
		if curl --silent --fail "$(FRONTEND_URL)" >/dev/null 2>&1; then \
			echo "frontend ready: $(FRONTEND_URL)"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "frontend did not become ready at $(FRONTEND_URL); see $(FRONTEND_LOG)" >&2; \
	exit 1

frontend-stop: ## Stop Vite frontend dev server started by this Makefile.
	@if [[ ! -f "$(FRONTEND_PID)" ]]; then \
		echo "no frontend PID file found"; \
		exit 0; \
	fi
	@pid="$$(cat "$(FRONTEND_PID)")"; \
	if [[ -n "$$pid" ]] && kill -0 "$$pid" >/dev/null 2>&1; then \
		echo "stopping frontend pid=$$pid"; \
		kill "$$pid" >/dev/null 2>&1 || true; \
	else \
		echo "frontend pid $$pid is not running"; \
	fi; \
	rm -f "$(FRONTEND_PID)"

frontend-restart: frontend-stop frontend-start ## Restart Vite frontend dev server.

status: ## Show local frontend/backend PID files and listening ports.
	@echo "PID files:"; \
	if compgen -G ".dev/pids/*.pid" >/dev/null; then \
		for f in .dev/pids/*.pid; do \
			name="$$(basename "$$f" .pid)"; pid="$$(cat "$$f")"; \
			if [[ -n "$$pid" ]] && kill -0 "$$pid" >/dev/null 2>&1; then state=running; else state=stopped; fi; \
			printf "  %-16s pid=%-8s %s\n" "$$name" "$$pid" "$$state"; \
		done; \
	else \
		echo "  none"; \
	fi
	@echo; echo "Listening ports:"; \
	ss -ltnp 2>/dev/null | awk 'NR==1 || /:(8080|8081|8082|8083|8084|8085|8086|5173)\b/' || true

test: ## Run frontend tests, build, lint, and backend Go tests.
	@npm run frontend:test
	@npm run frontend:build
	@npm run frontend:lint
	@go list ./... | grep -v '/web/node_modules/' | xargs go test

verify: test ## Run full local static verification.
	@bash scripts/verify-static.sh
	@docker compose config >/dev/null
	@git diff --check
	@echo "verification passed"
