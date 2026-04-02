BINARY := lazyclaude
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build test test-unit test-vhs readme-gif lint install uninstall clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/lazyclaude

test:
	go test -race -cover ./...

test-unit:
	go test -race -cover ./internal/...

## VHS tape recording (e.g. make test-vhs TAPE=ssh_launch)
COMPOSE_PROJECT := lazyclaude-e2e-$(shell git rev-parse --short HEAD 2>/dev/null || echo local)
COMPOSE := docker compose -p $(COMPOSE_PROJECT) -f vis_e2e_tests/docker-compose.ssh.yml

test-vhs:
	$(COMPOSE) build
	TAPE=$(TAPE) $(COMPOSE) run --rm vhs
	$(COMPOSE) down

## Generate README hero GIF and copy to docs/images/ (Docker required)
readme-gif:
	$(COMPOSE) build
	TAPE=hero $(COMPOSE) run --rm vhs
	$(COMPOSE) down
	cp vis_e2e_tests/outputs/hero/hero.gif docs/images/hero.gif
	@echo "Updated docs/images/hero.gif"

lint:
	golangci-lint run ./...

PREFIX ?= /usr/local

install: build
	@if echo "$(CURDIR)" | grep -q '\.claude/worktrees'; then \
		echo "ERROR: make install is not allowed from a worktree" >&2; exit 1; fi
	install -d $(PREFIX)/bin
	install -m 755 bin/$(BINARY) $(PREFIX)/bin/$(BINARY)

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)

clean:
	rm -rf bin/
