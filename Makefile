GO ?= go
DOCKER_COMPOSE ?= docker compose
GOMODCACHE ?=
GOENV :=
GO_FILES := $(shell find . -type f | grep '\.go$$')
BIN_DIR := $(CURDIR)/bin
APP_BIN := log-parser

.PHONY: setup infra-up infra-down run tidy fmt test build clean compose-up compose-down logs

ifneq ($(strip $(GOMODCACHE)),)
GOENV += GOMODCACHE=$(GOMODCACHE)
endif

setup:
	$(MAKE) clean
	$(if $(strip $(GOMODCACHE)),mkdir -p $(GOMODCACHE),true)
	$(GOENV) $(GO) mod download
	$(MAKE) infra-up

infra-up:
	$(DOCKER_COMPOSE) up -d postgres

infra-down:
	$(DOCKER_COMPOSE) down

run:
	$(GOENV) $(GO) run ./cmd/log-parser

tidy:
	$(GOENV) $(GO) mod tidy

fmt:
	gofmt -w $(GO_FILES)

test:
	$(GOENV) $(GO) test ./...

build:
	mkdir -p $(BIN_DIR)
	$(GOENV) $(GO) build -o $(BIN_DIR)/$(APP_BIN) ./cmd/log-parser

compose-up:
	$(DOCKER_COMPOSE) up -d --build

compose-down:
	$(DOCKER_COMPOSE) down

logs:
	$(DOCKER_COMPOSE) logs -f app

clean:
	rm -rf $(BIN_DIR)
