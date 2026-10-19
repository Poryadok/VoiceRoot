# Moderation Service

## Обзор

Система модерации: жалобы, автоматическая модерация, санкции, апелляции.

**Язык**: Go
**БД**: PostgreSQL `moderation_db`

## Ответственность

- Жалобы на пользователей, сообщения, пространства
- Категории: spam, harassment, offensive, fake, cheating, other
- Авто-блокировка: 1% или 10+ жалоб за 24 часа → shadow ban
- Pattern spam detection → auto-mute
- Shadow ban (пользователь не знает о бане)
- Очередь ручной модерации
- Санкции: warning, temp ban, permanent ban
- Апелляции (1 на санкцию, 7 дней на подачу)
- ММ-специфичные баны (через Matchmaking Service)
- Федеративная модерация (нода отвечает за свой контент, нарушения → дефедерация)

## Account-ban enforcement

Auth account status is suspended while at least one temp_ban or perm_ban
sanction for the account is not revoked and has not expired. Revoking, expiring,
or approving an appeal for one sanction reactivates the account only when no
other effective account ban remains. Applying or removing account bans and
reconciling Auth status are serialized per account by Moderation. Matchmaking
bans remain an independent measure and do not change Auth account status.

A sanction removal and its durable moderation_account_status_sync intent commit
before Moderation calls Auth to reconcile status. Reconciliation rechecks effective
bans while holding the same per-account lock, then removes the pending row only
after Auth accepts the derived status. Auth failures and ambiguous reconciliation
commits leave retry work for the existing one-minute expiry sweeper. A failed or
ambiguous sanction-mutation commit never triggers account activation; the persisted
ban remains effective if that transaction did not commit. Applying a ban still
suspends Auth before committing the local ban, so failure can only leave an extra
suspension; an uncertain apply result is reconciled from committed sanctions.

## API (gRPC)

Жалобы — **один** RPC `CreateReport` с полем `target_type` (`user` | `message` | `space` | `story`), не отдельные методы по типу объекта. Клиент: `POST /api/v1/moderation/reports` (202 Accepted). Категория `mm_toxic` на HTTP нормализуется в `cheating`.

```protobuf
service ModerationService {
  // Жалобы
  rpc CreateReport(CreateReportRequest) returns (CreateReportResponse);
  rpc GetReport(GetReportRequest) returns (Report);
  rpc ListReports(ListReportsRequest) returns (ReportList); // admin
  rpc ResolveReport(ResolveReportRequest) returns (Report); // admin

  // Санкции
  rpc ApplySanction(ApplySanctionRequest) returns (Sanction);
  rpc RevokeSanction(RevokeSanctionRequest) returns (Empty);
  rpc GetAccountSanctions(GetAccountSanctionsRequest) returns (SanctionList);
  rpc GetActiveSanction(GetActiveSanctionRequest) returns (Sanction);

  // Апелляции
  rpc SubmitAppeal(SubmitAppealRequest) returns (Appeal);
  rpc ReviewAppeal(ReviewAppealRequest) returns (Appeal); // admin
  rpc GetAppeal(GetAppealRequest) returns (Appeal);
  rpc ListAppeals(ListAppealsRequest) returns (AppealList); // admin

  // Auto-moderation
  rpc CheckMessage(CheckMessageRequest) returns (CheckResult); // internal, sync
  rpc GetAutoModStats(GetAutoModStatsRequest) returns (AutoModStats);

  // Shadow ban
  rpc IsShadowBanned(IsShadowBannedRequest) returns (IsShadowBannedResponse); // internal
}
```

## Модель данных

```
reports
├── id (UUID)
├── reporter_profile_id
├── target_type (user | message | space | story)
├── target_id (UUID)
├── category (spam | harassment | offensive | fake | cheating | other)
├── description (text, nullable)
├── evidence (jsonb — screenshots, message_ids)
├── status (pending | reviewing | resolved | dismissed)
├── assigned_to (admin profile_id, nullable)
├── resolved_at (nullable)
├── resolution (jsonb, nullable)
├── created_at
└── updated_at

sanctions
├── id (UUID)
├── target_account_id
├── type (warning | temp_ban | perm_ban | shadow_ban | mm_ban)
├── reason (text)
├── report_id (FK, nullable)
├── issued_by (profile_id)
├── expires_at (nullable)
├── revoked_at (nullable)
├── revoked_by (profile_id, nullable)
├── created_at
└── updated_at

moderation_account_status_sync
├── account_id (UUID, primary key)
└── requested_at (timestamp; pending Auth reconciliation)

appeals
├── id (UUID)
├── sanction_id (FK)
├── appellant_account_id
├── reason (text)
├── status (pending | approved | denied)
├── reviewed_by (profile_id, nullable)
├── reviewed_at (nullable)
├── review_notes (text, nullable)
├── created_at
└── UNIQUE(sanction_id) -- 1 appeal per sanction

auto_mod_log
├── id (UUID)
├── target_profile_id
├── trigger (spam_pattern | report_threshold)
├── action (mute | shadow_ban)
├── details (jsonb)
├── created_at
└── reverted_at (nullable)
```

## Авто-модерация

### Scan statistics

`GetAutoModStats.messages_checked` counts durable persisted scan samples
since the scan aggregate was introduced. Each configured `CheckMessage` evaluation
is eligible for one sample, including allowed/blocked results and
each retried RPC. The current request has no message ID or idempotency key, so
this is not a unique-message count. Pattern evaluation happens before the
existing spam-sender validation; an evaluation followed by that validation error
is still eligible. A missing AutoMod implementation bypasses scanning and does not
produce a sample. Historical clean scans cannot be recovered from `auto_mod_log`: the new
aggregate starts at zero rather than importing an incomplete action-log estimate.

The durable `auto_mod_scan_stats` singleton holds only `messages_checked` and
`recorded_since`, without message content or profile/message identifiers.
Concurrent increments are atomic. Counter storage failure emits a generic
warning and preserves the existing moderation result/error; the aggregate can
therefore omit evaluations under contention, cancellation, deadline pressure or
storage/network failure, including ordinary supported low-resource operation.
There is no promised coverage percentage, unbiased sampling, exact evaluation
total or eventual catch-up. It is not a billing or audit ledger. A successful
business RPC does not acknowledge metric persistence. An error/timeout can have
an unknown commit outcome; a late committed sample remains a real evaluation.
The business decision is computed before synchronous metric persistence. The
metric receives a child timeout capped at 50 ms and one quarter of the remaining
caller deadline, reserving the rest for returning the result; canceled callers
and metric budgets below 1 ms skip persistence. Counter lock/pool/network stalls
use a separate context after the business decision. The budget bounds intentional
waiting, not wall-clock scheduling under arbitrary starvation; a shared database
or pool outage can independently affect business queries. There is no background
worker, queue or per-request goroutine. Lock contention/timeouts and skipped
short-budget samples can undercount; no retry/backlog is fabricated.
`moderation_automod_scan_samples_total{outcome}` exposes exactly one terminal
outcome per eligible evaluation: `acknowledged`, `unconfirmed`,
`skipped_canceled`, or `skipped_budget`. Every persistence error is conservatively
unconfirmed, never classified as lost or rolled back. These finite-label,
process-lifetime diagnostics are not a durable ledger; crashes and restarts
limit reconciliation. No content, identity or request labels are recorded.
Stats reads fail on missing/broken storage rather than presenting a fabricated
zero. `blocked` retains its existing action-log aggregation and time coverage;
it is not a ratio-compatible counterpart of this since-introduction scan count.

Apply the additive migration before deploying the binary. Old binaries can run
with the extra table but cannot populate it. Rolling back the counter migration
deletes only this aggregate and resets its baseline if reapplied; preserve it
for operational continuity and roll back the binary before any schema removal.

```
message.sent event ──► CheckMessage():
  1. Spam pattern detection (repeated messages, link flooding)
  2. Если trigger → auto-mute (5 мин первый раз, экспоненциально)

report.created event ──► Check thresholds:
  1. Count reports on target in 24h
  2. Если ≥10 или ≥1% от активных пользователей → shadow_ban
  3. Добавить в очередь ручной модерации
```

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`moderation.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                       | Данные                               |
|-------------------------------|--------------------------------------|
| `moderation.report_created`   | report_id, target_type, target_id    |
| `moderation.report_resolved`  | report_id, resolution                |
| `moderation.sanction_applied` | sanction_id, target_account_id, type |
| `moderation.sanction_revoked` | sanction_id, target_account_id       |
| `moderation.appeal_submitted` | appeal_id, sanction_id               |
| `moderation.appeal_reviewed`  | appeal_id, status                    |
| `moderation.auto_action`      | target_id, trigger, action           |

## Зависимости

- **User Service** — получение данных о пользователе
- **Messaging Service** — получение содержимого сообщения при жалобе
- **Matchmaking Service** — ММ-баны
- **Notification Service** — (через NATS) уведомление о санкции
- **Federation Service** — (через NATS) дефедерация при нарушениях

## P3 Space deletion classification (accepted target)

This section applies to platform reports. The separate local jurisdiction is
specified below and never enters the platform sanctions/appeal pipeline.

Moderation is not a purge-completion participant. At Space purge, `pending` and
`reviewing` Space-target reports atomically become `dismissed` with
`resolution_code=TARGET_DELETED`, then descriptions, screenshots and message
evidence are deleted. Resolved or already dismissed Space-target reports and
their Space evidence are deleted.

An issued account/profile sanction remains effective until its own expiry or
revocation; Space deletion never lifts it. Before report removal
`sanction.report_id` becomes null and only a minimal non-Space reason code
remains. A pending appeal continues against the sanction snapshot and may
approve, deny or revoke without retaining report or Space content. The accepted
product/Moderation decision declares that snapshot sufficient for every active
appeal; no hidden `RETAIN_UNTIL_POLICY` exception or staff evidence archive is
created in P3.

## Space-local report jurisdiction (accepted target)

BE-066 adds the separate local scenario in [reports.md](../features/reports.md#локальные-жалобы-space).
This is a target contract; the existing `Report` DTO, `reports` table and global
handlers do not acquire local Role authorization. Existing rows remain PLATFORM.
Use independent `space_reports` persistence, API messages and repository methods;
no backfill/copy, implicit routing by target owner or cross-jurisdiction IDs.

| Target RPC | HTTP route under `/api/v1/spaces/{space_id}/moderation/reports` |
|---|---|
| `CreateSpaceReport` | `POST /` → 202, opaque `report_id` only |
| `ListSpaceReports` | `GET /` → scoped summary page |
| `GetSpaceReport` | `GET /{report_id}` → permitted local detail |
| `AssignSpaceReport` | `PATCH /{report_id}/assignee` → versioned result |
| `ResolveSpaceReport` | `POST /{report_id}/resolve` → versioned result |

Requests bind canonical `space_id`, exact `ChatRef` and `message_id`; optional
evidence references must equal that target. No body can override the path scope.
Assign and Resolve carry `expected_version`; initial version is positive and
increments on each actual change. Terminal same-decision retry returns its
existing result without changing version; conflicting terminal actions fail.
Report IDs are UUIDv4. Target message IDs remain Messaging UUIDv7; their version
does not prove existence or jurisdiction. `List` supports status filtering and
bounded cursor pagination after authorized jurisdiction/content selection.

Local DTOs never contain reporter identity, global report/sanction identifiers,
arbitrary evidence JSON, account mappings or copied message bodies. Summary
excludes description/evidence/decision notes; detail includes only fields allowed
by current target ACL. Description and required decision note each have a
500-codepoint bound. Local reports never feed platform threshold/event workers.
Global staff status supplies no implicit local read override; staff use a
separately authorized local actor or the platform jurisdiction.

### Protected authority and owner inputs

Every new client RPC accepts only verified Gateway `delegated_user`,
`iss=gateway`, `aud=moderation`, account `sub`, active `profile_id`,
`session_epoch`, exact full RPC, deterministic protobuf request hash and
`request_id`, plus ordinary temporal/JWKS/replay/TLS guards. No raw identity or
internal metadata fallback; the legacy listener cannot dispatch new methods.

Moderation obtains message existence, canonical Chat/Space ownership, immutable
message timestamp, current actor membership and exact content visibility from
the owning services through protected request-bound RPCs. It does not query
foreign databases or infer jurisdiction from client `space_id`. Fetching a
message without a viewer is not proof of viewer access. The owner protocol must
also support per-row list filtering, current read authorization and positive
deletion/move observations without exposing a public existence oracle.

Role is checked via the separately allowed signed `service:moderation`
`CheckPermission` for `MODERATION_MANAGE_REPORTS` and the exact canonical Chat
node. Check actor on List/Get/Assign/Resolve and assignee on Assign; no role-name
shortcut. Each operation revalidates current owner/Role inputs before its local
decision. A short-lived credential or earlier list result is not an atomic
remote-revocation fence. This v1 follows request-time owner authorization, with
ordinary in-flight operations serialized by local version checks; it does not
promise distributed ACID or revoke-before-commit linearizability.

Unavailable verifier/Role/owner inputs fail closed before disclosure/effects.
Foreign or inaccessible report IDs have the same non-disclosing response.
Deletion/move cleanup requires an authoritative positive observation and exact
saved target binding; absence, timeout and malformed replies never trigger it.
Target protocol implementation/qualification is required before activation.

### Durable data and lifecycle

`space_reports` owns immutable report/Space/Chat/message binding, closed private
reporter profile reference, category, bounded description, typed target evidence,
status, nullable assignee, version, bounded terminal note/code and timestamps.
Any idempotency/work/cursor records are Moderation-owned and jurisdiction-bound.
Cross-service identifiers are references without foreign-database FKs.

Creation and state/version changes commit atomically. A failed authorization,
conflict or rollback emits no report decision, sanction or event. Assignment
does not reserve permission. Local effects are limited to the review workflow;
enforcement stays with each action owner. All lifecycle writers preserve this
separation. Freeze denies ordinary local access; positive delete/move scrubs
content, and irreversible P3/account erasure cleans owned personal content and
derived work. Local evidence is not copied into global audit/export/automod.
Schema rollback must refuse while retained local data or replay obligations
would be lost; no delete-to-downgrade path is allowed.

Implementation and discriminating tests are tracked in
[space-local-reports-exec-plan.md](../testing/space-local-reports-exec-plan.md).

