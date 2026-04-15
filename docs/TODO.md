# TODO — открытые вопросы и пробелы в документации

Статус проверки: полный обход всех feature-файлов и архитектурных документов. Ниже — замечания, сгруппированные по приоритету.

Последний целевой аудит согласованности и полноты: [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md).

---

## 🔴 Критические пробелы (блокируют реализацию)

### 1. Data model / схема БД нигде не описана
**Закрыто:** конкретные таблицы, индексы, внутренние FK и логические связи для волны v1 — секции «Модель данных» в [microservices/](microservices/). Инвентарь БД: [DATA_STORES.md](DATA_STORES.md). Общие правила: [DATA_MODEL.md](DATA_MODEL.md). Скоуп и трассировка фич: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md). Миграции: [OPERATIONS.md](OPERATIONS.md#миграции-бд-database-per-service).

---

## 🟡 Важные пробелы (нужно решить до кода)

### 2. `docs/PLAN.md` — пробелы в покрытии фич
**Закрыто в [PLAN.md](PLAN.md):** после дробления дорожной карты — Фазы **16** (боты: Portal + runtime) и **17** (сторис); **11** vs **14** — базовые репорты и доверие vs авто-мод и панель + [features/reports.md](features/reports.md); верификация и мульти-профиль — **Фаза 13** + [features/verification.md](features/verification.md).

### 3. Масштабирование WebSocket — edge cases
Базовый сценарий (LB, без sticky, Redis Pub/Sub, падение инстанса, догрузка сообщений через Messaging): [microservices/realtime-service.md](microservices/realtime-service.md). При нагрузочных тестах при необходимости дополнить гонками `resume` vs историей и прочими краевыми случаями.

---

## 🟢 Технические вопросы

*(все закрыты)*

### Инженерная практика (Git, тесты, стенды)

Закрыто документами: [CONTRIBUTING.md](CONTRIBUTING.md), [TESTING.md](TESTING.md), [DEPLOYMENT.md](DEPLOYMENT.md), [REPOSITORIES.md](REPOSITORIES.md).

---

## 📝 Мелкие замечания

*(сняты в проходе 2026-04-16)* — модерация сторис: [features/stories.md](features/stories.md), тип репорта в [features/reports.md](features/reports.md); поиск по содержимому файлов — post-V1: [features/search.md](features/search.md).


