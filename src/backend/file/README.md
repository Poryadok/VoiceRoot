# File Service

Go scaffold for the Voice file service.

Current public surface:

- GET /health returns {"service":"file","status":"ok"}.
- gRPC `FileService.RequestUpload` creates a pending `file_db.files` row and returns a Cloudflare R2 presigned PUT URL with the Phase 3 free-tier 50 MiB limit.
- gRPC `FileService.GetFileURL` returns a 1h presigned GET URL for ready files owned by the caller profile.

Still out of scope for this increment: `ConfirmUpload`, chat access checks for `context_chat`, attachments wiring in Messaging, thumbnails/WebP conversion, ClamAV, deduplication, and `file.events`.