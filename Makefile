.PHONY: buf-lint buf-format buf-breaking buf-generate compose-up compose-down \
	build-all build-all-breaking compose-config-ci buf-ci gateway-test-ci gateway-image-ci buf-breaking-ci

# Container images (pin for CI-like reproducibility; bump with README toolchain table)
BUF_IMAGE ?= bufbuild/buf:1.50.0
GO_IMAGE ?= golang:1.26-bookworm
ROOT := $(CURDIR)

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

gateway-test-ci:
	docker run --rm -v "$(ROOT):/workspace" -w /workspace/src/backend/gateway $(GO_IMAGE) \
		sh -c "CGO_ENABLED=0 go test ./..."

gateway-image-ci:
	docker build -f src/backend/gateway/Dockerfile -t voice-gateway:local src/backend/gateway

# Full local CI stack in containers (no buf breaking: needs local master ref)
build-all: compose-config-ci buf-ci gateway-test-ci gateway-image-ci

buf-breaking-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf breaking protos --against '.git#branch=master,subdir=protos'"

buf-generate-ci:
	docker run --rm --entrypoint sh -v "$(ROOT):/workspace" -w /workspace $(BUF_IMAGE) \
		-c "buf generate"

# Same as build-all plus protobuf compatibility vs master (fails if master ref missing)
build-all-breaking: build-all buf-breaking-ci
