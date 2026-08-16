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

## Progress (2026-08-16 Wave 1b WT-INTEGRATION)

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
| P1.1 | **done** | auth `depends_on: nats` + convert negatives + `TestComposeConvertGuestNATS_live` |
| P1.2 | **done** | server reminder + `TestComposeGuestReminder_live` + Flutter `guest_reminder_e2e_live_test` |
| P1.3 | **done** | ListSessions/RevokeSession + `TestComposeAuthSessions_live` |
| P1.5 | **done** | `TestComposeForwardChannelCommentary_live` + Flutter channel+commentary |
| P1.6 | **done** | `TestComposeForwardPrivacyDeny_live` + Flutter FW-04 deny |
| P1.7 | **done** | Messaging ITs (E2E forward policy); compose not required |
| P1.8 | **done** | `TestComposeRolesSendDeny_live` + Flutter SendMessage 403 |
| P1.9 | **done** | `TestComposeMentionsEveryoneDeny_live` |
| P1.10 | **done** | `TestComposeVoiceJoinDeny_live` + Flutter VOICE_JOIN deny |
| P1.11 | **done** | `TestComposeMatchmakingCrossPartyDecline_live` |
| P1.14 | **done** | `TestComposeSpaceProMemberCap_live` (seed filler + webhook join) |
| P1.15 | **done** | `TestComposeSubscriptionGraceReminder_live` (D1 sweeper assert) |
| P2.9 | **done** | ENC-12: `TestReindexChat_SkipsE2EBodies_postgres` (reindex skips `IsE2E`) |
| P2.7 | **done** | product: `HideStoryFromFeed` + CreateStory `show_stories` floor (ST-05/06 ITs); #21 |
| P2.10 | **done** | PR-02: `user.presence_changed` + Realtime friend fan-out; `TestComposePresenceDNDInvisible_live` + Flutter |
| P2.11 | **product done** | MuteChat/ArchiveChat + Gateway + Flutter list actions (#16); compose/Flutter live IT still open |
| P2.12 | **done** | Flutter `onboarding_coach_e2e_live_test` (spaces/MM steps + invite mid-tour); ON-03 (#20) |
| P2.8 | **done** | SR-05: Search filters profile hits by `allow_friend_requests` + blocks; unit + `TestComposeSearchPrivacyAudience_live` |
| P2.3 | **product done** | SubmitGameRequest → pending_moderation + Admin approve/reject (#19); compose/Flutter live IT still open |
| P2.4 | **product done** | Twitch/YouTube OAuth + cron refresh + org DNS (#25); compose/Flutter live IT still open |
| P2.5 | **product done** | GetQuietHours + Flutter server sync; voice_member_joined quiet-hours assert; compose live IT still open |
| P2.6 | **product done** | GrantFloor/RevokeFloor + SetBroadcasting + LiveKit client ducking + Flutter organizer panel; VC-07 compose/Flutter live IT still open |
| P2.15 | **product done** | `without_attribution` ForwardMessage + Flutter Copy as new; Messaging/Gateway ITs; compose live still open |
| P2.13 | **done** | live entitlements for User cosmetics + GIF avatar; `TestComposePremiumCosmetics_live` + Flutter SUB-06 (#26) |
| P2.14 | **done** | BT-07: `TestComposeBotsAutocomplete_live` + Flutter `bots_autocomplete_e2e_live_test`; portal command catalog (#30) |
| P2.2 | **product done** | RespondToLfpStory → NATS → DecideLfpRequest party queue + Notification Accept/Decline + Flutter LFP card (#35); compose/Flutter live IT still open |
| P2.1 | **product done** | StartSpaceQueue + `mm:space:{id}` isolation + UpdateSpaceMmConfig + Gateway/Flutter (#32); compose/Flutter live IT still open |
| P3.4 | **done** | `scripts/staging/smoke-request-id.sh` (DM → Loki chain); OBS-02 |
| P3.2 | **partial** | `windows_desktop_smoke_test` + existing version/auto-update unit paths (UPD-03); tray/global PTT/overlay product still П.17–18 (PL-03) |
| P3.1 | **partial** | `integration_test/device_driver_smoke_test.dart` + CI `flutter-device-driver` (host tester: deeplink + FCM/VoIP register contracts); NT-05 / on-device App Links / CallKit still open |
| P3.6 | **done** | ResendMailSender when `AUTH_RESEND_API_KEY` set; `/password/reset` E2E + AU-13 |
| P2.* / P3.* (rest) | **deferred** | roadmap / soft-launch+ |

---

## Слои и команды запуска

| Слой | Где писать | Как гонять |
|------|------------|------------|
| A. CI manifest / smoke | `.github/ci/e2e-features.yml` | `make compose-e2e-smoke` |
| B. Gateway compose live | `src/backend/gateway/compose_*_live_test.go` | `VOICE_RUN_LIVE_COMPOSE=true go test` / `make compose-e2e-live` |
| C. Flutter live | `src/frontend/test/*_e2e_live_test.dart` | `VOICE_RUN_LIVE_INTEGRATION=true flutter test …` |
| D. Go service IT | `*_integration_test.go` + testcontainers | `go test ./...` (full) / nightly |
| E. Auth Java IT | `*IntegrationTest.java` | `mvn -B test` |
| F. Device `integration_test/` | `src/frontend/integration_test/` | CI job `flutter-device-driver` (host tester scaffold); physical device / emulator suite still open |

Effort: **S** ≤0.5d · **M** 1–2d · **L** 3–5d · **XL** >1w (инфра/продукт + тесты).

---

## P0 — блокер корректности CI / soft-launch (1–3 дня)

| # | Работа | Слой | Inventory IDs | Effort | Status | Зависимости |
|---|--------|------|---------------|--------|--------|-------------|
| P0.1 | Починить имена в `e2e-features.yml`: `TestComposeFriendsInvitation_live`, явный Search smoke (`SearchInChat` или `UsersSearch`) | A | (manifest note) | S | **done** | нет |
| P0.2 | Добавить в **smoke** Tier-0 delivery: `TestComposeWSResume_live`, `TestComposeDeliveryReceipts_live`, `TestComposeInAppNotifications_live` (+ Flutter counterparts уже в full) — см. todo cross-cutting | A | TC-DM-03/04, NT-01 | S | **done** | P0.1 |
| P0.3 | Зафиксировать в smoke хотя бы один encryption DM path (`TestComposeE2EDM_live` / `encryption_dm_e2e_live_test`) — сейчас только key backup | A | ENC-01 | S | **done** | compose-migrate-e2e |
| P0.4 | Проверить содержимое `TestComposeModerationShadowBan_live` vs todo «smoke mentions shadow ban but…»; дописать assert если пусто | B | MD-03 | S | **done** | moderation seed |

**DoD P0:** `scripts/ci/e2e-manifest.sh` резолвит все smoke_gateway имена; master push smoke ловит resume+delivery.

---

## P1 — критичные продуктовые gaps (1–2 недели)

Группировка по фичам; внутри — сначала Gateway (дешевле), затем Flutter mirror если нужен клиентский контракт.

### 1.1 Auth / Guest / Contacts

| # | Кейс | Слой | IDs | Effort | Status | Notes |
|---|------|------|-----|--------|--------|-------|
| P1.1 | Convert-guest negative + NATS event на compose | B+E | AU-05/14 | M | **done** | auth nats depends_on + Maven negatives + `TestComposeConvertGuestNATS_live` |
| P1.2 | Guest reminder cadence (2nd login ≤1/day) | C | AU-07 | M | **done** | `TestComposeGuestReminder_live` + `guest_reminder_e2e_live_test` |
| P1.3 | Active sessions list/revoke | B | AU-12 | M | **done** | List/Revoke + `TestComposeAuthSessions_live` |
| P1.4 | Friend-request denied by `allow_friend_requests` | B+C | FR-03, PV-04 | M | **done** | Gateway + Flutter + Social IT |

### 1.2 Messaging / Forward / Roles enforce

| # | Кейс | Слой | IDs | Effort | Status | Notes |
|---|------|------|-----|--------|--------|-------|
| P1.5 | Forward to channel + commentary | B+C | FW-02 | M | **done** | `TestComposeForwardChannelCommentary_live` + Flutter channel+commentary |
| P1.6 | Forward privacy forbid | B | FW-04 | S | **done** | `TestComposeForwardPrivacyDeny_live` + Flutter FW-04 |
| P1.7 | E2E forward policy (no ciphertext leak) | B | FW-06 | M | **done** | Messaging ITs |
| P1.8 | SendMessage respects `TEXT_CHAT_SEND_MESSAGES` deny | B+C | RL-02 | M | **done** | `TestComposeRolesSendDeny_live` + Flutter SendMessage 403 |
| P1.9 | @here / @everyone permission gate | B | TC-MSG-03 | M | **done** | `TestComposeMentionsEveryoneDeny_live` |
| P1.10 | VOICE_JOIN deny live | B+C | RL-03 | M | **done** | `TestComposeVoiceJoinDeny_live` + Flutter |

### 1.3 Matchmaking semantics

| # | Кейс | Слой | IDs | Effort | Status | Notes |
|---|------|------|-----|--------|--------|-------|
| P1.11 | Cross-party decline: acceptors continue | B | MM-07 | M | **done** | `TestComposeMatchmakingCrossPartyDecline_live` |
| P1.12 | Flutter `matchmaking_rating_e2e_live_test` + skip rating | C | MM-05 | M | **done** | compose rating exists |
| P1.13 | MM ban live compose | B | MM-06 | S | **done** | `TestComposeMatchmakingBan_live` |

### 1.4 Subscription / Space Pro

| # | Кейс | Слой | IDs | Effort | Status | Notes |
|---|------|------|-----|--------|--------|-------|
| P1.14 | Space Pro webhook → invite/member-cap | B | SP-11, SUB-03 | L | **done** | `TestComposeSpaceProMemberCap_live` |
| P1.15 | Grace notifications stubs (event emitted) | B | SUB-04 | M | **done** | `TestComposeSubscriptionGraceReminder_live` |

### 1.5 Analytics DoD

| # | Кейс | Слой | IDs | Effort | Status | Notes |
|---|------|------|-----|--------|--------|-------|
| P1.16 | Assert CH row <60s after `message.sent` | B | AN-03 | M | **done** | ingest asserts CH row <60s |
| P1.17 | Export audit log assertion | B | AN-04 | S | **done** | Redis/memory audit store + live Redis assert |

### 1.6 File reliability

| # | Кейс | Слой | IDs | Effort | Status | Notes |
|---|------|------|-----|--------|--------|-------|
| P1.18 | Fix thumb mapper **или** test expectation | C | FL-02 | S | **done** | map `thumbnailR2Key`/`convertedR2Key` |
| P1.19 | ClamAV test uses scannable MIME/ext | C | FL-03 | S | **done** | `.exe` + `application/x-msdownload` |

**Порядок P1:** P1.4 → P1.8/10 (после product fix) → P1.11 → P1.14 → P1.16/17 → file S-fixes. Параллельно P1.5–6 если Messaging свободен.

---

## P2 — покрытие фич partial / roadmap (2–4 недели)

Только где продукт уже (или одновременно) реализуется; иначе тест будет skip forever.

| # | Фича / путь | Слой | IDs | Effort | Product dep |
|---|-------------|------|-----|--------|-------------|
| P2.1 | Space MM queue isolation | B+C | MM-08 | L | **product done** (#32); compose/Flutter live IT open |
| P2.2 | LFP story → party | B+C | ST-04 | L | **product done** (#35); compose/Flutter live IT open || P2.3 | Game request + admin approve | B + Admin | GC-02/03 | L | **product done** (#19); compose/Flutter live IT open |
| P2.4 | Twitch/YouTube/DNS verification live | B+C | VR-02/03 | XL | **product done** (#25); compose/Flutter live IT open |
| P2.5 | Quiet hours / granular notif | B+C | NT-04 | L | **product done** — GetQuietHours + Flutter sync; voice join push assert; live IT open |
| P2.6 | Commander / raise hand | B+C | VC-07 | L | **product done** — GrantFloor + broadcast ducking + organizer panel; compose/Flutter live IT open |
| P2.7 | Stories moderation hide + Nobody floor | B | ST-05/06 | M | **done** — HideStoryFromFeed + CreateStory floor (#21) |
| P2.8 | Search privacy audience | D+B | SR-05 | M | **done** — SearchUsers/SearchGlobal filter by `allow_friend_requests` |
| P2.9 | E2E reindex skips ciphertext | D | ENC-12 | M | **done** — Search IT + indexer skip |
| P2.10 | Presence DND/invisible | B+C | PR-02 | M | **done** — friend NATS fan-out + compose/Flutter |
| P2.11 | DM archive/hide | B+C | TC-DM-08 | M | **product done** (#16); live IT open |
| P2.12 | Onboarding coach-marks MM/space | C | ON-03 | M | **done** — `onboarding_coach_e2e_live_test` (#20) |
| P2.13 | Premium cosmetics cross-smoke | B+C | SUB-06 | M | **done** — live tier + GIF + compose/Flutter (#26) |
| P2.14 | Bot autocomplete / portal catalog | B+C | BT-07 | L | **done** — compose/Flutter autocomplete live + portal catalog (#30) |
| P2.15 | Copy-forward without attribution | C | FW-03 | S | **product done** — Messaging/Gateway/Flutter; live IT open |
| P2.16 | Multi-forward | C | FW-05 | M | UI backlog? |

---

## P3 — платформа / device / a11y / ops (после soft-launch)

| # | Работа | Слой | IDs | Effort |
|---|--------|------|-----|--------|
| P3.1 | Реальный `integration_test` driver suite (push, deeplink, VoIP) | F | PL-04, DL-04, NT-05 | XL | **partial** — host driver + CI; device push/App Links/CallKit still open |
| P3.2 | Windows tray/PTT/auto-update smoke | C/CI | PL-03, UPD-02/03 | L | **partial** — auto-update stub smoke; tray/PTT blocked on П.17 |
| P3.3 | A11y automation beyond widgets | C | A11Y-03 | M |
| P3.4 | Staging observability request_id scripted smoke | ops | OBS-02 | M | **done** — `scripts/staging/smoke-request-id.sh` |
| P3.5 | Sticker/GIF/voice-message/recording lives | B+C | TC-MSG-09/10, VC-10 | L | нужен product scope |
| P3.6 | Password reset / Resend | E+B | AU-13 | L | **done** — ResendMailSender + password reset E2E |
| P3.7 | Federation suite | — | FED-01 | — | deferred |

---

## Рекомендуемый порядок спринтов

```text
Sprint 0 (2–3d):  P0.1–P0.4  — CI truth + smoke Tier-0
Sprint 1 (1w):    P1.4, P1.18–19, P1.16–17, P1.12–13  — privacy + analytics DoD + MM rating/ban + file test honesty
Sprint 2 (1w):    P1.5–11, P1.8, P1.14–15 compose/Flutter lives **done** (WT-INTEGRATION)
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
