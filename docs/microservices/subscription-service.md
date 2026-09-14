# Subscription Service

## Обзор

Управление подписками, биллингом и лимитами. Два плана: Premium (пользовательский) и Space Pro (для пространств).

**Язык**: Go
**БД**: PostgreSQL `subscription_db`

## Ответственность

- Планы: Premium ($5/мес), Space Pro ($5/мес), скидка -20% за год
- Платёжные провайдеры: Paddle (международный), CloudPayments (СНГ)
- Webhook обработка от провайдеров (создание, продление, отмена)
- Grace period 7 дней после неоплаты
- Downgrade: заморозка excess профилей, снижение лимитов
- Лимиты по подписке (source of truth для других сервисов)
- Федеративные ноды устанавливают свои лимиты

## Лимиты по плану

| Параметр               | Free    | Premium   | Space Pro |
|------------------------|---------|-----------|-----------|
| Размер файла           | 50 MB   | 200 MB    | —         |
| Retention файлов       | 90 дней | Бессрочно | —         |
| Профили                | 2       | 5         | —         |
| Пространства (join)    | 50      | 1000      | —         |
| Voice quality          | 480p    | 720p      | —         |
| Кастомный статус       | Нет     | Да        | —         |
| Premium username       | Нет     | Да        | —         |
| Анонимный просмотр     | Нет     | Да        | —         |
| Участники пространства | —       | —         | 5000      |
| Voice slots            | —       | —         | 128       |
| Узлы дерева спейса (текст + голос) | — | —         | 500       |
| Custom emoji           | —       | —         | Да        |

## API (gRPC)

```protobuf
service SubscriptionService {
  // Подписки
  rpc GetSubscription(GetSubscriptionRequest) returns (Subscription);
  rpc CreateCheckoutSession(CreateCheckoutRequest) returns (CheckoutResponse);
  rpc CancelSubscription(CancelSubscriptionRequest) returns (Subscription);
  rpc ResumeSubscription(ResumeSubscriptionRequest) returns (Subscription);

  // Space Pro
  rpc GetSpaceSubscription(GetSpaceSubRequest) returns (SpaceSubscription);
  rpc CreateSpaceCheckout(CreateSpaceCheckoutRequest) returns (CheckoutResponse);

  // Лимиты (internal — вызывается другими сервисами)
  rpc GetLimits(GetLimitsRequest) returns (Limits);
  rpc CheckLimit(CheckLimitRequest) returns (CheckLimitResponse);

  // Webhooks (от Paddle/CloudPayments)
  rpc HandlePaddleWebhook(WebhookRequest) returns (Empty);
  rpc HandleCloudPaymentsWebhook(WebhookRequest) returns (Empty);

  // Billing history
  rpc GetBillingHistory(GetBillingHistoryRequest) returns (BillingHistoryList);
}
```

## Модель данных

```
subscriptions
├── id (UUID)
├── account_id (FK)
├── plan (premium)
├── billing_period (monthly | yearly)
├── status (active | grace_period | paused | cancelled; observed current values)
├── provider (paddle | cloudpayments)
├── provider_subscription_id (string)
├── current_period_start
├── current_period_end
├── grace_period_end (nullable)
├── cancelled_at (nullable)
├── created_at
└── updated_at

space_subscriptions
├── id (UUID)
├── space_id (FK)
├── purchaser_account_id (FK)
├── plan (space_pro)
├── billing_period (monthly | yearly)
├── status (active | pending_cancel | cancelled; observed current values)
├── provider (paddle | cloudpayments)
├── provider_subscription_id (string)
├── current_period_start
├── current_period_end
├── grace_period_end (nullable)
├── created_at
└── updated_at

billing_events
├── id (UUID)
├── subscription_id (FK, nullable)
├── space_subscription_id (FK, nullable)
├── type (payment_success | payment_failed | subscription_created | subscription_cancelled | ...)
├── amount (decimal)
├── currency (string)
├── provider_event_id (string)
├── details (jsonb)
├── created_at
└── INDEX(subscription_id, created_at)
```

These values describe writes observed in the current runtime; the columns are
not constrained enums and must not be treated as a complete wire contract. The
A7 target below exposes
the canonical entitlement projection as `ACTIVE | GRACE_PERIOD | INACTIVE`;
schema migration must preserve the existing billing detail while making that
projection and its permanent aggregate revision explicit.

## Webhook и идемпотентность

События Paddle/CloudPayments обрабатываются по **`provider_event_id`**: уникальность в `billing_events` (или отдельной таблице дедупликации) по паре **(provider, provider_event_id)**; повтор webhook — no-op с **2xx** провайдеру.

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`subscription.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

**Фактический статус personal lifecycle:** сервисный sweeper раз в минуту обрабатывает
личные подписки в `grace_period`: публикует `subscription.grace_reminder` на
D1/D3/D7, после `grace_period_end` переводит строку в `cancelled` и публикует
`subscription.plan_expired` +
`subscription.downgrade`. Тот же sweeper завершает ручную отмену после
`current_period_end`. Это подтверждает producer/transition, но не downstream-effect:
Notification пока только распознаёт reminder event без push/email dispatch, а User и
Flutter не потребляют downgrade для выбора и заморозки excess-профилей.

`grace_reminders_sent` подавляет обычный последовательный повтор за уже отмеченный
день, но это не гарантия exactly-once: sweeper сначала публикует событие и затем
помечает день без атомарного claim, а публикация не задаёт `Nats-Msg-Id`. Сбой между
этими шагами или параллельные реплики могут опубликовать duplicate reminder.

**Отдельный открытый scope Space Pro:** sync активного entitlement и отмена в конце
оплаченного периода существуют, но payment-failure → 7-day grace/reminders/expiry
для `space_subscriptions` не реализован как lifecycle, эквивалентный personal.

| Событие                          | Данные                       |
|----------------------------------|------------------------------|
| `subscription.plan_started`      | account_id, plan, provider   |
| `subscription.plan_renewed`      | account_id, plan             |
| `subscription.plan_cancelled`    | account_id, plan             |
| `subscription.plan_expired`      | account_id, plan             |
| `subscription.payment_success`   | account_id, amount, currency |
| `subscription.payment_failed`    | account_id, reason           |
| `subscription.space_pro_started` | space_id, purchaser_id       |
| `subscription.space_pro_expired` | space_id                     |
| `subscription.downgrade`         | account_id, plan             |
| `subscription.grace_reminder`    | account_id, plan, day (1/3/7) |

## Зависимости

- **Paddle** — международные платежи (webhook → subscription events)
- **CloudPayments** — СНГ платежи (webhook)
- **User Service** — (через NATS) заморозка excess профилей при downgrade
- **Space Service** — (через NATS) снижение лимитов пространства при expiry
- **File Service** — (через NATS) изменение retention при downgrade

## A7 lifecycle convergence (accepted target; not implemented)

This section replaces post-commit best-effort publication as the target contract.
It does not mark the current publisher, sweeper or downstream consumers complete.
The executable RED plan is
[subscription-lifecycle-convergence-exec-plan.md](../testing/subscription-lifecycle-convergence-exec-plan.md).

### Entitlement aggregate and state machine

Subscription owns a permanent aggregate revision for `(PERSONAL, account_id)`
and `(SPACE, space_id)`, including cancellation followed by repurchase. Each
effective transition locks and increments that revision. Irreversible account
purge replaces raw account/purchaser identifiers with a permanent HMAC tombstone
while preserving replay fences and the last revision. Canonical states are:

| State | Benefits | Transition notes |
|---|---|---|
| `ACTIVE` | enabled | Start, renewal, payment recovery and cancel-resume end here. Manual cancellation remains `ACTIVE` with `cancel_at_period_end=true`. |
| `GRACE_PERIOD` | enabled | Failed payment only; `grace_period_end = failure_effective_at + P7D`. A newer failure for the same unresolved period does not restart seven days. |
| `INACTIVE` | disabled | Paid period end or grace expiry. Repurchase creates a later `ACTIVE` revision on the same aggregate key. |

For `ACTIVE`, `entitled_until=current_period_end`; for `GRACE_PERIOD`,
`entitled_until=grace_period_end`; for `INACTIVE`, it equals `effective_at` and
grants nothing. Equality is always not entitled. A provider pause is not a
fourth canonical state: when its verified snapshot preserves service through the
paid period it becomes `ACTIVE` with `cancel_at_period_end=true`, followed by
`INACTIVE`; if pause semantics/effective time cannot be established, the adapter
reconciles/quarantines instead of guessing.

Provider webhook arrival order is never used as lifecycle order. A verified
adapter normalizes provider/event/subscription IDs, target/product, provider
event time, billing period and a monotonic provider version or authoritative
provider snapshot. If a provider supplies no comparable version, the adapter
must reconcile the provider subscription before it may regress local state.
Unknown or ambiguous facts are retryable/quarantined. The deterministic fake
provider supplies versions and drives the same state machine; no live secret is
required for implementation or CI.

The exact `(provider, provider_event_id)` replay returns the persisted outcome
with no new revision/event. While billing detail is retained, reuse with changed
request bytes is a contract mismatch. After Space purge, the existing permanent
purpose-scoped HMAC fence and its saved terminal outcome remain authoritative;
the event must never recreate purged billing detail.

### Account delete, restore and purge

The account-deletion coordinator calls Subscription through a durable idempotent
operation. Scheduling deletion immediately transitions the personal aggregate to
`INACTIVE(ACCOUNT_DELETE_SCHEDULED)`, suppresses reminders, and atomically adds a
leased provider operation that cancels auto-renewal. Provider failure retries and
alerts but never restores access. That final raw personal snapshot carries an
opaque permanent `deletion_fence_id`, one-use `deletion_cycle_id`, and `purge_at`.
Explicit restore inside `P30D` cancels that cycle and first reconciles
the provider; verified remaining paid time may produce
`ACTIVE(ACCOUNT_RESTORED, cancel_at_period_end=true)`, but recurring billing is
not resumed without a separate explicit user action.

For Space Pro purchased by that account, Subscription cancels renewal and
suppresses purchaser reminders without transferring billing identity. The Space
keeps already-paid entitlement only through verified `current_period_end`, then
becomes `INACTIVE`. Each consumer persists its own DB-time purge job from the
scheduled-deletion snapshot; at `P30D` raw erasure is independent and no
participant receipt gates another. Offline recovery applies Subscription's
protected permanent purge-fence snapshot before serving and emits a late receipt.
`ACCOUNT_PURGED` is terminal HMAC-fence state, not a raw entitlement snapshot. A
missing provider cancel receipt likewise never blocks the `P30D` privacy deadline:
Subscription atomically detaches only provider + opaque subscription/cancel
handle into a random-ID, cancellation-only escrow with no raw account/purchaser
link or payment detail. Its sole worker retries until terminal receipt, then
crypto-shreds the handle; missing receipt pages from `P25D`. Merchant activation
requires provider API plus an audited manual path that can service this escrow.
At its deadline Subscription erases/crypto-shreds raw account/purchaser IDs from current state, journal/outbox,
quarantine, Analytics and provider detail even inside normal retention, and
deletes each recorded retained JetStream sequence carrying those raw IDs. Purge
receipts include the stream deletion; restore reapplies permanent purge
tombstones before a backup can serve. Only a permanent
purpose-scoped HMAC tombstone keeps aggregate/provider replay keys, last revision,
deletion fence/cycle, event IDs/hashes and terminal outcomes. It is excluded from
raw entitlement snapshots and included in the permanent purge-fence snapshot;
provider replay returns the fence and cannot resurrect billing detail.

If a consumer missed delete scheduling and restored only a legacy raw aggregate
key, it must call protected batch `ResolvePurgeFenceByLegacyAggregate` before
serving. Subscription computes purpose HMACs against retained key versions and
returns only `PURGED`, fence ID/cycle and `purged_at`, or explicit `NOT_FOUND`.
Only allowlisted participant workloads may call it; raw request IDs are neither
logged nor retained and audit uses only the returned fence ID.

Space aggregates remain keyed by `space_id`: purge clears raw
`purchaser_account_id`, sets `purchaser_deleted=true`, retains only its HMAC
billing/replay fence (an unlinked cancellation escrow may temporarily keep the
opaque provider handle), increments the Space revision and emits
`PURCHASER_PURGED` without changing paid state/deadline. Later snapshots and the
period-end `INACTIVE` event use the deleted marker and remain routable without a
raw payer ID.

### Authoritative event and outbox

Add the additive subject `subscription.entitlement_changed` and an
`EntitlementChanged` arm to `SubscriptionStreamEvent`. Its envelope is:

```text
event_id: UUID fixed before the database commit
occurred_at: Subscription database time
protocol_version: 1
aggregate_kind: PERSONAL | SPACE
aggregate_id: account_id | space_id
aggregate_revision: positive, monotonic per aggregate
```

The payload is a complete snapshot, not a delta: entitlement ID, plan, personal
`account_id` or Space `space_id` plus optional `purchaser_account_id` and
`purchaser_deleted`, opaque `deletion_fence_id`, optional
`deletion_cycle_id`/`purge_at`, state,
`cancel_at_period_end`, `effective_at`, `entitled_until`,
optional `downgrade_cycle_id`, `current_period_end`, optional
`grace_period_end`, and a closed reason enum:
`STARTED`, `RENEWED`, `PAYMENT_FAILED`,
`PAYMENT_RECOVERED`, `CANCEL_SCHEDULED`, `CANCEL_RESUMED`, `PERIOD_ENDED`, or
`GRACE_EXPIRED`, `ACCOUNT_DELETE_SCHEDULED`, `ACCOUNT_RESTORED`, or
`PURCHASER_PURGED` (Space metadata transition). `ACCOUNT_PURGED` exists only in
the terminal deletion-fence journal/snapshot. `ACTIVE`/`GRACE_PERIOD` are entitled only while consumer
database time is strictly before `entitled_until`; equality is inactive and
requires reconciliation. `INACTIVE` is not entitled. Thus missing renewal or
expiry delivery cannot preserve benefits forever.

The aggregate mutation and immutable `subscription_event_outbox` rows commit in
one `subscription_db` transaction. The outbox owns subject, deterministic proto
bytes/SHA-256, event kind, aggregate key/revision, `available_at`, attempt/error,
fenced lease/delivery timestamps and the PubAck stream sequence. A bounded dispatcher claims ready or
expired leases with `FOR UPDATE SKIP LOCKED`, publishes the stored bytes with
`Nats-Msg-Id=event_id`, requires JetStream PubAck and completes only its current
lease token. Lost response and crash after PubAck may resend only the same ID and
bytes. Undelivered rows do not expire; delivered rows follow the documented
`P400D` operations retention policy in [DATA_STORES.md](../DATA_STORES.md).

Legacy plan/payment/Space-Pro subjects may be emitted from the same transaction
during a measured compatibility window. Each legacy child has its own stable ID
derived from canonical event ID + payload kind, includes that canonical ID as
correlation, and never reuses one `Nats-Msg-Id` for different bytes. They are no
longer entitlement authority. In particular, `plan_cancelled` means cancellation
was scheduled, not that benefits ended; consumers change access only from
`subscription.entitlement_changed`. Analytics records a cutover watermark: after
it, legacy plan/Space/downgrade events cannot create lifecycle metrics, while
payment subjects remain billing-attempt telemetry.

### Grace reminder claims

Entering either personal or Space Pro grace inserts D1/D3/D7 scheduled outbox
rows atomically. The unique logical key is `(aggregate_kind, aggregate_id,
grace_revision, day)`. D1 is due at grace start, D3 at `+P2D`, D7 at `+P6D`;
each row's delivery window closes at the next reminder boundary or grace end and
records terminal `SUPPRESSED`/`EXPIRED` state instead of disappearing. A claim
re-locks the aggregate and publishes only if the same revision is still
`GRACE_PERIOD` in the row's window. Recovery, expiry or a newer revision cancels remaining rows. Thus
multiple replicas, crash around PubAck and a late worker cannot create a new ID,
duplicate effect, or a burst of old reminders.

The reminder payload references the grace revision. Personal rows carry payer
`account_id`; Space rows carry `space_id` and `purchaser_account_id`. Notification
routes one logical/in-app reminder plus an at-least-once push signal with the same
client ID to the payer's authoritative primary profile.
Email remains restricted to authentication events.

### Consumer transaction and replay contract

Auth, User, File, Space, Voice and Notification each store a local
`subscription_event_inbox` and aggregate projection in their PostgreSQL database.
Analytics instead deduplicates canonical event IDs in ClickHouse. Inside one
enforcement-consumer transaction they validate and hash the envelope, insert
`(consumer_name,event_id,payload_hash)`, lock the aggregate projection and:

- treat exact event replay as success with no side effect;
- record lower revisions as stale no-ops;
- accept the same revision only when the complete snapshot is identical;
- atomically replace the projection and apply a higher revision;
- quarantine/page the same event ID with other bytes or the same revision with
  another snapshot;
- ACK only after commit; parse/store/dependency failures NAK and retry. Unknown
  payload/protocol is durably quarantined and terminated/moved to DLQ after
  bounded retries, never ACKed as successful application.

A complete higher snapshot may apply before a lower one; later delivery of the
lower revision cannot undo it. Enforcement consumers first create their durable
consumer, then open an allowlisted protected snapshot session using one
repeatable-read owner watermark and signed scope-bound cursors. All non-purged
permanent aggregates, including `INACTIVE` tombstones, appear once; only the final page says
`complete=true`, and failed/absent pages never delete local state. Consumers
apply pages by source revision, drain queued events, and repeat the same protocol
for periodic reconciliation. Every enforcement projection persists
`entitled_until` and fails closed/reconciles at its boundary.

Subscription is the sole boundary-decision owner. Protected
`ResolveEntitlementAtBoundary` accepts aggregate key, observed deadline and
`minimum_revision`, and returns one complete current snapshot whose revision is
at least that minimum. Callers use `min(500ms, remaining request deadline)` and
do not retry beyond it. Timeout/auth failure or an ambiguous lower/missing result
cannot extend paid access; interactive enforcement falls back to the base/free
path and triggers reconciliation, while reminder delivery retries only inside
its scheduled window and otherwise stores `SUPPRESSED`.

Analytics instead replays a protected immutable provider-neutral lifecycle
journal by canonical event ID because current snapshots cannot recreate reason
history. It then drains NATS from the durable created before replay. Poison or
unsupported protocol events are durably quarantined with bytes/hash/error and an
alert; after bounded redelivery the consumer terminates/moves that delivery to
DLQ rather than ACKing it as successful application. Upgrade/operator replay
uses the quarantine record and the same event ID.
`DeliverNew`, a seven-day stream, an in-memory map or an unbounded timestamp
guess is insufficient startup or restore evidence.

### Downstream effects

- **Auth:** persists personal tier plus source revision in `auth_db`; access JWT
  includes `subscription_revision` and `subscription_entitled_until` from this
  durable projection. Paid benefit expires at equality even while the JWT itself
  remains valid; a boundary decision can discover a newer grace/renewal revision
  but dependency failure cannot extend paid access. Missing local projection
  triggers protected reconciliation, not a silent long-lived free default.
- **User:** scheduled cancel/grace reserves the primary slot and accepts an exact
  primary+eligible-secondary pair when such a secondary exists. The selection
  uses a stable `downgrade_cycle_id` preserved
  through any intervening failed-payment grace and its direct `INACTIVE`
  successor, then cleared by recovery/resume/renewal or new `STARTED` activation;
  `INACTIVE` applies it or deterministic primary+oldest eligible fallback even
  if no client is online. Every `ACTIVE` result, including a new purchase,
  unfreezes only subscription-owned freezes and clears the old cycle/selection;
  `GRACE_PERIOD` does not freeze. See [user-service.md](user-service.md).
- **File:** stores immutable verified `retention_account_id` on each exact
  reference. Free reference starts P90D from creation; activation clears expiry and downgrade gives
  every still-live non-E2E reference P90D from effective downgrade. Renewal can
  rescue only a live reference; E2E stays P90D from upload. See
  [file-service.md](file-service.md).
- **Space:** stores exact source revision; Pro caps remain through grace and are
  removed only at `INACTIVE`. Existing members/resources are grandfathered;
  growth above free caps is denied. See [space-service.md](space-service.md).
- **Voice:** stores Space revision/deadline for 32/128 room admission and uses a
  short-lived Auth entitlement claim for personal quality; both fail closed at
  `entitled_until`. See [voice-service.md](voice-service.md).
- **Notification:** persists event inbox plus a leased channel-dispatch outbox,
  waits for matching grace and checks the authoritative state immediately before
  send. Recovery before that compare suppresses dispatch; an already sent
  provider request cannot be recalled. See
  [notification-service.md](notification-service.md).
- **Analytics:** records all supported lifecycle reasons idempotently by event ID;
  analytics failure never becomes entitlement authority.

## P3 Space deletion and provider dedup (accepted target)

Subscription stores a durable lifecycle fence, operation receipts and compact
provider-event dedup fences. `FROZEN` denies entitlement use/mutation for the
Space; restore accepts only the next `LIVE` generation. Purge cancels renewal,
removes the active Space entitlement/projection and its billing detail, then
returns an immutable completion receipt bound to the root manifest. Full
request/receipt bytes retain 30 days from this participant's completion and
compact `PURGED` state is permanent.

Billing-detail removal never removes webhook idempotency authority. The compact
fence retains only provider, purpose-specific HMAC-SHA-256 of the exact provider
event ID, `first_seen_at`, stable terminal outcome class, `key_version` and
`retain_until`. HMAC input is UTF-8
`voice-subscription-provider-dedup-v1`, NUL, canonical lowercase provider, NUL,
then exact provider-event bytes. A transient or rolled-back attempt writes no
fence. Both Paddle and CloudPayments fences are permanent because no bounded
maximum replay/reconciliation/dispute-redelivery window is canonical.

Only Subscription workload identity may compute or transactionally read the
fence; there is no public/admin/staff/break-glass read surface. Its distinct
KMS/HSM family rotates every `P90D`, fails closed and has audited rotation and
destruction. The maximum restorable-backup window is `P30D`. Because fences are
permanent, old key versions also remain permanent until a separately reviewed
migration re-HMACs every fence. This HMAC evidence is accepted as compatible
with account erasure.
