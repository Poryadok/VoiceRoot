.PHONY: buf-lint buf-format buf-breaking compose-up compose-down

buf-lint:
	buf lint

buf-format:
	buf format -w .

# Requires fetch-depth 0 and master ref (CI); locally: against origin/master if present
buf-breaking:
	buf breaking protos --against ".git#branch=master,subdir=protos"

compose-up:
	docker compose up -d

compose-down:
	docker compose down
