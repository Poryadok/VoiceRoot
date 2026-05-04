# Container images (pin for CI-like reproducibility; bump with README toolchain table)
# Keep GOLANGCI_LINT_MOD in sync with go install version in .github/workflows/ci.yml (job golangci).
BUF_IMAGE ?= bufbuild/buf:1.50.0
GO_IMAGE ?= golang:1.26-bookworm
MAVEN_IMAGE ?= maven:3.9.11-eclipse-temurin-25
# Installed inside $(GO_IMAGE) so the binary matches Go 1.26 (official golangci image may lag).
GOLANGCI_LINT_MOD ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.1
ROOT := $(CURDIR)
GO_SERVICES := analytics bot chat federation file gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice
GO_MODULES_LINT := pkg $(GO_SERVICES)
GO_TEST_TARGETS := $(GO_SERVICES:%=go-test-%)
GO_IMAGE_TARGETS := $(GO_SERVICES:%=go-image-%)

.PHONY: buf-lint buf-format buf-breaking buf-generate compose-up compose-down \
	build-all build-all-breaking compose-config-ci buf-ci backend-test-ci backend-image-ci \
	gateway-test-ci gateway-image-ci go-test-pkg auth-test-ci auth-image-ci buf-breaking-ci \
	golangci-ci gateway-test-race-ci flutter-ci

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

compose-down:
	docker compose down

# --- Docker-based pipeline (parity with .github/workflows/ci.yml) ---

compose-config-ci:
	docker compose config --quiet

buf-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf lint && buf format -d --exit-code"

backend-test-ci: go-test-pkg $(GO_TEST_TARGETS) auth-test-ci

backend-image-ci: $(GO_IMAGE_TARGETS) auth-image-ci

go-test-pkg:
	docker run --rm -v "$(ROOT):/workspace" -w /workspace/src/backend/pkg $(GO_IMAGE) \
		sh -c "CGO_ENABLED=0 go test ./..."

go-test-%:
	docker run --rm -v "$(ROOT):/workspace" -w /workspace/src/backend/$* $(GO_IMAGE) \
		sh -c "CGO_ENABLED=0 go test ./..."

# gateway: go.mod replace ../pkg — Docker context is src/backend, not gateway/.
go-image-%:
	docker build -f src/backend/$(if $(filter gateway,$*),gateway,$*)/Dockerfile -t voice-$*:local $(if $(filter gateway,$*),src/backend,src/backend/$*)

gateway-test-ci:
	docker run --rm -v "$(ROOT):/workspace" -w /workspace/src/backend/gateway $(GO_IMAGE) \
		sh -c "CGO_ENABLED=0 go test ./..."

gateway-image-ci:
	docker build -f src/backend/gateway/Dockerfile -t voice-gateway:local src/backend

auth-test-ci:
	docker run --rm -v "$(ROOT):/workspace" -w /workspace/src/backend/auth $(MAVEN_IMAGE) \
		mvn -B test

auth-image-ci:
	docker build -f src/backend/auth/Dockerfile -t voice-auth:local src/backend/auth

golangci-ci:
	docker run --rm -v "$(ROOT):/workspace" -w /workspace $(GO_IMAGE) \
		sh -c 'GOBIN=/usr/local/bin go install $(GOLANGCI_LINT_MOD) && \
		for m in $(GO_MODULES_LINT); do \
			echo "== $$m ==" && cd "/workspace/src/backend/$$m" && golangci-lint run ./... || exit 1; \
		done'

gateway-test-race-ci:
	docker run --rm -v "$(ROOT):/workspace" -w /workspace/src/backend/gateway $(GO_IMAGE) \
		sh -c "CGO_ENABLED=1 go test -race ./..."

# Full local CI stack in containers (no buf breaking: needs local master ref).
# Flutter is not included (needs host SDK); run: make flutter-ci
build-all: compose-config-ci buf-ci backend-test-ci golangci-ci gateway-test-race-ci backend-image-ci

# Host Flutter SDK (parity with job `flutter` in .github/workflows/ci.yml).
flutter-ci:
	cd $(ROOT)/src/frontend && flutter pub get && flutter analyze && flutter test

buf-breaking-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf breaking protos --against '.git#branch=master,subdir=protos'"

buf-generate-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf generate"

# Same as build-all plus protobuf compatibility vs master (fails if master ref missing)
build-all-breaking: build-all buf-breaking-ci
