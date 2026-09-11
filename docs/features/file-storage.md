# File Storage — хранение файлов

## Провайдер: Cloudflare R2

S3-совместимое объектное хранилище.

| Параметр           | Значение                   |
|--------------------|----------------------------|
| Стоимость хранения | $0.015/GB/мес              |
| Исходящий трафик   | $0 (ключевое преимущество) |
| API                | S3-совместимый             |

Хранилище абстрагировано за интерфейсом — провайдер можно сменить без изменения бизнес-логики:

```
FileStorage {
    Upload(file) → url
    Delete(url)
    GeneratePresignedUrl(url, ttl) → tempUrl
}
```

## Лимиты

| Параметр                        | Бесплатно | Подписка |
|---------------------------------|-----------|----------|
| Макс. размер загружаемого файла | 50 MB     | 200 MB   |
| Retention                       | 90 дней   | Навсегда |

Retention реализуется крон-задачей: находит объекты старше порога, удаляет из R2 и помечает в БД как expired. Пользователь видит что файл "протух" — мотивирует к подписке.

## Автоматическая обработка при загрузке

Принимаем большой файл — храним меньший. Экономит место, ускоряет загрузку.

| Тип                         | Обработка                                      | Лимит хранения |
|-----------------------------|------------------------------------------------|----------------|
| Изображения (JPEG/PNG/WebP) | Перекодируем в WebP, quality 80–85%            | ≤5 MB          |
| GIF                         | Конвертируем в MP4/WebM (без звука)            | ≤5 MB          |
| Sticker (pack asset)        | Static WebP or animated WebP/MP4; ≤512×512 px  | ≤512 KB        |
| Видео                       | ffmpeg, 720p, умеренный битрейт                | ≤15 MB         |
| Video note (круглое видео)  | ffmpeg, квадрат/crop, короткий клип (≤60 сек)  | ≤5 MB          |
| Music (audio attach)        | metadata extract; хранится как audio             | ≤10 MB         |
| Article (rich payload)      | HTML/instant-view snapshot + optional thumb      | ≤2 MB payload  |
| Location (static map tile)  | client-generated or server map preview image   | ≤500 KB        |
| Документы, архивы, прочее   | Хранятся как есть (без дополнительного сжатия) | ≤10 MB         |

Оригинал не хранится. Пользователь скачивает обработанную версию.

## Дедупликация

SHA-256 от содержимого файла считается при загрузке.
Если такой хэш уже есть в R2 — новый объект не создаётся, только запись в БД.

Мемы, скриншоты, популярные хайлаты дублируются тысячами пользователей — ожидаемая экономия 30–50% объёма.

## Запись войс-сессий

Запись сохраняется **локально на устройство** того, кто инициировал запись.
Сервер не хранит ничего. Дешевле и privacy-friendly.

## Federated ноды — deferred

Федерация отложена (см. [federation.md](federation.md)). Целевая модель: файлы спейсов на federated нодах хранятся **на нодах**, не в R2 главного сервера. Нода сама выбирает своё объектное хранилище.

## Превышение лимита и истёкшие файлы

- **Файл > лимита**: клиент проверяет размер до загрузки и показывает ошибку; сервер повторно проверяет как защита
- **Истёкший файл (retention)**: в чате вместо файла отображается плейсхолдер с иконкой "кучка костей" — как намёк что файл "умер"; при hover/tap — подпись "Файл удалён. Подписка сохраняет файлы навсегда"

## Безопасность

- **Антивирус**: ClamAV, проверка при загрузке; на старте — только `.exe` / `.zip` / `.bat`, остальные по mime-type; заражённые файлы блокируются
- **Превью документов**: изображения → thumbnail; PDF → превью первой страницы; остальное (DOCX, ZIP и т.п.) → иконка + имя файла + размер; генерация превью — async worker

## Presigned URL — TTL и обновление

- **TTL**: 1 час с момента генерации
- **Клиентская логика**: при получении `403` или `410` на presigned URL клиент вызывает `GET /api/files/{id}/url` — сервер генерирует новую ссылку; клиент повторяет запрос
- **Зачем**: пользователь держит чат открытым часами — ссылки протухнут; lazy refresh решает это прозрачно без перезагрузки чата
- Дополнительный эффект: если история чата утечёт из кэша — старые presigned URL всё равно не работают без обновления

## Не делаем

- **P2P передача файлов в DM** — отложено, возможно не понадобится
- **Хранение оригиналов** — только обработанные версии

## Reference authority and Space deletion (P3 target)

File Service — единственный authority для durable references и binary GC;
`files.chat_id` остаётся legacy upload context и не доказывает liveness. Live
reference key равен `(file_id, owner_type, owner_id, subresource_id?,
scope_space_id?)`; Space-derived Message/Chat/media references обязаны иметь
`scope_space_id`, а Story/profile reference вне Space его не получает.

Каждый URL, metadata, bulk и thumbnail/original/converted refresh выбирает либо
один exact live reference, либо subject-bound File capability (TTL не более
одного часа). File в одной транзакции проверяет subject/surface/expiry, затем
Space fence, exact unreleased reference и только после этого file state и любые
owner/uploader conveniences. Freeze после выдачи capability всё равно запрещает
доступ. Один denied bulk item запрещает весь bulk result. До activation legacy
`file_id` допустим лишь при ровно одном live reference; zero/multiple и mixed
frozen/live references дают fail-closed. После activation selector обязателен.

Domain owner acquires the durable reference before an attachment becomes
visible and releases only the same owner tuple. For Space deletion File first
installs a generation-bound preliminary `FROZEN` fence, then seals fixed
`SPACE`, `CHAT`, `MESSAGING` producer declarations and their exact sorted chunks,
counts and hashes, including zero-count declarations. Restore changes the saved
fence to `LIVE` without releases. Purge replays the sealed producer manifests;
it never discovers a replacement set. A blob enters `GC_PENDING` only at zero
live references across all owner types. Logical deletion completes at durable
access denial/GC handoff; physical R2 removal retries until every object key is
confirmed absent.

