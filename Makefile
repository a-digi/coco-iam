.PHONY: upgrade-go
upgrade-go:
	@echo "Upgrading Go using Homebrew..."
	brew update
	brew upgrade go
	@echo "Go upgrade complete. Run 'go version' to verify."

.PHONY: get-branch
get-branch:
	@BRANCH=$(shell git rev-parse --abbrev-ref HEAD | sed 's/\./-/g'); echo $$BRANCH

.PHONY: build
build:
	@mkdir -p versions
	BRANCH=$(shell git rev-parse --abbrev-ref HEAD | sed 's/\./-/g'); \
	cd api && go build -o ../versions/coco-iam-$$BRANCH main.go

# Linux target architecture for the deploy build. Override if
# the server is x86_64:
#   make build-linux DEPLOY_GOARCH=amd64   (x86_64)
#   make build-linux DEPLOY_GOARCH=arm64   (aarch64, default)
DEPLOY_GOARCH ?= arm64

# Map GOARCH to the zig cc target triple.
ifeq ($(DEPLOY_GOARCH),amd64)
  ZIG_TARGET := x86_64-linux-gnu
else ifeq ($(DEPLOY_GOARCH),arm64)
  ZIG_TARGET := aarch64-linux-gnu
else
  $(error Unsupported DEPLOY_GOARCH "$(DEPLOY_GOARCH)" — use amd64 or arm64)
endif

.PHONY: build-linux
build-linux:
	@mkdir -p versions
	# Cross-compile to linux/$(DEPLOY_GOARCH). mattn/go-sqlite3
	# needs CGO, so we need a Linux C compiler on the Mac host.
	# `zig cc` supplies one — a single lightweight brew install
	# that ships a self-contained clang + libc for every target.
	@command -v zig >/dev/null 2>&1 || { \
	    echo "zig not found. Install with: brew install zig"; \
	    exit 1; \
	}
	cd api && CGO_ENABLED=1 GOOS=linux GOARCH=$(DEPLOY_GOARCH) \
	    CC="zig cc -target $(ZIG_TARGET)" \
	    CXX="zig c++ -target $(ZIG_TARGET)" \
	    go build -o ../versions/coco-iam-deploy main.go

.PHONY: run-dev
run-dev:
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	cd api && go run main.go start &

.PHONY: run-dev-app
run-dev-app: run-dev
	cd app && npm install && npm run dev

.PHONY: stop-dev
stop-dev:
	cd api && go run main.go shutdown

.PHONY: run-locally
run-locally: run-dev run-dev-app

.PHONY: run
run:
	BRANCH=$(shell git rev-parse --abbrev-ref HEAD | sed 's/\./-/g'); \
	./versions/coco-iam-$$BRANCH start &

.PHONY: stop
stop:
	BRANCH=$(shell git rev-parse --abbrev-ref HEAD | sed 's/\./-/g'); \
	./versions/coco-iam-$$BRANCH shutdown

.PHONY: check-port
check-port:
	@if [ -z "$(PORT)" ]; then \
		echo "Usage: make check-port PORT=xxxx"; \
		exit 1; \
	fi; \
	if lsof -i :$(PORT) | grep LISTEN > /dev/null; then \
		echo "Port $(PORT) is busy."; \
	else \
		echo "Port $(PORT) is free."; \
	fi

.PHONY: lint-fix
lint-fix:
	cd app && npx eslint . --ext .js,.jsx,.ts,.tsx --fix

.PHONY: test-api
test-api:
	cd api && go test ./...

.PHONY: test-api-verbose
test-api-verbose:
	cd api && go test ./... -v

.PHONY: test-api-cover
test-api-cover:
	cd api && go test ./... -cover

.PHONY: test-backend
test-backend:
	cd api && go test -v $(if $(PKG),$(PKG),./...) $(if $(RUN),-run $(RUN))

.PHONY: test-app
test-app:
	cd app && npm test

.PHONY: test-app-watch
test-app-watch:
	cd app && npm run test:watch

.PHONY: test-smoke
test-smoke:
	cd api && go test -tags smoke ./tests/smoke/... -v

.PHONY: test
test: test-api test-app

define ENSURE_ADMIN_ENV
if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
if [ -z "$$ADMIN_USERNAME" ]; then printf 'Admin username: '; read ADMIN_USERNAME; fi; \
if [ -z "$$ADMIN_EMAIL" ]; then printf 'Admin email: '; read ADMIN_EMAIL; fi; \
if [ -z "$$ADMIN_PASSWORD" ]; then \
	printf 'Admin password: '; \
	stty -echo 2>/dev/null; \
	trap 'stty echo 2>/dev/null' EXIT INT TERM; \
	read ADMIN_PASSWORD; \
	stty echo 2>/dev/null; \
	trap - EXIT INT TERM; \
	printf '\n'; \
fi; \
if [ -z "$$ADMIN_USERNAME" ] || [ -z "$$ADMIN_EMAIL" ] || [ -z "$$ADMIN_PASSWORD" ]; then \
	echo 'Error: admin username, email, and password are all required'; \
	exit 1; \
fi; \
export ADMIN_USERNAME ADMIN_EMAIL ADMIN_PASSWORD
endef

.PHONY: create-admin
create-admin:
	@$(ENSURE_ADMIN_ENV); \
	BRANCH=$$(git rev-parse --abbrev-ref HEAD | sed 's/\./-/g'); \
	./versions/coco-iam-$$BRANCH create-admin "$$ADMIN_USERNAME" "$$ADMIN_EMAIL" "$$ADMIN_PASSWORD"

.PHONY: create-admin-dev
create-admin-dev:
	@$(ENSURE_ADMIN_ENV); \
	cd api && go run main.go create-admin "$$ADMIN_USERNAME" "$$ADMIN_EMAIL" "$$ADMIN_PASSWORD"

.PHONY: api-mod-tidy-vendor
api-mod-tidy-vendor:
	cd api && go mod tidy && go mod vendor

DEPLOY_CONFIG        ?= deploy/prod/deploy.yaml
DEPLOY_BIN           ?= deploy/app/bin/deploy
DEPLOY_SERVICE_FILE  ?= deploy/prod/coco-iam.service
DEPLOY_SSH_PW_FILE   ?= deploy/secrets/ssh-password
DEPLOY_SSH_TARGET    ?= $(shell awk '/^  user:/ {u=$$2} /^  host:/ {h=$$2} END {print u "@" h}' $(DEPLOY_CONFIG))

.PHONY: deploy-tool-build
deploy-tool-build:
	cd deploy/app && go build -o bin/deploy ./cmd/deploy

.PHONY: deploy-build
deploy-build: build-linux
	cd app && npm install && npm run build

.PHONY: deploy
deploy: deploy-tool-build deploy-build
	./$(DEPLOY_BIN) deploy --config $(DEPLOY_CONFIG) $(FLAGS)

.PHONY: deploy-api
deploy-api: deploy-tool-build build-linux
	./$(DEPLOY_BIN) deploy --config $(DEPLOY_CONFIG) --artifact backend-binary $(FLAGS)

.PHONY: deploy-frontend
deploy-frontend: deploy-tool-build
	cd app && npm install && npm run build
	./$(DEPLOY_BIN) deploy --config $(DEPLOY_CONFIG) --artifact frontend-bundle $(FLAGS)

.PHONY: deploy-validate
deploy-validate: deploy-tool-build
	./$(DEPLOY_BIN) validate --config $(DEPLOY_CONFIG) $(FLAGS)

.PHONY: deploy-status
deploy-status: deploy-tool-build
	./$(DEPLOY_BIN) status --config $(DEPLOY_CONFIG) $(FLAGS)

.PHONY: deploy-rollback
deploy-rollback: deploy-tool-build
	./$(DEPLOY_BIN) rollback --config $(DEPLOY_CONFIG) $(FLAGS)

.PHONY: deploy-install-service
deploy-install-service:
	@command -v sshpass >/dev/null 2>&1 || { \
	    echo "sshpass not found. Install with: brew install hudochenkov/sshpass/sshpass"; \
	    exit 1; \
	}
	@[ -s $(DEPLOY_SSH_PW_FILE) ] || { \
	    echo "Password file $(DEPLOY_SSH_PW_FILE) is empty. Run: printf '%s' 'yourpw' > $(DEPLOY_SSH_PW_FILE)"; \
	    exit 1; \
	}
	@echo "Installing systemd unit on $(DEPLOY_SSH_TARGET)..."
	SSHPASS=$$(cat $(DEPLOY_SSH_PW_FILE)) sshpass -e \
	    scp -o StrictHostKeyChecking=accept-new \
	    $(DEPLOY_SERVICE_FILE) \
	    $(DEPLOY_SSH_TARGET):/etc/systemd/system/coco-iam.service
	SSHPASS=$$(cat $(DEPLOY_SSH_PW_FILE)) sshpass -e \
	    ssh -o StrictHostKeyChecking=accept-new $(DEPLOY_SSH_TARGET) \
	    'systemctl daemon-reload && systemctl enable coco-iam'
	@echo "Systemd unit installed and enabled. Now run: make deploy"
