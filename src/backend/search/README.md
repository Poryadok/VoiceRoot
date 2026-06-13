# Search Service (Go)

PostgreSQL full-text search (Phase 9): indexes `message.events` / profile / space catalog; gRPC `SearchInChat`, `SearchGlobal`, `SearchUsers`, `SearchPublicSpaces`, `ReindexChat`.

HTTP via Gateway: `/api/v1/search/*` — see [transcode_search.go](../gateway/transcode_search.go).

Local: part of `docker compose --profile app`; DB `search_db`.
