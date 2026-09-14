# Analytics Search/Voice dashboards: Query API RED contract

## Status and purpose

This document records the backend contract gap that blocks the Search and Voice
staff dashboards. It is a **RED contract**, not an accepted runtime design and
not evidence that either dashboard is implemented.

The product docs name the desired panels, but they do not yet define enough
wire, aggregation, filtering, privacy, or source-event semantics to implement
them without guessing. The missing decisions below must be accepted in the
canonical Analytics, Search, and Voice docs before protobuf, producer, query,
Gateway, Grafana, or Admin UI work starts.

## Sources and current evidence

- Product scope and named panels:
  [analytics.md](../features/analytics.md).
- Analytics service, storage, and current REST list:
  [analytics-service.md](../microservices/analytics-service.md).
- Search telemetry table:
  [search-service.md](../microservices/search-service.md).
- Voice event table and media boundary:
  [voice-service.md](../microservices/voice-service.md).
- Canonical Query API wire:
  [`analytics.proto`](../../protos/voice/analytics/v1/analytics.proto).
- Current query and normalization paths:
  `src/backend/analytics/internal/store/query.go`,
  `src/backend/analytics/internal/grpcsvc/query.go`, and
  `src/backend/analytics/internal/adapters/domain.go`.
- Current Search publisher:
  `src/backend/search/internal/grpcsvc/search.go`.
- Canonical Voice stream payloads:
  [`jetstream_events.proto`](../../protos/voice/events/v1/jetstream_events.proto).

Observed current state:

| Surface | Evidence |
|---|---|
| Dashboard type | `GetDashboardRequest.dashboard_type` is a free-form string. The proto comment and runtime support only `product`, `engagement`, `revenue`, `health`, and `moderation`. |
| Dashboard filters | `GetDashboardRequest` has only `from` and `to`; it cannot carry filters. `GetMetricsRequest.filters` is a string map and the runtime recognizes only `event_type`, only for the `health` query. |
| Metric shape | `MetricPoint` has `name`, numeric `value`, and one optional string `label`; it has no timestamp/bucket boundary, unit, value kind, or typed dimensions. |
| Search source | `SearchGlobal` publishes on the documented `analytics.search.query` subject, but its envelope uses `event_type=query_executed` with only `query_len` and `message_hits`. It emits no profile identity, scope, total results across result kinds, zero-result event, or result-click event. The other Search RPC paths do not publish Analytics telemetry. |
| Voice source | `VoiceStreamEvent` carries `room_id` on start/end and `duration_seconds` on end. The Analytics adapter omits start `room_id`, drops `duration_seconds`, maps only screen-share start, and has no codec fact. Voice Service does not process media streams, so it cannot infer negotiated codec mix from its orchestration state. |
| Storage | Official reads use deduplicated `voice.events_logical`. Raw events have a 90-day TTL. Aggregates may remain indefinitely only when de-identified. |

## Existing accepted invariants

The eventual contract must preserve these already documented requirements:

1. `/api/v1/analytics/**` is staff-only through Gateway. It is not a Flutter
   user API or a Space-owner self-service surface.
2. Raw Analytics events retain for 90 days. Longer-lived aggregates are
   de-identified. The accepted Space deletion rule is narrower and additionally
   allows only de-identified counts with no aggregate keyed by a raw deleted
   Space ID.
3. Account/profile identifiers use Analytics-domain HMAC fields. Arbitrary
   message content and PII are forbidden in `properties`.
4. Official queries deduplicate source redelivery by stable `event_id` through
   `voice.events_logical`; a dashboard must not count raw redelivery rows.
5. Search panels are limited to the already named product concepts:
   **queries/day**, **zero-result rate**, and **average result click position**.
6. Voice panels are limited to the already named product concepts:
   **concurrent calls**, **average call duration**, **screen shares**, and
   **codec distribution**.
7. Federation analytics remains deferred and is not part of this contract.

These names do not by themselves define formulas or authorize adjacent
metrics. Implementations must not substitute convenient counters such as
`message_hits`, call-event count, or configured codec support for the named
product metrics.

## Required Query API decisions

### Dashboard and point types

Before adding `search` and `voice` to `dashboard_type`, the contract must define:

- whether every metric is a scalar summary, a daily series in an explicitly
  chosen reporting timezone, a categorical distribution, or another explicit
  value kind;
- the unit and numeric representation for counts, durations, rates, and shares;
- how a point unambiguously represents its bucket and categorical dimensions;
- stable response ordering and whether empty buckets/categories are returned;
- compatibility behavior for older Gateway/Admin clients;
- the client-visible error for an unsupported dashboard type.

The current `MetricPoint` may be retained only if the accepted response can
unambiguously represent all named metrics. Encoding a date, unit, codec, and
other dimensions into the single free-form `label` without a documented schema
is not acceptable.

### Time range

The contract must define all of the following together:

- UTC normalization and exact inclusive/exclusive boundaries for `from` and
  `to`;
- the default range when either bound is absent;
- validation of invalid timestamps, `from >= to`, future bounds, and ranges
  crossing the raw 90-day TTL;
- the daily bucket timezone and partial first/last bucket behavior;
- whether a request outside raw retention is rejected, clipped, or answered
  from a named de-identified aggregate.

Current implementation defaults to a moving 30-day range and commonly queries
`timestamp >= from AND timestamp < to`; this is implementation evidence, not a
complete Search/Voice contract.

### Filters

The accepted contract must choose whether filters belong on
`GetDashboardRequest`, on a separate typed request, or only on narrowly named
`GetMetrics` calls. For every supported dashboard it must define:

- the exact allowlisted keys, value enums, and normalization;
- whether multiple values are union or intersection;
- behavior for duplicate, empty, unknown, or disallowed filters;
- whether filters change a metric denominator;
- audit requirements and maximum cardinality/cost;
- privacy-safe treatment of identifiers.

There is no accepted Search/Voice filter allowlist today. Raw query text cannot
be retained or passed into Analytics as an ad-hoc property/filter under the
current content/PII rule. For account/profile identities, the contract must
decide whether staff input is forbidden or accepted only for server-side
Analytics-domain HMAC matching, with explicit audit and non-disclosure behavior.
Space/chat/room/result identifiers likewise require an explicit classification,
transformation, retention, and deletion decision; the Search telemetry table's
existing `result_id` label does not answer those questions. The existing
`event_type` metrics filter does not implicitly become a dashboard filter.

## Required metric decisions and source facts

### Search

| Named metric | Decision required before query SQL | Missing source/contract evidence |
|---|---|---|
| Queries/day | Define whether this is a daily count series or a range-normalized scalar; define which of `SearchGlobal`, `SearchInChat`, `SearchUsers`, and `SearchSpaces` count, which scopes they represent, and how multi-page requests/retries are deduplicated. | `SearchGlobal` publishes on `analytics.search.query`, but its envelope uses `query_executed`; the other RPC paths emit no telemetry. Current properties omit the documented profile identity, `scope`, and total results. |
| Zero-result rate | Define numerator, denominator, units, observation stage (before or after authorization/privacy filtering), and which of messages, profiles, chats, Spaces, and future result kinds participate. | No `search.zero_results` event is emitted. `message_hits == 0` is not proof that a global search returned zero results. |
| Average result click position | Define a click, 0-based vs 1-based position, ordering across mixed result kinds, correlation window, repeated clicks, and the no-click case. | No click event is emitted. The documented `search.result_clicked` payload has no position or query correlation field. |

Search query strings are arbitrary user input and may contain message content or
PII. The conflict between the Search telemetry table (which lists `query`) and
Analytics privacy rules must be resolved explicitly. Until then, raw query text
must not be added to Analytics properties merely to satisfy a dashboard.

The Search contract must also choose the identity and delivery behavior for the
accepted operations. The documented `profile_id` must use the Analytics-domain
HMAC field rather than a raw property; the current generic `Publish` call sets
neither identity field. Analytics publication errors are currently discarded,
so the accepted design must state whether loss is sampled/best-effort, retried
durably, or made observable without turning Analytics into Search authority.

### Voice

| Named metric | Decision required before query SQL | Missing source/contract evidence |
|---|---|---|
| Concurrent calls | Define point-in-time, peak, or average concurrency; call/room types included; carry-in calls active at `from`; matching and incomplete lifecycle behavior; and whether participants or rooms are counted. | Start/end can carry `room_id`, but the Analytics start mapping omits it. The raw 90-day boundary cannot reconstruct an older still-active interval without an accepted snapshot/aggregate rule. |
| Average call duration | Define units, included room/call types and end reasons, treatment of active/incomplete calls, and invalid/negative/capped durations. | `CallEnded.duration_seconds` exists, but the Analytics adapter drops it. |
| Screen shares | Define whether the metric is starts, unique sessions, duration, or concurrency; define room types and incomplete stop handling. | Analytics currently maps only `screen_share_started`; no accepted session/duration aggregation exists. |
| Codec distribution | Define media-source authority, audio/video/screen dimensions, negotiated vs configured codec, simulcast/layer behavior, and denominator. | Voice Service explicitly does not process media. No canonical Voice/Analytics event carries a negotiated codec. Configured Opus/VP8/VP9 support is not observed usage. |

The dashboard must not derive codec distribution from server configuration, nor
infer concurrency or duration from coarse counts of `call_started` and
`call_ended` events.

## Privacy and retention decisions still required

Before source events or filters are extended, the accepted contract must state:

- which Search fields are omitted, bucketed, HMACed, or allowlisted; arbitrary
  query text cannot pass through as a property under the current privacy rule;
- whether result, chat, room, and Space identifiers are needed at all and, if
  needed, their field classification, transformation, retention, and deletion
  behavior (the current docs and adapters contain raw domain IDs but do not
  establish one cross-domain rule);
- whether click correlation uses a random short-lived analytics ID and its
  retention, rather than a raw query or domain object ID;
- which de-identified daily aggregates survive raw TTL and which dimensions are
  safe to retain indefinitely;
- how a requested range crossing the 90-day raw TTL reports partial or
  aggregate-only coverage.

The contract must also decide whether a dashboard response can contain any raw
event property or hashed subject ID, or must remain aggregate-only like the
current `GetDashboardResponse`. Staff authorization does not by itself resolve
that response-privacy decision or waive the Analytics privacy boundary.

## RED acceptance matrix

| ID | Required failing proof before implementation | Expected GREEN after an accepted contract |
|---|---|---|
| ASD-R01 | `GetDashboard(search)` and `GetDashboard(voice)` are unsupported. | Both accepted type values return the exact documented metric set; unknown types use the accepted client error. |
| ASD-R02 | A named daily series/distribution cannot be decoded unambiguously from `MetricPoint`. | Generated clients decode the accepted unambiguous representation of buckets, units, value kinds, and dimensions with stable ordering. |
| ASD-R03 | Invalid/reversed/future/out-of-retention ranges are unspecified or silently normalized. | Unit and Gateway contract tests prove the accepted range/error/coverage policy. |
| ASD-R04 | Dashboard filters have no request field or allowlist. | Positive and negative tests prove exact keys, enums, denominator effects, cost limits, raw-query rejection, and the accepted reject/transform/audit behavior for each identifier input. |
| ASD-R05 | Only `SearchGlobal` publishes; it uses `query_executed{query_len,message_hits}`, sets no Analytics identity, and discards publish errors. | The accepted RPC/scope set uses one stable taxonomy, Analytics-domain profile hashing, defined pagination/retry dedupe, and documented loss/retry observability without raw query text or PII. |
| ASD-R06 | A global search with zero message hits but a profile/chat/Space hit looks like zero under current data. | Zero-result evidence is derived at the accepted observation stage across the accepted result kinds. |
| ASD-R07 | No click position/correlation fact exists. | Click telemetry proves the accepted position and correlation semantics without retaining raw query/domain IDs beyond policy. |
| ASD-R08 | Voice adapter drops start room identity and end duration. | Deduplicated lifecycle facts supply the accepted duration and concurrency inputs, including boundary/incomplete-event tests. |
| ASD-R09 | Screen-share start count is the only mapped fact. | Source and query tests prove the accepted start/session/duration/concurrency definition and incomplete stop behavior. |
| ASD-R10 | No negotiated codec event exists. | An explicitly owned media-authority source supplies the accepted codec dimensions; configured support is rejected as usage evidence. |
| ASD-R11 | Queries can scan only raw rows while raw TTL is 90 days. | Requests crossing retention follow the accepted reject/clip/de-identified aggregate policy and report unambiguous coverage. |
| ASD-R12 | Raw query text, PII, or an identifier contrary to its accepted classification is retained, queried, or returned. | Tests reject prohibited fields, enforce the accepted input/storage/response treatment of account/profile and each allowlisted domain identifier, and expose only the accepted response dimensions. |
| ASD-R13 | Redelivered source events can appear more than once in raw storage. | Every official metric reads the logical deduplicated source or an equivalently deduplicated aggregate. |
| ASD-R14 | Staff/user access is tested only for the existing product dashboard. | Gateway tests prove staff access and ordinary-user denial for both new types without exposing a user or Space-owner route. |

## Implementation order after decisions are accepted

1. Update `analytics.proto` and the Analytics/Search/Voice docs in one contract
   PR; run buf lint, format, breaking, and generated-stub checks.
2. Add producer/adapter RED tests and then publish only the accepted privacy-safe
   source facts.
3. Add Analytics query-store RED tests for formulas, boundaries, filters,
   dedupe, and retention; implement the smallest ClickHouse queries/aggregates.
4. Add gRPC/Gateway contract tests, then expose the accepted routes and errors.
5. Add Grafana/Admin UI work only after the backend contract is green.
6. Add compose evidence that real Search and Voice actions reach ClickHouse and
   the staff dashboard while an ordinary user receives `403`.

Each runtime slice is a separate PR owner and must pass exact-head CI before a
merge commit. Until the decisions above are accepted, adding partial counters
under the final metric names would make the API misleading and is not a valid
GREEN implementation.
