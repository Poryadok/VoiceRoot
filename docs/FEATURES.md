# Features — каталог фич

Краткий индекс всех фич проекта. Детальное описание — в файлах по ссылкам.

**Scope** (канон — [PLAN.md](PLAN.md)): **`current`** — в спеке, реализовать сейчас; **`partial`** — спека есть, код неполный; **`deferred`** — только федерация и её подфичи.

## Коммуникация

| Фича                | Файл                                                | Scope | Описание                                                                        |
|---------------------|-----------------------------------------------------|-------|---------------------------------------------------------------------------------|
| Текстовый чат       | [text-chat.md](features/text-chat.md)               | partial | Composer Telegram-parity, archive, folders, Quick Access (≤15), pins (≤5), list preview, стикеры/GIF, attach/send types |
| Пересылка сообщений | [forward-messages.md](features/forward-messages.md) | current | Forward как в Telegram: мульти-выбор, атрибуция, копирование без атрибуции      |
| Войс-чат            | [voice-chat.md](features/voice-chat.md)             | partial | Звонки в DM, голосовые комнаты в спейсах, временные комнаты у групп, видео, PTT |
| Шара экрана         | [screen-share.md](features/screen-share.md)         | current | Демонстрация экрана/окна в войс-сессиях, системный звук, аннотации              |
| Сторис              | [stories.md](features/stories.md)                   | partial | Короткий контент (фото/видео/текст) на 24 часа, архив, гейминг-фишки            |

## Пользователь и аккаунт

| Фича                   | Файл                                                  | Scope | Описание                                                              |
|------------------------|-------------------------------------------------------|-------|-----------------------------------------------------------------------|
| Регистрация и контакты | [auth-and-contacts.md](features/auth-and-contacts.md) | partial | Вход по телефону/email, гостевой доступ, контакты из телефонной книги |
| Верификация            | [verification.md](features/verification.md)           | partial | Значки для физлиц (Twitch/YouTube) и организаций (DNS), анти-спуфинг  |
| Профиль пользователя   | [user-profile.md](features/user-profile.md)           | partial | Публичный профиль, аватар, биография, статусы, верификация            |
| Множественные профили  | [multi-profile.md](features/multi-profile.md)         | partial | Несколько независимых "цифровых личностей" на одном аккаунте          |
| Друзья и контакты      | [friends.md](features/friends.md)                     | current | Два уровня отношений (контакт / друг), запросы дружбы, блокировка     |
| Статусы присутствия    | [presence.md](features/presence.md)                   | partial | Онлайн / не активен / DND / невидимый, кастомный статус               |
| Приватность            | [privacy.md](features/privacy.md)                     | partial | Гранулярные настройки видимости полей профиля и активности            |

## Сообщества

| Фича           | Файл                            | Scope | Описание                                                            |
|----------------|---------------------------------|-------|---------------------------------------------------------------------|
| Спейсы         | [spaces.md](features/spaces.md) | partial | Контейнеры: дерево (текст + голос), группы и каналы, инвайты, каталог, категории |
| Ролевая модель | [roles.md](features/roles.md)   | current | Иерархия прав (владелец → гость), кастомные роли, гранулярные права |

## Геймерские фичи

| Фича        | Файл                                        | Scope | Описание                                                         |
|-------------|---------------------------------------------|-------|------------------------------------------------------------------|
| Матчмейкинг | [matchmaking.md](features/matchmaking.md)   | partial | Поиск тиммейтов по игре/рангу/ролям — глобальный и внутри спейса |
| Каталог игр | [game-catalog.md](features/game-catalog.md) | current | Справочник игр для ММ: страница игры, режимы, роли, UX каталога  |

## Безопасность и доверие

| Фича                | Файл                                    | Scope | Описание                                                         |
|---------------------|-----------------------------------------|-------|------------------------------------------------------------------|
| Репорты / модерация | [reports.md](features/reports.md)       | partial | Жалобы на пользователей, сообщения, спейсы; глобальная модерация |
| Шифрование          | [encryption.md](features/encryption.md) | current | TLS везде; E2E опционально для DM с ограничениями поиска         |
| Онбординг           | [onboarding.md](features/onboarding.md) | current | Минимальный туториал для нового пользователя (4–5 шагов)         |

## Платформа и доступность

| Фича                 | Файл                                          | Scope | Описание                                                        |
|----------------------|-----------------------------------------------|-------|-----------------------------------------------------------------|
| Accessibility (a11y) | [accessibility.md](features/accessibility.md) | current | Клавиатурная навигация, screen reader, контраст, reduced motion |
| Локализация (i18n)   | [i18n.md](features/i18n.md)                   | current | EN+RU baseline, ARB-файлы, pg_trgm для поиска, RTL отложен      |

## Экосистема

| Фича | Файл                        | Scope | Описание                                                                            |
|------|-----------------------------|-------|-------------------------------------------------------------------------------------|
| Боты | [bots.md](features/bots.md) | partial | Slash-команды, app manifest, scopes, per-channel контроль, webhook/polling доставка |

## Инфраструктура и монетизация

| Фича                 | Файл                                          | Scope | Описание                                                                              |
|----------------------|-----------------------------------------------|-------|---------------------------------------------------------------------------------------|
| Навигация            | [navigation.md](features/navigation.md)       | partial | Rail IA: folders, Quick Access (≤15), ProfileStack, ☰ settings; archive via profile RC; mobile tab bar + drawer + active strip; virtual «Запросы» folder |
| Поиск                | [search.md](features/search.md)               | current | Поиск внутри чата и глобальный поиск по контактам, спейсам, сообщениям                |
| Deep links / Sharing | [deep-links.md](features/deep-links.md)       | current | Схема URL для ссылок на спейс, канал, сообщение, профиль                              |
| Платформы            | [platforms.md](features/platforms.md)         | partial | Web → Windows → Mobile; Flutter, ограничения веб-версии                               |
| Обновления клиентов  | [updates.md](features/updates.md)             | partial | Версионирование API, force-update, desktop auto-update, mobile in-app + Shorebird OTA |
| Уведомления          | [notifications.md](features/notifications.md) | partial | Типы (`new_message`, `message_request`, `mention`, …), presence routing (online → in-app only), `send_silent`, quiet hours (push suppressed / in-app delivered), dual-path read sync |
| Подписка             | [subscription.md](features/subscription.md)   | partial | Премиум профиль ($5/мес) и буст спейса, косметика и расширенные лимиты                |
| Хранение файлов      | [file-storage.md](features/file-storage.md)   | partial | Cloudflare R2, лимиты, сжатие медиа, дедупликация, retention; stickers/GIF via `UploadIntent` (😊 panel, wire spec — [file-service.md](microservices/file-service.md)) |
| Наблюдаемость        | [observability.md](features/observability.md) | partial | Логи (Loki), метрики (Prometheus), Grafana, алерты                                    |
| Продуктовая аналитика | [analytics.md](features/analytics.md)     | partial | ClickHouse, NATS ingest, staff dashboards, воронки/retention, export с audit log      |
| Федерация            | [federation.md](features/federation.md)       | deferred | Self-hosted ноды — **deferred** (по запросу рынка); спека и scaffold в репозитории |

