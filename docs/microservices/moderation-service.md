# Moderation Service

## Обзор

Система модерации: жалобы, автоматическая модерация, санкции, апелляции.

**Язык**: Go
**БД**: PostgreSQL `moderation_db`

BE-137 global intake follows the accepted [opaque report contract](../testing/report-intake-contract.md):
Gateway-derived verified reporter, durable receipt-only `202`, asynchronous
owner decisions, validation-gated staff/count/actions and transactional outbox.
The implementation must carry new intake state and historical publication
provenance together; owner absence is not a public rejection or report oracle.

## Ответственность

- Жалобы на пользователей, сообщения, пространства
- Категории: spam, harassment, offensive, fake, cheating, other
- Авто-блокировка: `max(10, ceil(N/100))` distinct qualifying reporter accounts за 24 часа → shadow ban; accepted authority contract below
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
├── reporter_account_id (nullable; verified submission identity, NULL for legacy)
├── target_type (user | message | space | story)
├── target_id (UUID)
├── validated_target_account_id (nullable; User LIVE mapping only)
├── validation_state (PENDING | VALID | MISSING | DELETED)
├── validation_publication_legacy (boolean; historical events must not replay)
├── validation_attempts / validation_next_at
├── validation_claim / validation_claim_until (durable validation lease)
├── validation_failure_code (sanitized dependency/disposition code)
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

`GetAutoModStats.messages_checked` counts configured `CheckMessage` evaluations
since the scan aggregate was introduced, including allowed/blocked results and
each retried RPC. The current request has no message ID or idempotency key, so
this is not a unique-message count. Pattern evaluation happens before the
existing spam-sender validation; an evaluation followed by that validation error
still counts. A missing AutoMod implementation bypasses scanning and does not
count. Historical clean scans cannot be recovered from `auto_mod_log`: the new
aggregate starts at zero rather than importing an incomplete action-log estimate.

The durable `auto_mod_scan_stats` singleton holds only `messages_checked` and
`recorded_since`, without message content or profile/message identifiers.
Concurrent increments are atomic. Counter storage failure emits a generic
warning and preserves the existing moderation result/error; the aggregate can
therefore undercount during failures and is not a billing or audit ledger.
The business decision is computed before synchronous metric persistence. The
metric receives a child timeout capped at 50 ms and one quarter of the remaining
caller deadline, reserving the rest for returning the result; canceled callers
and metric budgets below 1 ms skip persistence. Counter lock/pool/network stalls
therefore cannot exhaust the business query's context. There is no background
worker, queue or per-request goroutine. Lock contention/timeouts and skipped
short-budget samples can undercount; no retry/backlog is fabricated.
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
  2. Если trigger → account mute на 1 час; второй независимый hit того же паттерна за 30 дней → perm_ban

report.created event ──► Check thresholds:
  1. Count reports on target in 24h
  2. Если count ≥ max(10, ceil(authoritative target audience / 100)) → shadow_ban
  3. Добавить в очередь ручной модерации
```

## Публикуемые события (→ NATS)

The accepted BE-046 target is the
[authoritative report-threshold vertical](../testing/report-threshold-exec-plan.md).
Only exact user-profile cases are automatic; no profile-as-account UUID
fallback or environment denominator is authority. Count distinct verified
submission accounts from VALID unresolved reports with compatible target
binding and original server time. User supplies a complete fresh potential
public-profile account denominator; Moderation separately resolves the BE-045
subject/lifecycle, then rechecks under the common account lock. Unknown authority
preserves accepted/manual reports and persistent retry, without a sanction.

One logical active account case, machine-issued shadow sanction, log, evidence
associations and immutable event outbox commit atomically. Review closure writes
an account cutoff against immediate replay; shadow ban does not mutate Auth
account status. This is an accepted target, not an assertion of shipped runtime.

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

### Private committed-message moderation contract (BE-045)

`voice.automod.v1.ModerationEvidenceService` has protected unary methods
`ProvisionSubject`, `IngestCommittedMessageEvidence`, `GetAccountSendRestriction`
and `GetEffectiveRestrictionSnapshot`. Only `service:messaging` may provision,
ingest or read send restrictions; only `service:auth` may read the effective-ban
snapshot. Fresh Phase-0 credentials bind exact RPC/request bytes/request ID,
audience `moderation`, short expiry and replay state; mTLS protects the listener.
Legacy listeners/headers do not grant access. Evidence carries immutable subject,
account/source epoch/sequence/source time, event/message/chat/profile identities,
eligibility and keyed aliases/link count; no raw text or URLs.

Provisioning establishes a verified Auth subject and zero-sequence baseline.
Ingestion cannot infer a baseline from its first event. One account advisory
transaction serializes ordered inbox/cursor, samples, consumed-through sequence,
strikes, effective mute/post-mute state, machine `perm_ban`, log, immutable domain
event and Auth restriction intent. Duplicate delivery never extends a mute.
Machine issuer is `service:moderation/automod`, with no fabricated human profile.
Manual sanctions, revocation, appeals and expiry use this same restriction journal.
`CheckMessage` becomes read-only existing-state admission, never pattern counting.
See [the complete contract, retention rules and T1-T12](../testing/async-automod-exec-plan.md).

## P3 Space deletion classification (accepted target)

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

Startup/restore uses the [P0-P5 barriers](../testing/async-automod-startup-restore.md):
local issuer/private listeners start before public readiness; a complete sealed
Auth terminal snapshot is imported as a monotonic union and every terminal purge
participant finishes before restored evidence, restriction or domain-event
dispatch. Only private restriction repair runs in the intermediate phase. Normal
consumer/manual/appeal/expiry/publication paths open after this boot's finite
reconciliation cut. An old in-flight reply is distinguished from actual local
history loss by comparing CURRENT local revision; neither acknowledges the old
request or justifies incrementing a restored stale snapshot.

The one Moderation signer loads `MODERATION_PRINCIPAL_SIGNING_KEYS_DIR` and
`MODERATION_PRINCIPAL_ACTIVE_KID`. All report/evidence/Auth clients receive that
same issuer instance. Its certificate-only mTLS JWKS route is
`https://voice-moderation-authority:8443/internal/principals/jwks`, with
`MODERATION_PRINCIPAL_JWKS_LISTEN` defaulting to `:8443`. The shared publisher uses
`MODERATION_PRINCIPAL_TLS_CERT_FILE`, `MODERATION_PRINCIPAL_TLS_KEY_FILE` and
`MODERATION_PRINCIPAL_TLS_CLIENT_CA_FILE`; it starts independently of public
readiness and serves only the current/next public keys. It never requires the
JWT, session floor or reciprocal domain RPC which its keys bootstrap.

