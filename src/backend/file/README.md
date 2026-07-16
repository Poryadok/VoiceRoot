# File Service

Go service for chat attachments (file-storage (docs/features/file-storage.md)+) and avatar-adjacent uploads via R2/MinIO.

Current public surface:

- GET /health — `{"service":"file","status":"ok"}`.
- gRPC `FileService.RequestUpload` — pending `file_db.files` row + presigned PUT (50 MiB free tier).
- gRPC `FileService.ConfirmUpload` — server-side SHA-256 verify (R2 read), optional ClamAV scan (`CLAMAV_ADDR`), image resize + thumbnail upload (`internal/imgproc`); original R2 object removed after image processing.
- gRPC `FileService.GetFileURL` — presigned GET for ready files (prefers `converted_r2_key` when present).
- gRPC `FileService.DeleteFile` — R2 purge + DB soft-delete.
- Background expiry worker (`internal/jobs`) — retention purge, `file.expired` NATS when `NATS_URL` is set.
- gRPC `FileService` quota/list — see `file_grpc.go` and `file_service_upload_test.go`.

Compose: `file` + `clamav` services in `--profile app`; set `FILE_R2_*` in `.env` (see `.env.example`). Optional `FILE_RETENTION_DEV` shortens retention TTL in dev (seconds).

Still out of scope: deduplication, full `file.events` fan-out (`uploaded`/`processed`/`downloaded`), subscription downgrade retention adjustment.