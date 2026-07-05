GO := GOTOOLCHAIN=local go
LDFLAGS :=

ifeq ($(shell uname -s),Darwin)
LDFLAGS := -ldflags=-linkmode=external
endif

.PHONY: fmt vet test race build-api build-cli

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test $(LDFLAGS) ./...

race:
	$(GO) test -race $(LDFLAGS) ./...

build-api:
	mkdir -p bin
	$(GO) build $(LDFLAGS) -o bin/api ./cmd/api

build-cli:
	mkdir -p bin
	$(GO) build $(LDFLAGS) -o bin/amazonas ./cmd/amazonas
