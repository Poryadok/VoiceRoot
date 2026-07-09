# File Service

Go service for chat attachments (file-storage (docs/features/file-storage.md)+) and avatar-adjacent uploads via R2/MinIO.

Current public surface:

- GET /health — `{"service":"file","status":"ok"}`.
- gRPC `FileService.RequestUpload` — pending `file_db.files` row + presigned PUT (50 MiB free tier).
- gRPC `FileService.ConfirmUpload` — SHA-256 verify, optional ClamAV scan (`CLAMAV_ADDR`), image resize + thumbnail upload (`internal/imgproc`).
- gRPC `FileService.GetFileURL` — presigned GET for ready files owned by the caller.
- gRPC `FileService` quota/list/delete — see `file_grpc.go` and `file_service_upload_test.go`.

Compose: `file` + `clamav` services in `--profile app`; set `FILE_R2_*` in `.env` (see `.env.example`).

Still out of scope: deduplication, `file.events` fan-out, subscription quotas beyond free tier.