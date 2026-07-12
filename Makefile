# Container images (pin for CI-like reproducibility; bump with README toolchain table)
# Keep GOLANGCI_LINT_MOD in sync with go install version in .github/workflows/ci.yml (job golangci).
# Sync host test targets with .github/workflows/ci.yml (job local-ci-parity, tier 3).
BUF_IMAGE ?= bufbuild/buf:1.50.0
GO_IMAGE ?= golang:1.26-bookworm
MAVEN_IMAGE ?= maven:3.9.11-eclipse-temurin-25
GOLANGCI_LINT_MOD ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.1
GOLANGCI_LINT ?= golangci-lint
GO_TEST_FLAGS ?= ./...
GO_TEST_SHORT_FLAGS ?= -short ./...
export PATH := $(shell go env GOPATH)/bin:$(PATH)
ROOT := $(CURDIR)

ifeq ($(OS),Windows_NT)
BASH ?= "C:/Program Files/Git/bin/bash.exe"
GO_TEST_RUN = set CGO_ENABLED=0&& go test $(GO_TEST_FLAGS)
GO_TEST_SHORT_RUN = set CGO_ENABLED=0&& go test $(GO_TEST_SHORT_FLAGS)
# Host gcc is often missing on Windows; run -race in Docker (parity with Linux CI).
GATEWAY_RACE_RUN = docker run --rm -v "$(ROOT):/workspace" -w /workspace/src/backend/gateway $(GO_IMAGE) \
	bash -c "apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq libopus-dev libopusfile-dev libsoxr-dev pkg-config >/dev/null 2>&1 && CGO_ENABLED=1 go test -race $(GO_TEST_FLAGS)"
else
BASH ?= bash
GO_TEST_RUN = CGO_ENABLED=0 go test $(GO_TEST_FLAGS)
GO_TEST_SHORT_RUN = CGO_ENABLED=0 go test $(GO_TEST_SHORT_FLAGS)
GATEWAY_RACE_RUN = CGO_ENABLED=1 go test -race $(GO_TEST_FLAGS)
endif
GO_SERVICES := analytics bot chat federation file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice
# Dockerfiles with context=src/backend (sync scripts/ci/backend-docker-context.txt and ci.yml dockerctx).
GO_SERVICES_BACKEND_CONTEXT := gateway realtime chat messaging user social voice file role space bot matchmaking moderation notification search story subscription analytics federation
GO_MODULES_LINT := pkg $(GO_SERVICES)
GO_TEST_TARGETS := $(GO_SERVICES:%=go-test-%)
GO_TEST_SHORT_TARGETS := $(GO_SERVICES:%=go-test-short-%)
GO_IMAGE_TARGETS := $(GO_SERVICES:%=go-image-%)

.PHONY: buf-lint buf-format buf-breaking buf-generate buf-generate-dart buf-dart-check sync-pb-from-gen buf-generate-all compose-up compose-app-up compose-down compose-logs-collect compose-observability-up \
	compose-migrate-all compose-migrate-e2e compose-migrate-bot compose-migrate-story compose-e2e-smoke compose-e2e-live compose-e2e-full compose-e2e-voice-live \
	build-all build-all-breaking check-toolchain compose-config-ci buf-ci backend-test-ci backend-test-ci-short backend-image-ci \
	gateway-test-ci gateway-image-ci go-test-pkg go-mod-tidy-all auth-test-ci auth-image-ci buf-breaking-ci \
	golangci-ci gateway-test-race-ci design-tokens-check flutter-ui-color-gate flutter-ci flutter-windows-prefetch-sqlite3 flutter-linux-prefetch-sqlite3 prekey-golden-check coverage-report testcontainers-prune buf-generate-ci-local-template-check \
	staging-matrix-test generate-staging-services

buf-lint:
	buf lint

buf-format:
	buf format -w .

# Requires fetch-depth 0 and master ref (CI); locally: against origin/master if present
buf-breaking:
	buf breaking protos --against ".git#branch=master,subdir=protos"

# Go: scratch output under gen/ (gitignored). Sync into src/backend/*/pb: make sync-pb-from-gen — see docs/REPOSITORIES.md.
buf-generate:
	buf generate --template buf.gen.local-go.yaml

sync-pb-from-gen:
	$(BASH) "$(ROOT)/scripts/dev/sync-pb-from-gen.sh"

# buf-generate + sync committed pb/ trees (proto-change workflow).
buf-generate-all: buf-generate sync-pb-from-gen

# Remote BSR plugins (requires network). Output: gen/go (gitignored).
buf-generate-bsr:
	buf generate

# Dart: committed stubs under src/frontend/lib/gen (scripts/ci/buf-generate-dart.sh).
buf-generate-dart:
	$(BASH) "$(ROOT)/scripts/ci/buf-generate-dart.sh"

# CI / pre-PR: regenerate Dart stubs and fail if src/frontend/lib/gen drifts from protos/.
buf-dart-check:
	$(BASH) "$(ROOT)/scripts/ci/buf-generate-dart.sh"
	@git diff --exit-code -- src/frontend/lib/gen || (echo "Run make buf-generate-dart and commit src/frontend/lib/gen" >&2; exit 1)

compose-up:
	docker compose up -d

compose-app-up:
	COMPOSE_PARALLEL_LIMIT=4 docker compose --profile app up -d --build

# golang-migrate on Compose Postgres (network auto-detected). auth_db: Flyway on Auth boot by default — see migrations README.
compose-migrate-all:
	$(BASH) "$(ROOT)/scripts/dev/compose-migrate-all.sh" all

compose-migrate-e2e:
	$(BASH) "$(ROOT)/scripts/dev/compose-migrate-all.sh" e2e

compose-migrate-bot:
	$(BASH) "$(ROOT)/scripts/dev/compose-migrate-all.sh" bot

compose-migrate-story:
	$(BASH) "$(ROOT)/scripts/dev/compose-migrate-all.sh" story

compose-down:
	docker compose down

ifeq ($(OS),Windows_NT)
COMPOSE_LOGS_COLLECT = powershell -NoProfile -ExecutionPolicy Bypass -File "$(ROOT)/scripts/dev/collect-compose-logs.ps1"
else
COMPOSE_LOGS_COLLECT = $(BASH) "$(ROOT)/scripts/dev/collect-compose-logs.sh"
endif

compose-logs-collect:
	$(COMPOSE_LOGS_COLLECT)

# App stack + observability profile (Prometheus, Grafana, Loki, Promtail). Independent of compose-logs-collect.
compose-observability-up:
	COMPOSE_PARALLEL_LIMIT=4 docker compose --profile app --profile observability up -d --build

# Feature smoke E2E (one test per product area; same as CI compose-e2e on master).
compose-e2e-smoke:
	$(BASH) "$(ROOT)/scripts/ci/compose-e2e-smoke.sh"

# Opt-in compose E2E full coverage (all live tests; see .github/ci/e2e-features.yml).
compose-e2e-live:
	$(BASH) "$(ROOT)/scripts/ci/compose-e2e-live.sh"

compose-e2e-full: compose-e2e-live

compose-e2e-voice-live: compose-e2e-full

# --- CI parity: host Go/Maven/golangci (tests need Docker socket for testcontainers); Docker for buf/compose/images ---

check-toolchain:
	$(BASH) "$(ROOT)/scripts/ci/check-toolchain.sh"

compose-config-ci:
	docker compose config --quiet
	docker compose config --format json | docker run --rm -i -v "$(ROOT):/w" ghcr.io/jqlang/jq:1.7 -e -f /w/scripts/ci/compose-nats-jetstream.jq

buf-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf lint && buf format -d --exit-code"

backend-test-ci: go-mod-tidy-all go-test-pkg $(GO_TEST_TARGETS) auth-test-ci testcontainers-prune

backend-test-ci-short: go-mod-tidy-all go-test-pkg $(GO_TEST_SHORT_TARGETS) auth-test-ci testcontainers-prune

go-mod-tidy-all:
	$(BASH) -c 'for m in $(GO_MODULES_LINT); do \
		echo "== go mod tidy $$m =="; \
		(cd src/backend/$$m && go mod tidy); \
	done'

backend-image-ci: $(GO_IMAGE_TARGETS) auth-image-ci

# Remove testcontainers leftovers (Ryuk may be disabled on Windows). Does not touch compose stacks.
testcontainers-prune:
	$(BASH) "$(ROOT)/scripts/ci/testcontainers-prune.sh"

go-test-pkg:
	cd "$(ROOT)/src/backend/pkg" && $(GO_TEST_RUN)

go-test-%:
	cd "$(ROOT)/src/backend/$*" && $(GO_TEST_RUN)

go-test-short-%:
	cd "$(ROOT)/src/backend/$*" && $(GO_TEST_SHORT_RUN)

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

contrast-tokens-check:
	$(BASH) "$(ROOT)/scripts/design/contrast-tokens-check.sh"

flutter-ui-color-gate:
	$(BASH) "$(ROOT)/scripts/design/flutter-ui-color-gate.sh"

# Host Flutter SDK (parity with job `flutter` in .github/workflows/ci.yml).
ifeq ($(OS),Windows_NT)
flutter-windows-prefetch-sqlite3:
	powershell -NoProfile -ExecutionPolicy Bypass -File "$(ROOT)/scripts/ci/flutter-windows-prefetch-sqlite3.ps1"
flutter-linux-prefetch-sqlite3:
	@true
FLUTTER_SQLITE_PREFETCH := flutter-windows-prefetch-sqlite3
else
flutter-windows-prefetch-sqlite3:
	@true
flutter-linux-prefetch-sqlite3:
	$(BASH) "$(ROOT)/scripts/ci/flutter-linux-prefetch-sqlite3.sh"
FLUTTER_SQLITE_PREFETCH := flutter-linux-prefetch-sqlite3
endif

prekey-golden-check: $(FLUTTER_SQLITE_PREFETCH)
	cd $(ROOT)/src/frontend && flutter test test/tools/prekey_golden_drift_test.dart

flutter-ci: design-tokens-check contrast-tokens-check flutter-ui-color-gate buf-dart-check prekey-golden-check
	cd $(ROOT)/src/frontend && flutter pub get && flutter analyze --no-fatal-infos && flutter test

# Go (-coverprofile), Auth (JaCoCo), Flutter (lcov). Writes .local/coverage/summary.txt
coverage-report:
	$(BASH) "$(ROOT)/scripts/ci/coverage-report.sh" "$(ROOT)"

buf-breaking-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf breaking protos --against '.git#branch=master,subdir=protos'"

buf-generate-ci:
	# Smoke: buf generate to gen/go (gitignored); no committed pb drift check.
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf generate --template buf.gen.local-go.yaml"

buf-generate-ci-local-template-check:
	$(BASH) "$(ROOT)/scripts/ci/buf-generate-ci-local-template_test.sh"

staging-matrix-test:
	$(BASH) "$(ROOT)/scripts/ci/resolve-staging-matrix_test.sh"

generate-staging-services:
	$(BASH) "$(ROOT)/scripts/ci/generate-staging-go-services.sh"

# Same as build-all plus protobuf compatibility vs master (fails if master ref missing)
build-all-breaking: build-all buf-breaking-ci
