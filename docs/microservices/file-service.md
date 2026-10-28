# File Service

## Обзор

Загрузка, хранение, конвертация и раздача файлов. Все медиа проходят через этот сервис.

**Язык**: Go
**БД**: PostgreSQL `file_db`
**Хранилище**: Cloudflare R2 (S3-совместимое)

**Scope:** все медиа проходят через этот сервис. Аватар профиля может использовать R2 presigned в контуре User до полного развёртывания File Service — см. [user-profile.md](../features/user-profile.md), [PLAN.md](../PLAN.md).

## File → User retention-owner principal

File signs short-lived request-bound `service:file` credentials only for
`ResolveAccountIDForProfile`, with issuer/audience `file`, and calls User's
dedicated TLS listener (normally `voice-user:9092`). Configure exactly two
distinct RSA PKCS#8 keys (at least 2048 bits) in
`FILE_PRINCIPAL_SIGNING_KEYS_DIR`, select one with
`FILE_PRINCIPAL_ACTIVE_KID`, and expose both public keys over TLS with
`FILE_PRINCIPAL_JWKS_LISTEN`, `FILE_PRINCIPAL_JWKS_TLS_CERT_FILE` and
`FILE_PRINCIPAL_JWKS_TLS_KEY_FILE`. The User client requires all of
`USER_FILE_PRINCIPAL_GRPC_ADDR`, `USER_FILE_PRINCIPAL_TLS_CA_FILE`, and
`USER_FILE_PRINCIPAL_TLS_SERVER_NAME`; retries are disabled and TLS is at least
1.2. No setting reuses the inbound Story verifier namespace.

For rotation publish current and next public keys first, wait through the
credential hard-expiry, then change the active key. Retain a retired public key
for at least 35 seconds (30-second credential lifetime plus 5-second skew).

## Ответственность

- Upload файлов (presigned URL → R2)
- Download / раздача (presigned URL или CDN proxy)
- Автоконвертация:
  - Изображения → WebP
  - GIF → MP4/WebM
  - Видео → 720p H.264
  - Документы → без изменений
- Генерация превью (thumbnail, PDF первая страница, иконка по типу)
- Лимиты размера: 50 MB free / 200 MB paid (per file)
- Retention: 90 дней (free) / бессрочно (paid); E2E чаты — 90 дней всегда
- SHA-256 дедупликация (один файл в R2, несколько ссылок)
- ClamAV антивирус для исполняемых файлов (.exe, .zip, .bat)
- Expired files → placeholder "файл удалён"

## API (gRPC)

Источник истины: [protos/voice/file/v1/file.proto](../../protos/voice/file/v1/file.proto). **`GetFileURLResponse`** содержит напрямую `presigned_get_url` и `expires_at` (`google.protobuf.Timestamp`).

```protobuf
service FileService {
  // Protected, read-only Story media attestation. Only service:story may call.
  rpc ValidateStoryMedia(ValidateStoryMediaRequest) returns (ValidateStoryMediaResponse);
  rpc RequestUpload(RequestUploadRequest) returns (RequestUploadResponse);
  rpc ConfirmUpload(ConfirmUploadRequest) returns (ConfirmUploadResponse);
  rpc GetFileURL(GetFileURLRequest) returns (GetFileURLResponse);
  rpc GetFileMetadata(GetFileMetadataRequest) returns (GetFileMetadataResponse);
  rpc GetBulkMetadata(GetBulkMetadataRequest) returns (GetBulkMetadataResponse);
  rpc DeleteFile(DeleteFileRequest) returns (DeleteFileResponse);
  rpc ListFiles(ListFilesRequest) returns (ListFilesResponse);
  rpc CheckQuota(CheckQuotaRequest) returns (CheckQuotaResponse);
}
```

### Thumbnail URL variant

The next compatible extension of `GetFileURLRequest` is:

```protobuf
enum FileURLVariant {
  FILE_URL_VARIANT_UNSPECIFIED = 0;
  FILE_URL_VARIANT_THUMBNAIL = 1;
}

message GetFileURLRequest {
  string file_id = 1;
  FileAccessSelector access = 2;
  FileURLVariant variant = 3;
}
```

`FILE_URL_VARIANT_UNSPECIFIED` preserves the delivered selection rule:
`converted_r2_key`, when present, otherwise the original deliverable. The
existing `GetFileURLResponse` remains exactly `presigned_get_url` plus
`expires_at`; its one-hour TTL is unchanged.

`FILE_URL_VARIANT_THUMBNAIL` selects **only** `thumbnail_r2_key`. It must never
fall back to `converted_r2_key`, `r2_key`, an original, a generated placeholder,
or an R2 key exposed to the caller. An absent thumbnail returns gRPC
`FailedPrecondition`; any unrecognised enum value returns `InvalidArgument`.

Plaintext `application/pdf` documents use the existing `processing` state as a
durable asynchronous thumbnail queue. The worker renders only page one to PNG
(at most 320 pixels per edge and 1 MiB), with a 10 MiB input bound, a ten-second
renderer deadline and one renderer per process. Poppler `pdftoppm` must be
installed in the File runtime; the File image includes `poppler-utils`.
Startup registers the worker when database and R2 storage are configured and
refuses missing renderer/schema dependencies or a disabled tracked-write GC
guard. Encrypted documents bypass server rendering.
The worker rechecks the confirmed SHA-256 before rendering. The original PDF,
SHA-256, size and retention deadline remain unchanged.
Malformed PDFs become `failed`; temporary storage/tool failures remain
`processing` and retry. For [Poppler's exit contract](https://manpages.debian.org/bookworm/poppler-utils/pdftoppm.1.en.html),
PDF-open (1) and PDF-permission (3) failures are permanent; output-open (2),
other (99), unknown exits and killed processes remain retryable. Cancellation
also leaves the document retryable. Apply migrations `000010` and `000011`
before activating the PDF worker. It uses the shared File tracked-write ledger,
with a unique immutable thumbnail attempt key, measured digest/size and
`pdf_thumbnail` purpose. `STARTED` commits before the single storage attempt;
hidden SDK retries and redirects are disabled. An exact successful ACK settles
physical custody independently of caller cancellation or a lost SQL session.
Publication separately rechecks the File, attempt key, blob and references;
logical deletion or release cannot resurrect `ready`. Settled publication can
resume without another PUT. Ready metadata and its redacted preview outbox
event commit together.

Any missing/ambiguous storage ACK remains a non-expiring physical cleanup pin.
HEAD absence, a timer, process restart or an administrator assertion cannot clear
it. Reference release and logical deletion can proceed while physical deletion
remains pending. The worker reports unresolved write counts at most once per
minute; actual remote settlement/fencing proof is required for reconciliation.
This is an explicit recovery limitation, not automatic orphan cleanup success.
See [the shared tracked-write contract](../testing/file-tracked-write-contract.md).

The request retains `FileAccessSelector`, and File performs the same ACL,
reference/capability, file-state and `FILE_READ_SURFACE_URL` checks before
creating either URL. The returned URL is presigned; direct R2 object URLs or
keys are not public API.

## Модель данных

```
files
├── id (UUID)
├── uploader_profile_id
├── original_name
├── mime_type
├── size_bytes
├── sha256_hash
├── r2_key (string — path in R2)
├── status (pending_upload | processing | ready | failed | deleted | expired)
├── type (image | video | audio | document | other)
├── width (nullable, for images/video)
├── height (nullable)
├── duration_seconds (nullable, for audio/video)
├── thumbnail_r2_key (nullable)
├── converted_r2_key (nullable — WebP/MP4 version)
├── chat_id (nullable)
├── chat_type (dm | group | channel)
├── is_e2e (bool)
├── expires_at (nullable)
├── scan_result (pending | clean | infected | error | skipped)
├── created_at
└── updated_at

file_references (дедупликация)
├── file_id (FK)
├── message_id (FK)
├── chat_id
└── created_at
```

## Pipeline обработки

Сводка по типам — полная таблица в [file-storage.md](../features/file-storage.md) § «Автоматическая обработка при загрузке».

| Upload category (`RequestUploadRequest.intent`) | Обработка | Max stored |
|-------------------------------------------------|-----------|------------|
| `image` | WebP quality 80–85% | ≤5 MB |
| `gif` | MP4/WebM (no audio); provider import or user upload | ≤5 MB |
| `sticker` | Static → WebP; animated → WebP or MP4/WebM (no audio) | ≤512 KB per asset |
| `video` | ffmpeg 720p H.264 | ≤15 MB |
| `video_note` | Square crop, ≤**60 s** | ≤5 MB |
| `music` / `audio` | Metadata extract (title/artist/album) | ≤10 MB |
| `document` | As-is + optional PDF thumb | ≤10 MB |
| `article_thumb` | OG/thumb for Article attach | ≤2 MB |
| `location_map` | Static map tile | ≤500 KB |

**`intent` field:** `RequestUploadRequest.intent` implements `UNSPECIFIED` (legacy MIME processing), `GIF` (2), `STICKER` (3, static only), `MUSIC` (6), and `AUDIO` (7). Intent is persisted at request admission and cannot be changed at confirm. GIF is an authenticated upload API slice described below; provider imports, picker and GIF messaging are separate. Other values below remain the target specification, not implemented processing branches. Music and Audio require chat context and ≤10 MiB stored bytes, including encrypted overhead. Audio admits MP3, FLAC, Ogg, and WAVE MIME types. Plaintext processing supports MP3, FLAC, Ogg Opus/Vorbis/FLAC and PCM/IEEE-float WAVE; encrypted files bypass server extraction and derive tags only in the client. Audio metadata uses the File metadata fields and generic processed notification; only Music emits the Music stream event. The remaining intended values are:

```protobuf
enum UploadIntent {
  UPLOAD_INTENT_UNSPECIFIED = 0;
  UPLOAD_INTENT_IMAGE = 1;
  UPLOAD_INTENT_GIF = 2;
  UPLOAD_INTENT_STICKER = 3;      // static or animated WebP — same intent
  UPLOAD_INTENT_VIDEO = 4;        // attach-menu video → 720p H.264
  UPLOAD_INTENT_VIDEO_NOTE = 5;   // composer hold-to-record → square crop, ≤60 s
  UPLOAD_INTENT_MUSIC = 6;
  UPLOAD_INTENT_AUDIO = 7;
  UPLOAD_INTENT_DOCUMENT = 8;
  UPLOAD_INTENT_ARTICLE_THUMB = 9;
  UPLOAD_INTENT_LOCATION_MAP = 10;
}

message RequestUploadRequest {
  // … existing fields (original_name, mime_type, size_bytes, context_chat, is_e2e) …
  UploadIntent intent = 8;
  // Remote URLs are not public upload parameters; see protected catalog import.
}
```

**GIF provider import:** Chat resolves the selected provider identity and calls the protected `CatalogAssetService.ImportCatalogAsset`; a public `RequestUpload` or client URL does not authorize a fetch. File stores source identity and actual byte facts, then uses its GIF pipeline. User-uploaded GIF retains presigned PUT. See the [non-E2E catalog protocol](../testing/be085-catalog-asset-contract.md) for the pending implementation and caller matrix.

| Composer / attach path | Required `intent` | Wrong intent → |
|------------------------|-------------------|----------------|
| 📎 Video attach | `video` | `video_note` applies square crop + 60 s cap |
| Composer video note (hold-to-record) | `video_note` | `video` skips square crop / duration guard |
| 😊 GIF tab (provider or upload) | `gif` | — |
| Sticker pack asset | `sticker` | — |

Sticker pack assets use `sticker`; GIF messages (composer 😊 panel or provider import) use `gif`. **Ownership:** File stores/converts bytes only; **Chat** owns pack catalog + `SearchGifs`; **Messaging** owns send payload — [messaging-service.md](messaging-service.md) § Stickers and GIF, [chat-service.md](chat-service.md) § Sticker packs.

**Post-transcode `files.type` (normative):**

| `UploadIntent` | After processing | `files.type` | Notes |
|----------------|------------------|--------------|-------|
| `gif` | MP4/WebM in `converted_r2_key` | **`video`** | Messaging validates GIF send against `file_id` + `intent=gif` metadata, not `type=image` |
| `sticker` (static) | WebP in `converted_r2_key` or `r2_key` | **`image`** | |
| `sticker` (animated) | WebP animated or MP4/WebM | **`image`** or **`video`** | Same `UPLOAD_INTENT_STICKER`; worker picks branch from mime |

Do **not** leave post-transcode GIF rows as `other` — breaks Messaging `ListSharedMedia` / validation predicates.

```
Client ──presigned URL──► R2 (upload)
         │
         ▼
    ConfirmUpload (intent → worker queue)
         │
         ▼
  ┌──────────────┐
  │ Scan (ClamAV) │──infected──► mark as infected, notify
  └──────┬───────┘
         │ clean
         ▼
  ┌──────────────┐
  │ Convert      │  branch by intent: image→WebP, GIF→MP4, video→720p, video_note→square crop
  └──────┬───────┘
         │
         ▼
  ┌──────────────┐
  │ Thumbnail    │  generate preview
  └──────┬───────┘
         │
         ▼
    status = ready
    publish file.processed event → Messaging preview refresh ([messaging-service.md](messaging-service.md) § File → Messaging)
```

## Stickers and GIF assets

Normative product scope: [text-chat.md](../features/text-chat.md) § «Медиа и вложения»; wire payloads — [messaging-service.md](messaging-service.md) § Stickers and GIF; pack catalog — [chat-service.md](chat-service.md) § Sticker packs. GIF processing exists; protected catalog import and sticker ingest remain separate unfinished work. Stickers/GIF are **not** 📎 attach-menu uploads — composer **😊 panel** only.

### Ownership split

| Concern | Owner | File Service role |
|---------|-------|-------------------|
| Sticker pack catalog / install | **Chat** (`sticker_packs`, `profile_installed_packs`) | Stores per-sticker binary; returns `file_id` |
| GIF provider search | **Chat** (`SearchGifs` — selected provider adapter) | Resolve selection through Chat; protected File import or exact authorized source binding; cached `file_id` alone grants nothing |
| Send payload / `content_type` | **Messaging** `SendMessage` | `file_id` references processed asset |
| Composer picker UI | Flutter §3.6b | — |

### Static sticker upload implementation (BE-086)

`RequestUpload(intent=STICKER)` admits plaintext PNG/WebP with the existing uploader, optional chat ACL and upload-size policy. E2E and Story uploads are rejected. Confirmation verifies SHA-256, rejects GIF, APNG and animated WebP container bytes, scans the source and uses the existing image processor to produce a static WebP on a 512 by 512 transparent canvas, preserving aspect ratio. Invalid source, scan and processing failures cannot become ready. Processing and scanning must be configured. Sticker metadata retains its intent; the closed generic attachment preview v1/v2 serializer and resolver exclude stickers, as they exclude music. The existing legacy `file.processed` notification remains published.

Processing consumes the exact source bytes validated and scanned by confirmation, without rereading the mutable upload key. Both sticker derivatives must fit the 512 KiB asset ceiling before either is written; ordinary images retain their existing 5 MiB cap.

The animated sticker claims below remain an unresolved contract discrepancy: normative `text-chat.md` limits user packs to static PNG/WebP. This slice does not activate animated stickers, Chat pack operations or Messaging shared media.

### Upload flows

**Sticker (user pack or system seed):**

```
Client → RequestUpload(intent=sticker, mime, size)  // animated: same intent; worker branches on mime
      → presigned PUT → R2
      → ConfirmUpload → worker: resize 512×512 WebP, scan, ready
      → file_id returned to Chat AddStickersToUserPack
```

**GIF (composer 😊 panel — provider search only; not 📎 attach menu):**

```
Client → Chat.SearchGifs(query) → provider adapter (Giphy or Tenor — one at deploy)
      → user picks result
      → Chat ResolveGifAsset (stable operation, provider identity, destination)
      → protected File ImportCatalogAsset → GIF processing, MP4/WebM and thumb
      → Messaging SendMessage(content_type=GIF, payload { file_id, provider, provider_id, preview_url })
```

**Rules:**

| Rule | Value |
|------|-------|
| Sticker dimensions | 512×512 px max; preserve aspect; pad transparent |
| Animated sticker | WebP animated preferred; reject if >512 KB after encode |
| GIF transcode | Same pipeline as composer GIF / user-upload GIF ([file-storage.md](../features/file-storage.md)); no audio track |
| Dedup | SHA-256 per binary; provider GIFs may share `file_id` across chats |
| Retention | Same as chat media (90 d free / paid unlimited); E2E chat 90 d always |
| Premium ★ packs | Entitlement gate on **install** in Chat, not on File storage |

`file.processed` for sticker/GIF intents triggers Messaging list preview refresh ([messaging-service.md](messaging-service.md) § File → Messaging).

### API-only GIF processing (BE-033)

`RequestUpload(intent=GIF, mime_type="image/gif")` accepts plaintext source
uploads under the existing uploader/chat authorization and upload limits. E2E,
Story context and provider source URLs are not admitted by this API slice.
The existing Flutter A1 `.gif` attachment fallback remains octet-stream/other.
`UNSPECIFIED` with `image/gif` is rejected instead of decoding a static first
frame. Existing pending rows are not migrated or reinterpreted by this guard.
New GIF-intent files cannot be sent or forwarded by relabeling them as generic
video or stickers; Messaging rejects them until its GIF-send contract is enabled.

Confirmation verifies source bytes and atomically stores the source hash and
one durable processing job. Repeating the same hash returns current metadata;
a different hash fails with `FailedPrecondition`. Processing is asynchronous:
poll metadata until `ready` or `failed`. The ordinary File runtime owns the job
worker; no live provider/key is needed. Recovery claims a fresh attempt token
after lease expiry. Only the live attempt can commit ready state, delivered
descriptors and the preview outbox together. Attempts use distinct storage keys.

Source `original_name`, `mime_type`, `size_bytes`, `sha256_hash` and
`object_revision=1` retain source provenance. `delivered_full` and
`delivered_thumbnail` describe the actual MIME, size, hash, filename and processor
version of output bytes. They contain no public URL or storage key. Clients use
these descriptors to validate delivered downloads; source SHA is not an MP4 hash.
The selected key, read bound, MIME and filename travel together through presigned
and revocable downloads. Missing GIF output fails closed without original fallback.
Existing `CheckQuota` remains source-size accounting, not physical R2 usage.

Processor `gif-mp4-v1` produces H.264/yuv420p MP4 without audio, faststart and a
static first-composited-frame WebP thumbnail. Each output is at most 5 MiB.
Frame timing and GIF disposal are preserved, with transparent pixels composited
on black and odd dimensions padded to even edges. Zero delays normalize to
100 ms; positive delays preserve centisecond timing. One terminal 10 ms packet
closes the final frame hold. Metadata duration rounds the source duration up to
whole seconds. Technical rejection budgets: 4096-pixel canvas edge, 16 Mi pixels
per canvas, 64 Mi cumulative canvas pixels, 600 frames, 120 seconds and the
existing upload byte cap. Codec execution is bounded to 90 seconds; a whole
attempt has 120 seconds, its lease 180 seconds, and stale-key cleanup waits 240
seconds from claim. Malformed or over-budget inputs become failed; transient
storage/codec failures remain recoverable processing jobs.

Every attempt declares keys durably before writing. Successful completion queues
original cleanup only after the metadata commit; ambiguous storage/commit failure
leaves durable cleanup evidence. Cleanup retries never delete a committed winner
or another live alias's original. Normal File reference GC still owns retained
full/thumb deletion, with existing ACL and retention ownership unchanged.
An expired signed upload URL does not prove that a previously accepted PUT has
finished. Public upload clients supply no trusted storage drain/revocation proof.
Original cleanup therefore remains durable unresolved reconciliation and repeats
deletion every minute even after URL expiry; old clock-completed original rows
are rediscovered. The legacy `recheck_until` value is not a completion fence.
Successful DELETE or HEAD absence cannot authorize final completion while such
a request may still commit. No finite physical-erasure bound is claimed without
an actual storage-side write fence. Exact tracked catalog/derivative custody
retains its separate ACK/unsettled checks. Processing/status/download never fall
back to the original during reconciliation.

Apply File migration `000009` before the binary. DOWN refuses to discard retained
GIF jobs, outputs or cleanup evidence. Deploy File/Messaging accepting the shared
GIF preview protocol v2 before using the new GIF upload producer. Legacy preview
v1 remains unchanged and excludes nonempty intents. GIF v2 terminal snapshots
remain redacted; ready snapshots require video plus converted/thumbnail variants.
This does not advertise first-class GIF messaging or completion of A6.

### Protected catalog import (non-E2E)

The separate `voice.file.v1.CatalogAssetService` protocol is defined in
`catalog_assets.proto` and the [concrete contract](../testing/be085-catalog-asset-contract.md).
Chat alone imports and polls scoped operation/asset status; Messaging alone
classifies File IDs and validates source-scoped READY metadata. These methods
are unavailable on the ordinary listener. Optional client provider attribution
never selects the authorization path. Ordinary upload/read ACL remains unchanged.

Migration 10 adds private actor-generation gates, active source mappings,
completed import details, purpose-HMAC terminal fences and declared write/GC
work. Raw source URLs are never persisted. An internal catalog-pending File may
have unknown size zero only in that marked pending stage; public metadata/bulk/
list queries hide it. Actual positive size and digest precede object write and
processing. The database rejects READY publication after the actor gate closes
or changes generation. These gates do not fence outstanding object-store PUTs;
unsettled writes remain reconciliation work until actual drain/fence evidence.

The byte transport, protocol/caller boundaries, schema, predicate and typed
Messaging consumer are source components. The actual import/status handlers,
runtime wiring, actor purge/recovery consumers and complete delivery proof remain
unfinished in this slice. No provider or E2E branch is activated by this schema.

### Messaging send payload cross-ref (R2-A32)

After `status=ready`, `file_id` is referenced from Messaging `SendMessage` — File Service does **not** embed pack/catalog IDs:

| `content_type` | Messaging `content_payload` (canonical) | File fields consumed |
|----------------|----------------------------------------|----------------------|
| `STICKER` | `{ pack_id, sticker_id, file_id, emoji?, width, height }` | `file_id` → thumb via presigned URL; dimensions from File metadata |
| `GIF` | `{ file_id, provider?, provider_id?, preview_url?, width?, height?, duration_seconds? }` | `file_id` → `converted_r2_key` (MP4/WebM) + thumb |

Validation owner: **Messaging** (pack installed, sticker row match) + **File** (row exists, `type` compatible, `status=ready`). Full rules — [messaging-service.md](messaging-service.md) § Stickers and GIF.

### Async processing → Messaging preview refresh

When conversion finishes (`status=ready` or `failed`), File publishes **`file.processed`** on `file.events`:

| Payload field | Consumer use |
|---------------|--------------|
| `file_id` | Lookup messages referencing attachment |
| `status` | `ready` → swap thumb/dimensions; `failed` → composer error state |
| `preview` | Complete File-owned v1 snapshot with independent positive revision, category, persisted intent, size/scan, optional dimensions/duration and variant availability; never URLs/storage keys |

**BE-080 contract:** File commits state, preview revision and stable event bytes
in one transaction. Generic snapshots exclude Music and E2E. The new protected
Messaging-only exact-reference resolver refreshes an existing MESSAGE projection,
without creating a reference or authorizing a viewer. Terminal snapshots expose
only identity/revision/status. Messaging privately binds attachment slots and
updates its ordinary message.edited outbox atomically; URL resolution retains
real-viewer authorization. Existing Music remains a separate mutually exclusive
payload and revision namespace. This extension does not create processors for
unsupported intents. Implementation and staged activation are tracked in the
[BE-080 plan](../testing/be080-file-preview-exec-plan.md).

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`file.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие              | Данные                                    |
|----------------------|-------------------------------------------|
| `file.uploaded`      | file_id, uploader_id, type, size          |
| `file.processed`     | file_id, status, mutually exclusive redacted revisioned preview or existing Music snapshot; no URL |
| `file.scan_infected` | file_id, uploader_id                      |
| `file.expired`       | file_id, chat_id                          |
| `file.downloaded`    | `file_id`, `downloader_profile_id`        |

`file.downloaded` означает не подтверждённую передачу байтов, а успешное
намерение скачать: авторизованный профиль запросил `GetFileURL` для доступного
файла в статусе `ready`, и File Service успешно создал presigned GET URL. На
ошибках авторизации, доступа, статуса файла или presign событие не публикуется.
Публикация выполняется best-effort: ошибка NATS логируется, но уже успешно
созданный `GetFileURLResponse` остаётся успешным.

## Зависимости

- **Cloudflare R2** — объектное хранилище
- **ClamAV** — антивирусное сканирование (sidecar или отдельный pod)
- **Subscription Service** — проверка лимитов (размер файла, retention)
- **Messaging Service** — (через NATS) обновление превью в сообщении после конвертации

## Story media validation

`ValidateStoryMedia` is a synchronous, read-only protected RPC. It accepts only
the verified `service:story` principal and a request-bound Phase-0 credential;
it never trusts forwarded profile or caller headers. File evaluates ownership,
unscoped/non-E2E context, ready lifecycle, clean-or-skipped scan result, and
the closed photo/image or video/duration-1..60 predicate atomically from its
locked File row. Its empty success response is an attestation, not metadata for
Story to re-evaluate. `GetFileMetadata` remains Gateway-only.

`ClaimStoryMedia` and `ReleaseStoryMedia` are separate protected `service:story`
mutations. Claim rechecks that same predicate under the blob/File locks and
acquires only `(file_id, STORY, story_id, NULL, NULL)`. The same Story cannot be
rebound to a different File or a retired reference. Immutable operation receipts
bind the entire request; changed operation input returns `AlreadyExists`.
Operation and Story locks serialize concurrent retries before receipt lookup.
Replaying an old successful claim returns its original receipt without creating
a new reference after release. Claim/release receipt hashes use the full protobuf
message name plus NUL and deterministic request bytes, as other File lifecycle
operations do.

Release requires a current `StoryReferenceReleaseAuthority` proving the complete
global owner backfill and File-ledger activation for the installation/restore
cohort. **No production provider exists yet.** Without that authority the RPC
fails closed with `FailedPrecondition` before retiring any reference or creating
a receipt; Story's durable release stays pending and its live reference continues
retaining the blob. A boolean configuration value or an empty ledger is not this
proof. This is an unmet activation criterion owned by the global File-reference
cutover consumer, not a claim of lifecycle completion.

After that authority is installed, release retires the exact key even if a
delayed claim has not arrived. It queues
blob GC only when an existing live exact reference was released and zero live
references remain across all aliases. An absent File is idempotent success;
historical media without an exact claim does not authorize broad deletion.
Another Story/profile/message reference continues to retain the shared blob.
Physical GC remains File-owned and independently retried.

| gRPC code | Validation outcome |
| --- | --- |
| `Unauthenticated` | Credential/request-ID/Phase-0 binding, raw-header, replay, JWKS or verifier failure |
| `PermissionDenied` | Verified caller is not `service:story`, or uploader differs |
| `InvalidArgument` | Invalid UUID, missing/unknown/text expected type, unknown request field |
| `NotFound` | File row is absent |
| `FailedPrecondition` | Chat/E2E/Story context, lifecycle, scan, category or video-duration predicate fails |
| `Unavailable` | Listener or persistence is unavailable |

The protected listener uses `FILE_PRINCIPAL_GRPC_LISTEN`,
`FILE_PRINCIPAL_TLS_CERT_FILE`, `FILE_PRINCIPAL_TLS_KEY_FILE`,
`FILE_PRINCIPAL_CLIENT_CA_FILE` to verify Story's client certificate, and
`FILE_PRINCIPAL_REPLAY_REDIS_ADDR` (optional password via
`FILE_PRINCIPAL_REPLAY_REDIS_PASSWORD`). `S2S_JWKS_URLS_JSON` must contain the
Story and Messaging HTTPS endpoints; `S2S_JWKS_CA_FILE` adds their private trust roots, and
`S2S_JWKS_CLIENT_CERT_FILE`/`S2S_JWKS_CLIENT_KEY_FILE` present File's client
certificate to those endpoints. Cache
settings follow Phase-0 defaults of 30s/2m/5s. Partial configuration fails startup.
The ordinary listener returns `Unavailable` for protected validation and
reference mutations. The protected listener admits only its closed method and
issuer allowlists. See Story's deployment runbook; global reference backfill,
activation and live cleanup verification remain separate release work.

## Масштабирование

Legacy uploader deletion and expiry must always use persistence-backed blob
authority, independently of protected listener/reference feature flags. Lock the
target file `FOR NO KEY UPDATE`, then its blob `FOR UPDATE`; in a fresh statement
reject any live reference across all aliases, or another usable legacy alias
whose ledger may be incomplete. Preserve ordinary zero-reference singleton
deletion. Commit terminal file status, music tombstone and irreversible durable
GC intent before touching object storage. Missing storage, partial deletion or
response loss leaves the intent pending for the ordinary reference-GC worker;
RPC success requires observed completion. Retries preserve the original terminal
status and operation. Shared/dedup/backfill activation still requires sealed
owner evidence and writers serialized by the same blob authority. See
[BE-264 execution plan](../testing/be264-legacy-delete-authority-exec-plan.md)
for outstanding runtime and concurrency gates.

The first logical `expired` transition also commits the exact `file.expired`
envelope in `file_expiry_event_outbox`. The existing expiry worker drains this
queue even when storage deletion fails or no ready files remain after restart.
Publication is independent of byte-GC completion: consumers observe logical
expiry. A bounded row lock covers PubAck; failure/response loss leaves the same
event ID, time and payload pending. JetStream uses that ID as `Nats-Msg-Id`.
NATS-disabled/Noop mode does not acknowledge queued events. Apply migration
`000008` before the binary; DOWN refuses to erase any retained event evidence.

Upload/download — через presigned URLs (R2 обслуживает напрямую). Конвертация — отдельный пул воркеров, масштабируется по очереди задач.

### Subscription retention projection (A7 accepted target; not implemented)

File consumes only the revisioned authoritative
`subscription.entitlement_changed` snapshot for personal retention. New uploads
do not define retention ownership globally: every exact durable reference stores
immutable `retention_account_id`, verified from the account that acquires that
reference, plus actor/uploader profile where relevant. Forward/reuse creates a
new reference for the forwarding account, derived subresources inherit their
parent reference account, and binary dedup never transfers retention ownership.
Legacy rows must be resolved from their domain owner through protected paginated
APIs before this projection is activated; zero/ambiguous ownership blocks the
row instead of guessing from uploader convenience fields. User's protected
profile→account resolver must explicitly allow the authenticated File workload;
File cannot impersonate the current `messaging|chat` callers. The existing
legacy consumer that ACKs into an unwired in-memory cache is removed/replaced at
cutover. Inbox insert, account
entitlement projection and affected reference deadline changes commit in one
`file_db` transaction.

For non-E2E references, free upload starts at `created_at+P90D`, `ACTIVE` and
`GRACE_PERIOD` clear expiry, and `INACTIVE` assigns
`downgrade_effective_at+P90D` to each still-live reference. E2E stays
`created_at+P90D` for every tier. Upload and expiry workers lock against the same
source revision so a concurrent downgrade/renewal cannot commit the wrong
policy. Renewal clears an entitlement-created deadline only for a logically live
reference; an expired reference or blob already handed to zero-reference GC is
never resurrected. Removing one expired reference does not delete a shared blob
while any other live reference remains. Replay, out-of-order and RED cases are
specified in
[subscription-lifecycle-convergence-exec-plan.md](../testing/subscription-lifecycle-convergence-exec-plan.md).

## P3 reference authority, lifecycle fence and GC (accepted target)

File is sole reference and binary-GC authority. Add service-owned blob,
reference, access-capability, lifecycle-fence, immutable operation and deletion-
manifest storage. The unique live key is `(file_id, owner_type, owner_id,
subresource_id?, scope_space_id?)`; owner types are closed and Space-derived
references require `scope_space_id`. `files.chat_id` is upload context only.

Every URL/metadata/bulk/variant read selects an exact reference or opaque,
subject-bound capability with sorted surfaces and expiry no later than one hour.
In one transaction File checks capability subject/surface/database-time expiry,
then locks/requires a `LIVE` Space fence, exact unreleased reference and file
state before any uploader/owner shortcut. Fence and reference locks use that
order everywhere. A capability is rechecked after freeze. Bulk locks UUIDs in
raw-byte order and fails as a whole. Before cutover legacy `file_id` resolves
only when the same single live key remains after locking; zero, multiple or
mixed frozen/live references fail closed. After cutover selector is required.

`AcquireFileReferences` completes before owner data becomes visible; release is
bound to the same tuple and never decrements a caller-owned counter. For each
schedule generation File first durably installs preliminary `FROZEN` against
the Chat binding, then accepts exactly `SPACE`, `CHAT`, `MESSAGING` producer
declarations (including zero counts), contiguous chunks of at most 1000 sorted
keys, exact counts and aggregate hashes. Early, missing, duplicate or changed
chunks cannot seal. Final fence receipt binds the complete root manifest.
Restore reuses it and releases nothing; purge releases only saved manifests.

Zero-reference blobs enter durable `GC_PENDING`; File receipt means exact
releases were accepted and each newly unreferenced blob is pending or complete.
Physical original/converted/thumbnail R2 deletion is independently retried;
NotFound succeeds and ambiguous deletion is reconciled until all keys are
absent. A surviving reference in Story/profile/another live Space keeps shared
bytes and remains readable through its own selector.

Activation requires sealed deterministic backfills from every owner, dual-write
catch-up, a `LIVE` fence for every Space-scoped reference and zero remaining
file_id-only callers. Full lifecycle/operation receipts retain 30 days after
completion; permanent compact PURGED fence state survives identifier reuse.

### Protective historical inventory importer

The dedicated Story inventory protocol is consumed only by File's private worker.
Trusted controller composition must first commit `CreateInventoryOperation` with
the full identity. Migration `000016_reference_inventory` then blocks destructive
work before any remote snapshot call. The worker pulls signed/mTLS Story pages
outside File transactions and atomically records immutable receipts, cursor,
counted issues and private blob holds under the cohort/File/blob locks. Exact
replay is idempotent; changed bytes or identity block the operation. The compiled
coverage commitment includes service, component, closed owner kinds and writer
version; unknown catalog/pending-write key shapes and all other owners stay
MISSING. No caller can select a smaller coverage set or activate a cohort.

Holds protect bytes without creating a readable reference/capability, admitting
historical media or reviving terminal Files. Unknown/missing/terminal/processing,
wrong-context or conflicting rows remain issues. Negative journal state cannot
drop final protection during migration. GC claim and pre-DELETE, alias mutation,
uploader/error/catalog/derivative cleanup use the same barrier. Non-GC storage
deletes durably record STARTED before I/O; ambiguous requests retain STARTED or
UNSETTLED and prohibit a new proof window. A local transaction lock does not
pretend to drain an already submitted object-store request.

Optional `FILE_STORY_REFERENCE_INVENTORY_` client settings are complete-or-absent:
`GRPC_ADDR`, `TLS_CA_FILE`, `TLS_SERVER_NAME`, `TLS_CERT_FILE`, `TLS_KEY_FILE`.
They reuse File's actual signing principal and process only durable DB operations;
configuration does not invent installation/restore IDs or authority.

The global release provider remains nil. All-owner inventory/seals, writer and
reader cutover, Space fences, immutable storage-key generations and external
non-rollback restore/drain control remain prerequisites. Incomplete/unacknowledged
receipts/holds cannot expire merely after P30; completion-based compaction is not
implemented here. See the [execution plan](../testing/be096-reference-inventory-exec-plan.md).
