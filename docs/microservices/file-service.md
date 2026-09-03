# File Service

## Обзор

Загрузка, хранение, конвертация и раздача файлов. Все медиа проходят через этот сервис.

**Язык**: Go
**БД**: PostgreSQL `file_db`
**Хранилище**: Cloudflare R2 (S3-совместимое)

**Scope:** все медиа проходят через этот сервис. Аватар профиля может использовать R2 presigned в контуре User до полного развёртывания File Service — см. [user-profile.md](../features/user-profile.md), [PLAN.md](../PLAN.md).

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

**`intent` field (spec — not yet in proto):** distinguishes processing branch on `ConfirmUpload`. Extends shipped `RequestUploadRequest` / `ConfirmUploadRequest`:

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
  UploadIntent intent = 8;              // spec — not yet in proto
  optional string source_url = 9;       // GIF provider import only (HTTPS); server-side fetch → R2
}
```

**GIF provider import:** when `intent=gif` and `source_url` set, File Service (or worker) fetches from provider CDN, stores in R2, then runs GIF transcode pipeline — client does **not** presigned-PUT the remote URL directly. User-uploaded GIF uses presigned PUT without `source_url`.

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

Normative product scope: [text-chat.md](../features/text-chat.md) § «Медиа и вложения»; wire payloads — [messaging-service.md](messaging-service.md) § Stickers and GIF; pack catalog — [chat-service.md](chat-service.md) § Sticker packs. **Zero implementation** today. Stickers/GIF are **not** 📎 attach-menu uploads — composer **😊 panel** only.

### Ownership split

| Concern | Owner | File Service role |
|---------|-------|-------------------|
| Sticker pack catalog / install | **Chat** (`sticker_packs`, `profile_installed_packs`) | Stores per-sticker binary; returns `file_id` |
| GIF provider search | **Chat** (`SearchGifs` — Giphy/Tenor adapter) | Import selected asset via `RequestUpload(intent=gif, source_url=…)` or reuse cached `file_id` |
| Send payload / `content_type` | **Messaging** `SendMessage` | `file_id` references processed asset |
| Composer picker UI | Flutter §3.6b | — |

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
      → File RequestUpload(intent=gif, source_url=…) OR reuse cached file_id
      → ConfirmUpload → ffmpeg MP4/WebM, thumb frame
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
| `converted_url`, `thumb_url` | Bubble + list-row preview (presigned or CDN URLs) |
| `width`, `height`, `duration_seconds` | Video note / GIF / music metadata |
| `intent` (or `upload_intent`) | Branch refresh logic (`gif`, `sticker`, `video_note`, …) |

**Messaging consumer (spec — not yet implemented):** on `file.processed`, update attachment JSON on affected messages, recompute `GetChatListMetadata` for impacted `chat_id`s, emit Realtime `message_update` so client refreshes bubble + inbox row without full history reload. Applies to all intents including `video_note`, `music`, `sticker`, `gif`. Checklist — [todo/backend.md](../todo/backend.md).

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`file.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие              | Данные                                    |
|----------------------|-------------------------------------------|
| `file.uploaded`      | file_id, uploader_id, type, size          |
| `file.processed`     | file_id, status, converted_url, thumb_url, **width**, **height**, **duration_seconds**, **intent** |
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

## Масштабирование

Upload/download — через presigned URLs (R2 обслуживает напрямую). Конвертация — отдельный пул воркеров, масштабируется по очереди задач.


