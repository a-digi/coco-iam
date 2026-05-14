# Makefile for the coco-iam-deploy tool. Operators run these
# targets from the repo root via `make deploy-*` or from the
# deploy/ dir directly. Every target is a thin wrapper that
# shells out to the compiled binary.

BINARY     ?= bin/deploy
CONFIG     ?= deploy.yaml
GO         ?= go

.PHONY: build install test vet fmt clean deploy status rollback validate

build:
	$(GO) build -o $(BINARY) ./cmd/deploy

install:
	$(GO) install ./cmd/deploy

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf bin/

# -- Convenience wrappers -----------------------------------------
# Each forwards CONFIG + trailing flags (FLAGS="...") to the
# built binary. Build is a dep so the binary is always current.

deploy: build
	./$(BINARY) deploy --config $(CONFIG) $(FLAGS)

status: build
	./$(BINARY) status --config $(CONFIG) $(FLAGS)

rollback: build
	./$(BINARY) rollback --config $(CONFIG) $(FLAGS)

validate: build
	./$(BINARY) validate --config $(CONFIG) $(FLAGS)
