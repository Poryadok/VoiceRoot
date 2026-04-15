# File Service

## Обзор

Загрузка, хранение, конвертация и раздача файлов. Все медиа проходят через этот сервис.

**Язык**: Go
**БД**: PostgreSQL `file_db`
**Хранилище**: Cloudflare R2 (S3-совместимое)

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

```protobuf
service FileService {
  // Upload
  rpc RequestUpload(UploadRequest) returns (UploadResponse); // presigned URL
  rpc ConfirmUpload(ConfirmUploadRequest) returns (FileMetadata);

  // Download
  rpc GetFileURL(GetFileURLRequest) returns (FileURLResponse); // presigned download URL
  rpc GetFileMetadata(GetFileMetadataRequest) returns (FileMetadata);
  rpc GetBulkMetadata(GetBulkMetadataRequest) returns (BulkFileMetadata);

  // Management
  rpc DeleteFile(DeleteFileRequest) returns (Empty);
  rpc ListFiles(ListFilesRequest) returns (FileList); // by chat/channel

  // Internal
  rpc CheckQuota(CheckQuotaRequest) returns (QuotaResponse);
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
├── status (uploading | processing | ready | infected | expired)
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
├── scan_result (clean | infected | pending | skipped)
├── created_at
└── updated_at

file_references (дедупликация)
├── file_id (FK)
├── message_id (FK)
├── chat_id
└── created_at
```

## Pipeline обработки

```
Client ──presigned URL──► R2 (upload)
         │
         ▼
    ConfirmUpload
         │
         ▼
  ┌──────────────┐
  │ Scan (ClamAV) │──infected──► mark as infected, notify
  └──────┬───────┘
         │ clean
         ▼
  ┌──────────────┐
  │ Convert      │  image→WebP, GIF→MP4, video→720p
  └──────┬───────┘
         │
         ▼
  ┌──────────────┐
  │ Thumbnail    │  generate preview
  └──────┬───────┘
         │
         ▼
    status = ready
    publish file.processed event
```

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`file.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие              | Данные                                    |
|----------------------|-------------------------------------------|
| `file.uploaded`      | file_id, uploader_id, type, size          |
| `file.processed`     | file_id, status, converted_url, thumb_url |
| `file.scan_infected` | file_id, uploader_id                      |
| `file.expired`       | file_id, chat_id                          |
| `file.downloaded`    | file_id, downloader_id                    |

## Зависимости

- **Cloudflare R2** — объектное хранилище
- **ClamAV** — антивирусное сканирование (sidecar или отдельный pod)
- **Subscription Service** — проверка лимитов (размер файла, retention)
- **Messaging Service** — (через NATS) обновление превью в сообщении после конвертации

## Масштабирование

Upload/download — через presigned URLs (R2 обслуживает напрямую). Конвертация — отдельный пул воркеров, масштабируется по очереди задач.


