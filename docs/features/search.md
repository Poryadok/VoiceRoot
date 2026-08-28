# Search — поиск

За основу взята модель Telegram.

## Два контекста поиска

| Контекст          | Где                             | Что ищет                          |
|-------------------|---------------------------------|-----------------------------------|
| Поиск внутри чата | Иконка лупы в хедере чата       | Сообщения в текущем чате          |
| Глобальный поиск  | Строка поиска на главном экране | Контакты, чаты, спейсы, сообщения |

---

## Глобальный поиск — результаты

Результаты разбиты на секции:

### Контакты и чаты
- Совпадение по имени или @username
- Показывает: аватар, имя, последнее сообщение (если переписка есть) — семантика preview как в [text-chat.md](text-chat.md) § «Preview последнего сообщения в списке» (ticks, Photo/Video/Voice/File/Sticker/GIF, call labels)

### Спейсы
- Совпадение по названию спейса
- Показывает: иконка, название, число участников

### Сообщения
- Полнотекстовый поиск по содержимому сообщений (только в тех чатах, к которым у пользователя есть доступ)
- Показывает: фрагмент сообщения с подсвеченным совпадением, имя отправителя, дата, название чата
- Клик → переход к сообщению в чате

### Порядок секций
Контакты → Спейсы → Сообщения (от самого релевантного вверх).

---

## Поиск внутри чата

- Поиск по **тексту** сообщений этого чата
- **Ссылки (URL)** в теле сообщения — индексируются как текст (ILIKE / tsvector); отдельная вкладка **Links** в Shared Media — см. ниже
- Результаты с подсвеченным вхождением
- Стрелки "вверх/вниз" переходят между совпадениями
- Клик → чат скроллится к сообщению

**Policy:** link-only messages (no other text) still match URL substring search. Full URL preview indexing — same as message body.

---

## Фильтры shared media

В каждом чате есть раздел **Shared Media** с вкладками — отдельный вид поиска по типу контента. Канон вкладок (согласовано с [text-chat.md](text-chat.md) / [screen-controls.md](../design/screen-controls.md)):

| Вкладка   | `MessageContentType` / kind | Что показывает |
|-----------|----------------------------|----------------|
| **Медиа** | photo, video, video_note, gif | Фото, видео, круглые video notes, GIF |
| **Стикеры** | sticker | Sticker messages |
| **Файлы** | document, music | Документы, архивы, music attachments |
| **Ссылки** | link, article | URL в тексте + article payloads |
| **Голосовые** | voice | Voice messages |

### Content-type mapping (wire → UI → Shared Media)

Канонические идентификаторы — `MessageContentType` в [messaging-service.md](../microservices/messaging-service.md); legacy/kind aliases в `message_attachments.kind`:

| Product | `MessageContentType` | Attachment `kind` | List preview label | Shared Media tab | In-chat search |
|---------|---------------------|-------------------|-------------------|------------------|----------------|
| Photo | `PHOTO` | `image` | Photo | **Медиа** | — (media grid) |
| Video | `VIDEO` | `video` | Video | **Медиа** | — |
| Video note | `VIDEO_NOTE` | `video_note` | Video message | **Медиа** | — |
| GIF | `GIF` | `gif` | GIF | **Медиа** | — |
| Voice | `VOICE` | `voice_message` | Voice | **Голосовые** | — |
| Document | `DOCUMENT` | `document` | File | **Файлы** | filename |
| Music | `MUSIC` | `music` | Music | **Файлы** | title/artist metadata |
| Sticker | `STICKER` | `sticker` | Sticker | **Стикеры** | — |
| Inline URL | `TEXT` + link metadata | `link` | text preview or URL snippet | **Ссылки** | URL substring in body |
| Article | `ARTICLE` | `article` | Article | **Ссылки** | title/description in payload |
| Plain text | `TEXT` | — | text snippet | — | full text index |

**Inline URL vs Article:** inline URL — client OG preview on `TEXT`; Article — attach-menu payload, server metadata worker. См. [text-chat.md](text-chat.md) § «Article vs inline URL».

---

## Технические детали

- Поиск по контактам и спейсам — простой ILIKE / prefix-индекс
- Поиск мгновенный (debounce ~300ms после ввода)
- Результаты сообщений пагинируются (20 штук, подгрузка при скролле)
- **Federated поиск** — **deferred** (см. [federation.md](federation.md)); целевая модель: индекс на каждой ноде; master рассылает запрос на все ноды параллельно, агрегирует результаты; нода недоступна — её результаты пропускаются (graceful degradation)

### Стратегия роста поискового бэкенда

| Этап | Движок                               | Триггер перехода                             |
|------|--------------------------------------|----------------------------------------------|
| Старт | PostgreSQL `tsvector` / `GIN`-индекс | —                                            |
| Рост | Meilisearch                          | p95 latency поиска >500ms или ~10M сообщений |
| Аналитика | Elasticsearch                    | Нужна полная аналитика / сложные агрегации   |

- **Старт:** PostgreSQL `simple` конфигурация (без стемминга, работает для любого языка); `russian` + `english` — при росте базы
- **Meilisearch**: проще в ops чем Elasticsearch, хорошая поддержка русского, достаточно до этапа «Аналитика»
- При переходе — двойная запись: новые сообщения индексируются в обоих движках до полной миграции

---

## Что не ищем

- Содержимое зашифрованных / E2E чатов (если такие появятся в будущем)
- **Полнотекстовый поиск по содержимому файлов** (текст внутри PDF, DOCX и т.д.) — **не входит в scope**; отдельное решение по индексации и правам доступа. По вложениям доступен поиск по **имени файла** и отображение в **Shared Media** ([text-chat.md](text-chat.md) / вкладки медиа), без индексации тела файла.

**Владельцы:** [Search Service](../microservices/search-service.md); при федерации (deferred) — маршрутизация в [Federation Service](../microservices/federation-service.md); сообщения остаются в **Messaging**, контакты/спейсы — **User** / **Chat** / **Space** (см. [DATA_MODEL.md](../DATA_MODEL.md)).

