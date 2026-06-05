# Пробелы и открытые вопросы (документация)

Здесь — **вне дорожной карты** [PLAN.md](PLAN.md). Продуктовые и инфраструктурные чеклисты по фазам, остаток Фазы 0 (Auth: smoke контейнера в CI, порядок миграций `auth_db`), UUIDv7 для сообщений — в плане. Крупные темы при желании дублируйте в issue-трекере.

## Сейчас (по мере работы с контрактами)

- [ ] **Политика `protos/` и генерации** — зафиксировать в [REPOSITORIES.md](REPOSITORIES.md) и в скриптах сборки: что коммитим в git (Go/Java), что генерируем при CI/локально, по мере расширения публичных gRPC.

- [ ] **`GRPC_DIAL_TIMEOUT` для S2S dial при старте** — вынести хардкод `30*time.Second` (ранее 5 с) из `user` / `chat` / `messaging` / `file` `main.go` в общий helper (`pkg/grpcconn` или аналог) и env `GRPC_DIAL_TIMEOUT` (дефолт в compose, например `15s`); один источник для bootstrap-dial к upstream gRPC.

## Позже / по событию

- [ ] **Buf Schema Registry (BSR)** — сейчас только локальный модуль `protos/`; при введении удалённого registry обновить CI и [REPOSITORIES.md](REPOSITORIES.md).

- [ ] **Скрипт «поднять стенд + применить все миграции»** — опциональная обёртка в `Makefile` / `scripts/` поверх [src/backend/migrations/README.md](../src/backend/migrations/README.md); ускоряет onboarding, не блокирует фазы.

- [ ] **Аудит консистентности доков** — пройти [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md) после первого значимого изменения контрактов или при закрытии крупных фаз.
