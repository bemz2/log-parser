GO ?= go
MOCKERY ?= $(shell $(GO) env GOPATH)/bin/mockery
GOLANGCI_LINT ?= $(shell $(GO) env GOPATH)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.12.2
DOCKER_COMPOSE ?= docker compose
GOMODCACHE ?=
MOCKERY_GOCACHE ?= /tmp/log-parser-mockery-go-build
GOENV :=
GO_FILES := $(shell find . -type f | grep '\.go$$')
BIN_DIR := $(CURDIR)/bin
APP_BIN := log-parser

.PHONY: setup ensure-configs ensure-env ensure-compose infra-up infra-down run tidy fmt fmt-check gofmt lint test mocks build clean compose-up compose-down logs

ifneq ($(strip $(GOMODCACHE)),)
GOENV += GOMODCACHE=$(GOMODCACHE)
endif

setup:
	$(MAKE) clean
	$(MAKE) ensure-configs
	$(if $(strip $(GOMODCACHE)),mkdir -p $(GOMODCACHE),true)
	$(GOENV) $(GO) mod download
	$(MAKE) infra-up

ensure-configs: ensure-env ensure-compose

ensure-env:
	test -f .env || cp samples/.env.sample .env

ensure-compose:
	test -f docker-compose.yml || cp samples/docker-compose.yml.sample docker-compose.yml

infra-up:
	$(DOCKER_COMPOSE) up -d postgres

infra-down:
	$(DOCKER_COMPOSE) down

run:
	$(GOENV) $(GO) run ./cmd/log-parser

tidy:
	$(GOENV) $(GO) mod tidy

fmt:
	$(GOENV) $(GO) fmt ./...

fmt-check:
	test -z "$$($(GOENV) $(GO) fmt ./...)"

gofmt:
	gofmt -w $(GO_FILES)

lint: mocks
	test -x $(GOLANGCI_LINT) || $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GOLANGCI_LINT) run ./...

test: mocks
	$(GOENV) $(GO) test ./...

mocks:
	mkdir -p mocks
	test -x $(MOCKERY) || $(GO) install github.com/vektra/mockery/v2@v2.53.6
	GOCACHE=$(MOCKERY_GOCACHE) $(MOCKERY) --config .mockery.yaml --dir internal/service --name TopologyRepository --output mocks --outpkg mocks --filename topology_repository.go --structname TopologyRepository
	GOCACHE=$(MOCKERY_GOCACHE) $(MOCKERY) --config .mockery.yaml --dir internal/service --name LogParser --output mocks --outpkg mocks --filename log_parser.go --structname LogParser
	GOCACHE=$(MOCKERY_GOCACHE) $(MOCKERY) --config .mockery.yaml --dir internal/delivery/http/handler --name TopologyService --output mocks --outpkg mocks --filename topology_service.go --structname TopologyService

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
