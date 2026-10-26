# File Service

Go service for chat attachments (file-storage (docs/features/file-storage.md)+) and avatar-adjacent uploads via R2/MinIO.

Current public surface:

- GET /health — `{"service":"file","status":"ok"}`.
- gRPC `FileService.RequestUpload` — pending `file_db.files` row + presigned PUT (50 MiB free tier).
- gRPC `FileService.ConfirmUpload` — server-side SHA-256 verify (R2 read), optional ClamAV scan (`CLAMAV_ADDR`), image resize + thumbnail upload (`internal/imgproc`); original R2 object removed after image processing.
- gRPC `FileService.GetFileURL` — presigned GET for ready files (prefers `converted_r2_key` when present).
- gRPC `FileService.DeleteFile` — R2 purge + DB soft-delete.
- Background expiry worker (`internal/jobs`) — retention purge, `file.expired` NATS when `NATS_URL` is set.
- Plaintext PDFs remain `processing` after confirmation until the PDF worker writes a first-page PNG thumbnail; original document bytes remain the download. With database and R2 configured, startup registers this worker and requires migrations `000010`/`000011`, the active tracked-write GC guard, and Poppler `pdftoppm` (included in the File image). Missing dependencies stop startup.
- gRPC `FileService` quota/list — see `file_grpc.go` and `file_service_upload_test.go`.

Compose: `file` + `clamav` services in `--profile app`; set `FILE_R2_*` in `.env` (see `.env.example`). Optional `FILE_RETENTION_DEV` shortens retention TTL in dev (seconds).

Still out of scope: deduplication, full `file.events` fan-out (`uploaded`/`processed`/`downloaded`), subscription downgrade retention adjustment.

Music uses explicit `UPLOAD_INTENT_MUSIC` (6), persisted at upload admission;
unspecified intent preserves legacy MIME processing. Music requires chat context
and audio MIME, with a 10 MiB stored-byte limit including E2E overhead. Plaintext
MP3, FLAC, Ogg (Opus/Vorbis/FLAC) and PCM/IEEE-float WAVE use the bounded ffprobe
extractor. Unsupported codecs fail processing, even inside an admitted container.
Optional title/artist/album and duration are committed with a monotonic revision
and `file.processed` outbox snapshot. Delete/expiry commits a newer metadata-free
tombstone. Messaging refresh is ordered eventual revocation; File authorization
remains authoritative for every download.

Encrypted Music bypasses the extractor and publishes no public tags. The client
decrypts and decodes locally with the same supported-format policy. Music
consumer/grant activation must precede the new File producer; see the root NATS
v4 activation runbook. Unit fixtures do not replace native preservation or
platform playback evidence.
