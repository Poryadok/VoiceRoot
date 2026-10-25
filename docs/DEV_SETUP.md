# Подготовка машины (локальная разработка)

Чеклист первичной настройки рабочей станции для монорепозитория **Voice**. Процесс Git/PR — [CONTRIBUTING.md](CONTRIBUTING.md); тесты и CI-паритет — [TESTING.md](TESTING.md).

---

## 1. Клонирование

```powershell
git clone git@github.com:<org>/Voice.git
cd Voice
```

Ветка по умолчанию — **`master`** ([CONTRIBUTING.md](CONTRIBUTING.md)).

---

## 2. Toolchain

Ориентир — таблица в [README.md](../README.md). Минимум для `make build-all`:

| Инструмент | Версия (ориентир) | Зачем |
|------------|-------------------|--------|
| **Go** | 1.26.x | backend, gateway, `go test` |
| **Docker** + Compose v2 | см. README | compose, testcontainers, образы CI |
| **Java** | 21 | Auth (`mvn test`) |
| **Maven** | 3.x | Auth |
| **buf** | 1.50.x | protobuf lint/generate (опционально на хосте; в CI — Docker) |
| **Flutter** + Dart | см. README | клиент, `make flutter-ci` |
| **Node.js** | **24** | admin, developer-portal ([TESTING.md](TESTING.md)) |
| **Git** | 2.x | worktree, hooks |
| **gh** | актуальный | PR из CLI ([CONTRIBUTING.md](CONTRIBUTING.md)) |

Проверка минимального набора для backend CI:

```powershell
make check-toolchain
```

**Windows:** в PowerShell в цепочках команд используй **`;`**, не `&&` ([TESTING.md](TESTING.md)).

---

## 3. Git: политика истории и хуки (обязательно)

После clone установи **git-хуки репозитория** и **Cursor `beforeShellExecution`** (блок force-push, rebase, `--no-verify`, amend, `reset --hard`):

```powershell
.\scripts\git\install-hooks.ps1
```

Linux/macOS:

```bash
chmod +x scripts/git/install-hooks.sh scripts/git/hooks/*
./scripts/git/install-hooks.sh
```

Только git-хуки без Cursor:

```powershell
.\scripts\git\install-hooks.ps1 -SkipCursor
```

Самопроверка Cursor hook:

```powershell
node scripts\git\selftest-history-rewrite.js
```

Подробности: [`scripts/git/README.md`](../scripts/git/README.md), правило для агентов — [`.cursor/rules/git-safety.mdc`](../.cursor/rules/git-safety.mdc), политика — раздел «История Git и хуки» в [CONTRIBUTING.md](CONTRIBUTING.md).

**Если hook упал** — исправь файлы и сделай **новый коммит**; не используй `--no-verify`, amend и rebase для обхода.

---

## 4. Окружение и Docker Compose

Compose использует PostgreSQL 18 и Redis 8 согласно целевому стеку
[`MICROSERVICES.md`](MICROSERVICES.md). Это версии локального Compose;
манифесты существующих DEV/STAGING/production и их фактические версии проверяются
отдельно перед защищённым rollout.

Перед обновлением существующего Compose остановите приложения и сохраните
проверенный logical backup всех БД PostgreSQL 16. Замена image не обновляет
формат данных: PostgreSQL 18 откажется запускать старый `voice_pgdata`.
`PGDATA=/var/lib/postgresql/data` задан явно, чтобы PostgreSQL 18 не создал
другой пустой cluster в новом стандартном каталоге. Старый volume не удаляйте
и не запускайте `down -v` для обновления. Восстановление выполняйте в отдельный
Compose project с новыми volumes на PostgreSQL 18; проверьте migration state,
количество/целостность данных и приложение до переключения. Для возврата
сохраните старый PostgreSQL 16 volume и совместимый checkout; PostgreSQL 16
не открывает PostgreSQL 18 data directory. Redis AOF также требует backup и
отдельной проверки восстановления перед обновлением существующих данных.

Официальный образ PostgreSQL 18 изменил стандартный `PGDATA`; здесь оставлен
явный совместимый mount layout: [описание образа](https://hub.docker.com/_/postgres).

```powershell
copy .env.example .env
# при необходимости отредактировать порты — см. README
docker compose up -d
```

Инфра (Postgres, Redis, NATS) поднимается без профиля `app`. Полный стек (auth, gateway, web, …):

```powershell
make compose-app-up
```

Рекомендуемые порты в `.env`:

```text
GATEWAY_PORT=18080
VOICE_API_PUBLIC_URL=http://127.0.0.1:18080
WEB_PORT=9080
```

| Сервис | URL |
|--------|-----|
| Web UI | http://127.0.0.1:9080 |
| Gateway (REST + WS) | http://127.0.0.1:18080 |

Остановка: `make compose-down`.

Миграции после первого подъёма — [`src/backend/migrations/README.md`](../src/backend/migrations/README.md) (`make compose-migrate-all`).

---

## 5. Flutter (клиент)

```powershell
cd src\frontend
flutter pub get
flutter analyze
flutter test
```

Из корня: **`make flutter-ci`**. Live-тесты против compose — [`src/frontend/integration_test/README.md`](../src/frontend/integration_test/README.md).

---

## 6. Node (admin / developer-portal)

Только **Node 24** ([TESTING.md](TESTING.md)):

```powershell
cd src\developer-portal
npm ci
npm test
npm run build
```

Admin: `src/admin/` — аналогично (`npm ci`, `npm test`).

---

## 7. Cursor / агенты (опционально, рекомендуется)

| Что | Где |
|-----|-----|
| Git safety (always-on) | `.cursor/rules/git-safety.mdc` — в репозитории |
| Shell hook (history rewrite) | `.\scripts\git\install-hooks.ps1` → `~/.cursor/hooks/` |
| TDD workflow | `.cursor/skills/tdd-code-workflow/` → канон `.agent/workflows/tdd-code-workflow/` |
| Penpot / макеты | [design/penpot-setup.md](design/penpot-setup.md) |
| CodeGraph | локально `codegraph init` в корне (см. `.cursor/rules/codegraph.mdc`) |

---

## 8. Smoke после настройки

```powershell
make check-toolchain
make compose-app-up
make build-all
make flutter-ci
```

Полный sign-off перед merge/release — скилл `voice-project-full-verification`.

---

## Связанные документы

- [README.md](../README.md) — структура репо, toolchain, compose
- [CONTRIBUTING.md](CONTRIBUTING.md) — ветки, PR, история Git
- [TESTING.md](TESTING.md) — что гонять перед PR, Windows-грабли
- [REPOSITORIES.md](REPOSITORIES.md) — protos, codegen
- [scripts/git/README.md](../scripts/git/README.md) — git/Cursor hooks
