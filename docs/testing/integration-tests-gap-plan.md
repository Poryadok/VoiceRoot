# План закрытия пробелов интеграционных тестов

Дата: **2026-08-15**. Опирается на [integration-tests-inventory.md](integration-tests-inventory.md).  
**Реализация тестов не входит в этот документ** — только порядок работ.

Источники приоритетов: [TESTING.md](../TESTING.md) (критичные пути, smoke/full), [PLAN.md](../PLAN.md), [docs/todo/backend.md](../todo/backend.md), [docs/todo/product-roadmap.md](../todo/product-roadmap.md), [docs/todo/client.md](../todo/client.md).

---

## Цели

1. Исправить рассинхрон CI-манифеста с реальными именами тестов.
2. Закрыть `[missing]` / слабые `[partial]` на Tier-0 (auth, DM, WS resume, delivery).
3. Добавить сквозные кейсы для documented gaps (privacy friend-request, roles enforce, MM decline, Space Pro, analytics DoD).
4. Не изобретать поведение вне `docs/features/` — если код/спека расходятся, сначала спека/фикс продукта (TDD per TESTING.md).

---

## Progress (2026-08-15 sprint)

| Item | Status | Notes |
|------|--------|-------|
| P0.1 | **done** | `TestComposeFriendsInvitation_live`, `TestComposeSearchInChat_live` |
| P0.2 | **done** | resume/delivery/in-app already in smoke; kept |
| P0.3 | **done** | `TestComposeE2EDM_live` + Flutter `encryption_dm_e2e_live_test` in smoke |
| P0.4 | **done** | `TestComposeModerationShadowBan_live` already asserts peer cannot see content |
| P1.4 | **done** | Gateway + Flutter + Social IT for `allow_friend_requests` deny |
| P1.12 | **done** | `matchmaking_rating_e2e_live_test.dart` (rate + skip) |
| P1.13 | **done** | `TestComposeMatchmakingBan_live` + GET ban status transcoding |
| P1.16 | **done** | ingest already asserts CH row <60s |
| P1.17 | **done** | Redis/memory analytics audit store + live Redis assert |
| P1.18 | **done** | map `thumbnailR2Key`/`convertedR2Key`; live asserts key not HTTP previewUrl |
| P1.19 | **done** | ClamAV live uses `.exe` + `application/x-msdownload` |
| P1.6 | **product done** | `allow_forward` in privacy.md + User Get/UpdatePrivacy + Flutter toggle; S2S readable; ForwardMessage enforce → WT-MSG; compose FW-04 → WT-INTEGRATION |
| P1.1 | **done** | auth `depends_on: nats` + convert negatives + `TestComposeConvertGuestNATS_live` |
| P1.2 | **done** | server `guest_reminder_last_shown_at` + get/mark; Flutter cadence; Flutter live → WT-INTEGRATION optional |
| P1.3 | **done** | ListSessions/RevokeSession + `TestComposeAuthSessions_live` |
| P1.5, P1.7–11 | **blocked** | product deps (see inventory / todo/backend) |
| P1.14 | **product done** | compose `SPACE_GRPC_ADDR` + S2S/NATS Space Pro sync; member-cap without Seed; compose live join-51st → WT-INTEGRATION |
| P1.15 | **product done** | `subscription.grace_reminder` D1/D3/D7 + Notification consumer stub; compose assert → WT-INTEGRATION |
| P2.* / P3.* | **deferred** | roadmap / soft-launch+ |

---

## Слои и команды запуска

| Слой | Где писать | Как гонять |
|------|------------|------------|
| A. CI manifest / smoke | `.github/ci/e2e-features.yml` | `make compose-e2e-smoke` |
| B. Gateway compose live | `src/backend/gateway/compose_*_live_test.go` | `VOICE_RUN_LIVE_COMPOSE=true go test` / `make compose-e2e-live` |
| C. Flutter live | `src/frontend/test/*_e2e_live_test.dart` | `VOICE_RUN_LIVE_INTEGRATION=true flutter test …` |
| D. Go service IT | `*_integration_test.go` + testcontainers | `go test ./...` (full) / nightly |
| E. Auth Java IT | `*IntegrationTest.java` | `mvn -B test` |
| F. Device `integration_test/` | `src/frontend/integration_test/` | отдельный CI job; сейчас aspirational |

Effort: **S** ≤0.5d · **M** 1–2d · **L** 3–5d · **XL** >1w (инфра/продукт + тесты).

---

## P0 — блокер корректности CI / soft-launch (1–3 дня)

| # | Работа | Слой | Inventory IDs | Effort | Зависимости |
|---|--------|------|---------------|--------|-------------|
| P0.1 | Починить имена в `e2e-features.yml`: `TestComposeFriendsInvitation_live`, явный Search smoke (`SearchInChat` или `UsersSearch`) | A | (manifest note) | S | нет |
| P0.2 | Добавить в **smoke** Tier-0 delivery: `TestComposeWSResume_live`, `TestComposeDeliveryReceipts_live`, `TestComposeInAppNotifications_live` (+ Flutter counterparts уже в full) — см. todo cross-cutting | A | TC-DM-03/04, NT-01 | S | P0.1 |
| P0.3 | Зафиксировать в smoke хотя бы один encryption DM path (`TestComposeE2EDM_live` / `encryption_dm_e2e_live_test`) — сейчас только key backup | A | ENC-01 | S | compose-migrate-e2e |
| P0.4 | Проверить содержимое `TestComposeModerationShadowBan_live` vs todo «smoke mentions shadow ban but…»; дописать assert если пусто | B | MD-03 | S | moderation seed |

**DoD P0:** `scripts/ci/e2e-manifest.sh` резолвит все smoke_gateway имена; master push smoke ловит resume+delivery.

---

## P1 — критичные продуктовые gaps (1–2 недели)

Группировка по фичам; внутри — сначала Gateway (дешевле), затем Flutter mirror если нужен клиентский контракт.

### 1.1 Auth / Guest / Contacts

| # | Кейс | Слой | IDs | Effort | Notes |
|---|------|------|-----|--------|-------|
| P1.1 | Convert-guest negative + NATS event на compose | B+E | AU-05/14 | M | **done** — auth nats depends_on + Maven negatives + `TestComposeConvertGuestNATS_live` |
| P1.2 | Guest reminder cadence (2nd login ≤1/day) | C | AU-07 | M | **done** — server timestamp + Flutter; optional Flutter live → WT-INTEGRATION |
| P1.3 | Active sessions list/revoke | B | AU-12 | M | **done** — List/Revoke + `TestComposeAuthSessions_live` |
| P1.4 | Friend-request denied by `allow_friend_requests` | B+C | FR-03, PV-04 | M | Social hook already exists |

### 1.2 Messaging / Forward / Roles enforce

| # | Кейс | Слой | IDs | Effort | Notes |
|---|------|------|-----|--------|-------|
| P1.5 | Forward to channel + commentary | B+C | FW-02 | M | Messaging IT hole |
| P1.6 | Forward privacy forbid | B | FW-04 | S | **product done** (`allow_forward`); enforce + compose IT remaining |
| P1.7 | E2E forward policy (no ciphertext leak) | B | FW-06 | M | **после** фикс Messaging |
| P1.8 | SendMessage respects `TEXT_CHAT_SEND_MESSAGES` deny | B+C | RL-02 | M | **после** Messaging CheckPermission |
| P1.9 | @here / @everyone permission gate | B | TC-MSG-03 | M | |
| P1.10 | VOICE_JOIN deny live | B+C | RL-03 | M | Role store IT already |

### 1.3 Matchmaking semantics

| # | Кейс | Слой | IDs | Effort | Notes |
|---|------|------|-----|--------|-------|
| P1.11 | Cross-party decline: acceptors continue | B | MM-07 | M | **после** fix handleMatchDecline |
| P1.12 | Flutter `matchmaking_rating_e2e_live_test` + skip rating | C | MM-05 | M | compose rating exists |
| P1.13 | MM ban live compose | B | MM-06 | S | store IT exists |

### 1.4 Subscription / Space Pro

| # | Кейс | Слой | IDs | Effort | Notes |
|---|------|------|-----|--------|-------|
| P1.14 | Space Pro webhook → invite/member-cap | B | SP-11, SUB-03 | L | **product done** (sync+compose addr); compose join live → WT-INTEGRATION |
| P1.15 | Grace notifications stubs (event emitted) | B | SUB-04 | M | **product done** (event+notif stub); compose live → WT-INTEGRATION |

### 1.5 Analytics DoD

| # | Кейс | Слой | IDs | Effort | Notes |
|---|------|------|-----|--------|-------|
| P1.16 | Assert CH row <60s after `message.sent` | B | AN-03 | M | harden `TestComposeAnalyticsIngest_live` |
| P1.17 | Export audit log assertion | B | AN-04 | S | Gateway audit store |

### 1.6 File reliability

| # | Кейс | Слой | IDs | Effort | Notes |
|---|------|------|-----|--------|-------|
| P1.18 | Fix thumb mapper **или** test expectation | C | FL-02 | S | конфликт todo |
| P1.19 | ClamAV test uses scannable MIME/ext | C | FL-03 | S | |

**Порядок P1:** P1.4 → P1.8/10 (после product fix) → P1.11 → P1.14 → P1.16/17 → file S-fixes. Параллельно P1.5–6 если Messaging свободен.

---

## P2 — покрытие фич partial / roadmap (2–4 недели)

Только где продукт уже (или одновременно) реализуется; иначе тест будет skip forever.

| # | Фича / путь | Слой | IDs | Effort | Product dep |
|---|-------------|------|-----|--------|-------------|
| P2.1 | Space MM queue isolation | B+C | MM-08 | L | roadmap П.1 |
| P2.2 | LFP story → party | B+C | ST-04 | L | П.3 |
| P2.3 | Game request + admin approve | B + Admin | GC-02/03 | L | П.4 |
| P2.4 | Twitch/YouTube/DNS verification live | B+C | VR-02/03 | XL | П.6 |
| P2.5 | Quiet hours / granular notif | B+C | NT-04 | L | П.10 |
| P2.6 | Commander / raise hand | B+C | VC-07 | L | П.11 |
| P2.7 | Stories moderation hide + Nobody floor | B | ST-05/06 | M | Story fixes |
| P2.8 | Search privacy audience | D+B | SR-05 | M | Search fix |
| P2.9 | E2E reindex skips ciphertext | D | ENC-12 | M | Search fix |
| P2.10 | Presence DND/invisible | B+C | PR-02 | M | |
| P2.11 | DM archive/hide | B+C | TC-DM-08 | M | |
| P2.12 | Onboarding coach-marks MM/space | C | ON-03 | M | П.20 |
| P2.13 | Premium cosmetics cross-smoke | B+C | SUB-06 | M | entitlements |
| P2.14 | Bot autocomplete / portal catalog | B+C | BT-07 | L | П.15 |
| P2.15 | Copy-forward without attribution | C | FW-03 | S | UI |
| P2.16 | Multi-forward | C | FW-05 | M | UI backlog? |

---

## P3 — платформа / device / a11y / ops (после soft-launch)

| # | Работа | Слой | IDs | Effort |
|---|--------|------|-----|--------|
| P3.1 | Реальный `integration_test` driver suite (push, deeplink, VoIP) | F | PL-04, DL-04, NT-05 | XL |
| P3.2 | Windows tray/PTT/auto-update smoke | C/CI | PL-03, UPD-02/03 | L |
| P3.3 | A11y automation beyond widgets | C | A11Y-03 | M |
| P3.4 | Staging observability request_id scripted smoke | ops | OBS-02 | M |
| P3.5 | Sticker/GIF/voice-message/recording lives | B+C | TC-MSG-09/10, VC-10 | L | нужен product scope |
| P3.6 | Password reset / Resend | E+B | AU-13 | L | Resend client |
| P3.7 | Federation suite | — | FED-01 | — | deferred |

---

## Рекомендуемый порядок спринтов

```text
Sprint 0 (2–3d):  P0.1–P0.4  — CI truth + smoke Tier-0
Sprint 1 (1w):    P1.4, P1.18–19, P1.16–17, P1.12–13  — privacy + analytics DoD + MM rating/ban + file test honesty
Sprint 2 (1w):    product fixes then P1.5–11, P1.8, P1.14  — messaging/roles/MM decline/Space Pro
Sprint 3+ :       P2 as roadmap features land; P3 device suite parallel track
```

---

## Правила написания новых live-тестов (чтобы не плодить шум)

1. **Имена:** `TestCompose<Feature><Path>_live` / `<feature>_<path>_e2e_live_test.dart` — как существующие.
2. **Opt-in флаги:** `VOICE_RUN_LIVE_COMPOSE` / `VOICE_RUN_LIVE_INTEGRATION`; без флага — skip, не fail.
3. **Добавлять в** `e2e-features.yml` `full_*`; в `smoke_*` — только если путь Tier-0 или явный gate soft-launch.
4. **Не дублировать** service IT и compose без нужды: compose = Gateway+stack; service IT = store/grpc границы.
5. **Документировать** Windows/TLS грабли — не раздувать TESTING.md; ссылка на существующий раздел.
6. **При `[partial]` конфликте** (file thumb, clamav, Space Pro seed): сначала починить прод/маппер, потом тест — иначе зелёный ложный сигнал.

---

## Метрики прогресса

Обновлять сводку в inventory после каждого спринта:

| Метрика | Сейчас (2026-08-15 post-sprint) | Цель soft-launch |
|---------|---------------------:|------------------:|
| Total cases | 208 | ≥ same scope |
| `[exists]` | ~116 | ≥ 130 |
| `[missing]` | ~52 | ≤ 35 (остальное — roadmap/deferred) |
| `[missing]` Tier-0 related | ~0 (smoke paths) | 0 |
| Smoke manifest name mismatches | 0 | 0 |
| Analytics DoD assert | AN-03/04 covered | AN-03/04 green |

---

## Вне скоупа этого плана

- Реализация продуктовых фич (только тесты после/вместе с фиксом).
- Pact / contract brokers (явно out в TESTING.md).
- Federation.
- Ручные a11y / staging observability чеклисты (`[n/a]`).
- Повышение % coverage как KPI (TESTING: минимум не задаём).
