# Container images (pin for CI-like reproducibility; bump with README toolchain table)
# Keep GOLANGCI_LINT_MOD in sync with go install version in .github/workflows/ci.yml (job golangci).
# Sync host test targets with .github/workflows/ci.yml (job local-ci-parity).
BUF_IMAGE ?= bufbuild/buf:1.50.0
GO_IMAGE ?= golang:1.26-bookworm
MAVEN_IMAGE ?= maven:3.9.11-eclipse-temurin-25
GOLANGCI_LINT_MOD ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.1
GOLANGCI_LINT ?= golangci-lint
GO_TEST_FLAGS ?= ./...
export PATH := $(shell go env GOPATH)/bin:$(PATH)
ROOT := $(CURDIR)

ifeq ($(OS),Windows_NT)
BASH ?= "C:/Program Files/Git/bin/bash.exe"
GO_TEST_RUN = set CGO_ENABLED=0&& go test $(GO_TEST_FLAGS)
# Host gcc is often missing on Windows; run -race in Docker (parity with Linux CI).
GATEWAY_RACE_RUN = docker run --rm -v "$(ROOT):/workspace" -w /workspace/src/backend/gateway $(GO_IMAGE) \
	bash -c "CGO_ENABLED=1 go test -race $(GO_TEST_FLAGS)"
else
BASH ?= bash
GO_TEST_RUN = CGO_ENABLED=0 go test $(GO_TEST_FLAGS)
GATEWAY_RACE_RUN = CGO_ENABLED=1 go test -race $(GO_TEST_FLAGS)
endif
GO_SERVICES := analytics bot chat federation file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice
# Dockerfiles with context=src/backend (sync scripts/ci/backend-docker-context.txt and ci.yml dockerctx).
GO_SERVICES_BACKEND_CONTEXT := gateway realtime chat messaging user social voice
GO_MODULES_LINT := pkg $(GO_SERVICES)
GO_TEST_TARGETS := $(GO_SERVICES:%=go-test-%)
GO_IMAGE_TARGETS := $(GO_SERVICES:%=go-image-%)

.PHONY: buf-lint buf-format buf-breaking buf-generate compose-up compose-app-up compose-down \
	build-all build-all-breaking check-toolchain compose-config-ci buf-ci backend-test-ci backend-image-ci \
	gateway-test-ci gateway-image-ci go-test-pkg auth-test-ci auth-image-ci buf-breaking-ci \
	golangci-ci gateway-test-race-ci design-tokens-check flutter-ui-color-gate flutter-ci testcontainers-prune

buf-lint:
	buf lint

buf-format:
	buf format -w .

# Requires fetch-depth 0 and master ref (CI); locally: against origin/master if present
buf-breaking:
	buf breaking protos --against ".git#branch=master,subdir=protos"

# Emits Go stubs under gen/go (gitignored); requires network for remote BSR plugins.
buf-generate:
	buf generate

compose-up:
	docker compose up -d

compose-app-up:
	docker compose --profile app up -d --build

compose-down:
	docker compose down

# --- CI parity: host Go/Maven/golangci (tests need Docker socket for testcontainers); Docker for buf/compose/images ---

check-toolchain:
	$(BASH) "$(ROOT)/scripts/ci/check-toolchain.sh"

compose-config-ci:
	docker compose config --quiet
	docker compose config --format json | docker run --rm -i -v "$(ROOT):/w" ghcr.io/jqlang/jq:1.7 -e -f /w/scripts/ci/compose-nats-jetstream.jq

buf-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf lint && buf format -d --exit-code"

backend-test-ci: go-test-pkg $(GO_TEST_TARGETS) auth-test-ci testcontainers-prune

backend-image-ci: $(GO_IMAGE_TARGETS) auth-image-ci

# Remove testcontainers leftovers (Ryuk may be disabled on Windows). Does not touch compose stacks.
testcontainers-prune:
	$(BASH) "$(ROOT)/scripts/ci/testcontainers-prune.sh"

go-test-pkg:
	cd "$(ROOT)/src/backend/pkg" && $(GO_TEST_RUN)

go-test-%:
	cd "$(ROOT)/src/backend/$*" && $(GO_TEST_RUN)

go-image-%:
	docker build -f src/backend/$*/Dockerfile -t voice-$*:local $(if $(filter $*,$(GO_SERVICES_BACKEND_CONTEXT)),src/backend,src/backend/$*)

gateway-test-ci: go-test-gateway

gateway-image-ci:
	docker build -f src/backend/gateway/Dockerfile -t voice-gateway:local src/backend

auth-test-ci:
	cd "$(ROOT)/src/backend/auth" && mvn -B test

auth-image-ci:
	docker build -f src/backend/auth/Dockerfile -t voice-auth:local src/backend/auth

golangci-ci:
	$(BASH) "$(ROOT)/scripts/ci/golangci-ci.sh" "$(ROOT)"

gateway-test-race-ci:
	cd "$(ROOT)/src/backend/gateway" && $(GATEWAY_RACE_RUN)

# Full local CI (parity with .github/workflows/ci.yml): check-toolchain, compose+buf in Docker,
# host go test/mvn/golangci (testcontainers need host Docker), images, testcontainers-prune.
# Requires Go 1.26, Docker daemon, Maven/Java on PATH. Flutter: make flutter-ci
build-all: check-toolchain compose-config-ci buf-ci backend-test-ci golangci-ci gateway-test-race-ci backend-image-ci

design-tokens-check:
	$(BASH) "$(ROOT)/scripts/design/design-tokens-check.sh"

flutter-ui-color-gate:
	$(BASH) "$(ROOT)/scripts/design/flutter-ui-color-gate.sh"

# Host Flutter SDK (parity with job `flutter` in .github/workflows/ci.yml).
flutter-ci: design-tokens-check flutter-ui-color-gate
	cd $(ROOT)/src/frontend && flutter pub get && flutter analyze && flutter test

buf-breaking-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf breaking protos --against '.git#branch=master,subdir=protos'"

buf-generate-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf generate"

# Same as build-all plus protobuf compatibility vs master (fails if master ref missing)
build-all-breaking: build-all buf-breaking-ci
