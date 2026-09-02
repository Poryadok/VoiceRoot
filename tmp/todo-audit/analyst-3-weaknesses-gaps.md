# Analyst 3 — Weaknesses & Gaps (Batches 1–29, PRs #74–#114)

**Scope:** Quality risks, partial slices, and tech debt left open by the batch loop — not a completion inventory (see Analyst 1).  
**Sources:** `docs/todo/*.md` (open + `partial`/`deferred` notes), spot-checks in `src/`, batch themes from agent transcripts.  
**Note:** `git pull origin master` was blocked by a local hook; analysis uses the workspace as-is.

---

## Risk summary

### P0 — Product/security/revenue blockers

| Theme | Risk |
|-------|------|
| **Billing** | Checkout is a hardcoded Paddle test URL; CloudPayments webhook `Unimplemented`; cancel/resume never hits providers; JWT `subscription_tier` stays `free` without `AUTH_NATS_URL` — premium limits silently wrong outside File Gateway override. |
| **Messaging contract** | `schedule_message` / `send_when_online` / `send_silent` absent from proto; `video_note`/`music` validation open; `ForwardMessage` skips slow-mode/moderation/attachment guards — batch send-perm work does not extend to forwards. |
| **Space** | Tree **pin** (`is_pinned`/`pin_order`, `PinTreeNode` RPC) missing despite Gateway tree/reorder shipped; owner RPCs (`DeleteSpace`, `TransferOwnership`, audit log) still embed-`Unimplemented`; `entry_requirement` rejects non-`none` with generic error (no captcha/approval queue). |
| **Bots** | `lookupInteraction` always sets `CHAT_TYPE_CHANNEL` — deferred `SendBotMessage`/`CompleteInteraction` broken for DM/group (Batch 15 bot platform). |
| **Auth delete-account** | REST + Flutter delete UI shipped (Batch 28b), but tombstone incomplete: no “user deleted” in DM, no hide from `ListChats`; restore RPC exists, no client. |

### P1 — Shipped slices with hidden weakness

| Theme | Risk |
|-------|------|
| **Client stubs masquerading as features** | `syncPhoneContacts(const [])` always shows stub snackbar; backend + compose live test exist (`TestComposePhoneSync_live`) but mobile has no contact picker/hash pipeline. QR add-friend is paste-only (l10n promises camera). |
| **R2-A04 mobile shell** | Tab bar, drawer, keyboard-hide, overlay strip hide (Batch 29a) shipped incrementally; drawer still documented as stub; stacked chrome overflow (strip + pinned + composer + tabs) deferred — widget smoke only, no Penpot · v3 parity. |
| **Stickers/GIF** | Backend `content_type` enum + partial validation; **zero** Flutter composer tabs/send path; Penpot R4-04-L04 explicitly deferred. Users see emoji panel only. |
| **Moderation notifications** | `moderation.events` consumer runs but `routeModerationNotification` passes empty `recipientProfileID` — push/in-app for sanctions is a no-op ack. Appeals UI (Batch 27a) has HTTP test only, no widget test. |
| **Voice roles** | `VOICE_JOIN` + screen-share enforced; `VOICE_SPEAK` / `VOICE_MUTE_OTHERS` not on speak/mute paths — permission UI can lie. |
| **Analytics admin** | Admin pages call `GetDashboard` for engagement/revenue/health/moderation; **search/voice** dashboard types missing in `analytics/internal/store/query.go` — pages 404 or empty. |
| **CI regression window** | Tier-2 compose/flutter smokes do not block PRs (by design, Batch 7); master-only — batch merges can regress cross-service paths before nightly. |

### P2 — Debt / polish / human-blocked

| Theme | Risk |
|-------|------|
| **Penpot · v3** | Entire Telegram-parity frame registry deferred; Flutter shipped ahead of design (folders, QA, composer attach matrix, stickers/GIF). |
| **CI secrets / prod** | Staging k8s, billing, FCM, Alertmanager, DNS — human-blocked; prod `*.voice.example.com` placeholders; prod deploy lacks selective rollout/stack.lock. |
| **Cross-cutting** | 12+ `internal/authctx/` copies; no distributed tracing; `subscription.events` bus spec vs `analytics.subscription.*` only; partial-feature E2E (premium→cosmetics, grace→push) missing. |
| **Developer Portal** | Shipped + CI (Batch 6) but omitted from `PLAN.md` implementation map. |

---

## Weaknesses table

| Priority | Component | Gap | Why it matters | Effort |
|----------|-----------|-----|----------------|--------|
| P0 | Subscription | `CreateCheckoutSession` returns `checkout.paddle.test`; no CloudPayments | No real revenue path; staging with test URLs is a footgun | L |
| P0 | Auth / JWT | `InMemorySubscriptionTierStore` when `AUTH_NATS_URL` unset | User/Chat trust JWT tier; GIF/banner/profile caps wrong in prod misconfig | S (ops) / M (code guard) |
| P0 | Space | Tree pin migration + RPCs absent; Gateway has reorder only | Spec P0 in `backend.md`; pin UX cannot ship | M |
| P0 | Space | `DeleteSpace`/`TransferOwnership`/audit `Unimplemented` | Owner cannot safely leave or delete space | M |
| P0 | Messaging | `schedule_message`, `send_silent`, `send_when_online` not in proto | Telegram-parity composer backlog; batches added delivery ack but not scheduling | L |
| P0 | Messaging | `ForwardMessage` skips slow-mode, moderation, attachment validate | Forward path weaker than send after Batch 14 permission work | M |
| P0 | Bot | `lookupInteraction` → always `CHAT_TYPE_CHANNEL` | Breaks deferred interactions in DM/group bot flows | S |
| P0 | Auth | Delete-account tombstone: no DM label, no `ListChats` hide | Delete UI (28b) looks done; social graph still shows ghost users | M |
| P1 | Flutter Social | Phone sync UI calls API with `[]` hashes | Backend + gateway + compose test exist; feature is demo-only | M |
| P1 | Flutter Social | QR scanner: paste field only, no `mobile_scanner`/camera | Batch 26a ships half the spec; misleading copy in l10n | M |
| P1 | Flutter Shell | R2-A04 stacked chrome polish deferred | Overflow on narrow + keyboard + multi-pin (29a partial) | M |
| P1 | Flutter Shell | `MobileShellDrawer` labeled stub; no parity with desktop rail | Folders/QA wired but mobile IA incomplete vs §1.1 | M |
| P1 | Flutter Chat | No stickers/GIF composer UI | `content_type` backend partial; Penpot deferred | L |
| P1 | Flutter Chat | `ComposerEmojiPanelBody` — emoji only, no sticker/GIF tabs | §3.6b parity gap | M |
| P1 | Flutter Auth | No password-reset screen; REST exists | Batch 29b added sessions; reset still missing | M |
| P1 | Flutter Auth | No restore-account UI | Delete shipped 28b; recovery path dead | S |
| P1 | Flutter Auth | Phone + OTP auth screen missing | Email-only `auth_screen.dart` vs spec default phone | L |
| P1 | Moderation | Notification consumer acks without delivery (`recipientProfileID ""`) | Sanction push never reaches user despite consumer existing | M |
| P1 | Moderation | `appeal_sheet.dart` — no widget test (only `moderation_client_test`) | Batch 27a regression risk on settings navigation | S |
| P1 | Voice | `VOICE_SPEAK` / `VOICE_MUTE_OTHERS` not enforced on speak/mute | Role wiring partial after Batch 14 join deny | M |
| P1 | Analytics | Admin search/voice dashboard types missing in `query.go` | Admin UI routes exist; backend returns unknown type error | M |
| P1 | Analytics | CH ingest ack-before-durable-write | Event loss on crash after NATS ack | M |
| P1 | Search | Index failures logged but acked; no `Nak` | Silent search index holes | M |
| P1 | Search | No handlers for rename/visibility/tree changes | Stale search after chat/space updates | M |
| P1 | Chat | `ListChats` rows omit `e2e_enabled`, `space_id`, thread flags for link UI | List enrichment batches (12–13) incomplete for badges | M |
| P1 | Messaging | `MarkRead`/`GetReadState` DM-only (`validateChatRefDM`) | Group read receipts IT exists but API rejects non-DM refs | M |
| P1 | File | No SHA dedup, `file_references`, expiry placeholder UX | Attachment lifecycle partial after messaging validate batches | L |
| P1 | Subscription | `GetBillingHistory` empty; grace expiry no sweeper | Billing UI cannot show history; stuck grace | M |
| P1 | CI | PRs not gated on tier-2 E2E | Regressions land on master until nightly/full | S (policy) |
| P1 | CI | Billing/Auth/FCM secrets human-blocked | Blocks real staging validation of batches 16–29 billing/social | — (human) |
| P2 | Design | Penpot · v3 registry + stickers/GIF panel frames | Flutter ahead of design; audit trail gap | L |
| P2 | Design | Desktop Room · v2 missing composer CTAs (attach tabs, send menu) | FEATURES entry points absent in mockups | L |
| P2 | Client | `profile_accent_storage` dual-write stale | Batch 17 server accent vs local index drift | S |
| P2 | Client | Guest save-account reminder local-only | Cross-device reminder not synced | S |
| P2 | Client | `VoiceListSkeleton` / `api_error_messages` coverage gaps | Wave A–J marked done in places but loaders/errors inconsistent | M |
| P2 | Client | Idle presence `UpdatePresence idle` client-driven only | No 5-min idle auto | S |
| P2 | Admin | Developer Portal missing from `PLAN.md` | Audit/orientation drift after Batch 6 | S |
| P2 | Bot | Inbound `message` → bot webhook consumer absent | Outbound `bot.events` only; platform half-wired | L |
| P2 | Bot | Token rotation does not purge deferred hub sessions | Security gap per `bots.md` | M |
| P2 | Cross-cutting | `subscription.events` bus not published | Tier propagation ad hoc vs CONTRACT_MATRIX | M |
| P2 | Cross-cutting | Partial premium/MM/bot notification E2E missing | Isolated tests per batch; no cross-smoke | M |
| P2 | Prod deploy | No selective deploy / stack.lock on prod | Rollout risk unlike staging | M |
| P2 | Docs hygiene | Stale todo claims (see below) | False confidence in audit | S |

### Stale or overstated “done” notes (quality signal)

These are **not** new inventory items — they show where batch closures left partial truth in `docs/todo/`:

| Claim | Actual state |
|-------|----------------|
| Shadow-ban “не режет fanout” | `SendMessage` sets `ghost_only`, suppresses `message.sent` when banned — partial fix; Realtime still has no `ghost_only` path (events not published). |
| `social.events` “zero publisher” | `social/main.go` wires JetStream publisher; friend request/accept/remove publish. |
| `GuestConvertNatsEventIntegrationTest` false positive | Test now calls `convert-guest` and asserts subject — note in `backend.md` is stale. |
| E2E “smoke omits ws_resume/delivery/notifications” | `e2e-features.yml` `smoke_gateway` + `smoke_flutter` include them — `backend.md` cross-cutting bullet stale. |
| Moderation “Notification не consumит moderation.events” | Consumer exists; delivery is stubbed (empty profile ID). |

---

## Recommended next batches (grouped)

### Batch A — Revenue & tier correctness (P0)
- Wire real Paddle checkout + webhook lifecycle (renew/cancel/grace sweeper).
- Guard Auth: fail loudly or default-deny premium when `AUTH_NATS_URL` missing in non-dev.
- `GetBillingHistory` + Flutter billing history screen.
- **Human parallel:** staging billing secrets (`ci.md` § Critical).

### Batch B — Messaging send parity (P0)
- Proto + handlers: `schedule_message`, `send_silent`, `send_when_online`; worker skeleton.
- `validateRichPayloadAttachments` for `video_note`/`music`; sticker/gif file path rules.
- Align `ForwardMessage` with `SendMessage` guards (slow-mode, moderation, attachments).
- File `file.processed` → list metadata refresh consumer.

### Batch C — Space owner & tree pin (P0)
- Migration `is_pinned`/`pin_order`; `PinTreeNode`/`UnpinTreeNode`; Gateway routes.
- `DeleteSpace`, `TransferOwnership`, `GetAuditLog` handlers (or explicit defer in PLAN).
- `entry_requirement` queue/captcha MVP or narrow spec.

### Batch D — Client auth & social completeness (P1)
- Password-reset screen → existing REST.
- Phone contact hash pipeline + platform permission for `syncPhoneContacts` (replace `[]` stub).
- QR live scanner (`mobile_scanner` or defer l10n).
- Restore-account UI (optional link from delete success).

### Batch E — R2-A04 mobile shell finish (P1)
- Stacked chrome layout rules (strip + pinned collapse + composer + tabs) with overflow tests.
- Promote `MobileShellDrawer` from stub: profile stack, presence entry, match settings parity.
- Narrow E2E: `chat_info_panel` notification tile.

### Batch F — Stickers/GIF vertical slice (P1, design-deferred)
- Minimal sticker pack API or file-backed send path.
- Flutter composer tabs (emoji | stickers | GIF) — can ship without Penpot if spec-locked.
- Penpot R4-04-L04 frames in parallel track (human/design).

### Batch G — Bot & moderation hardening (P1)
- Fix `lookupInteraction` chat type from stored payload.
- Notification consumer: account→primary profile resolution + push copy.
- `appeal_sheet` widget tests + compose appeal E2E.
- Bot token rotation → hub purge.

### Batch H — Voice roles & admin analytics (P1)
- Enforce `VOICE_SPEAK` / `VOICE_MUTE_OTHERS` on speak/mute RPCs.
- Analytics `query.go`: search + voice dashboard types; Grafana panels.
- Search indexer: `Nak` on failure; chat rename/tree handlers.

### Batch I — CI & prod hardening (P2, human-heavy)
- Document prod deploy gaps (selective, stack.lock, smoke parity).
- Optional: tier-2 smoke as required check for `src/frontend/` + gateway paths.
- Secrets checklist → `ci.md` Critical (not README-only).

### Batch J — Docs & test debt cleanup (P2)
- Prune stale `partial`/`done` bullets in `backend.md` (shadow ban, social.events, GuestConvert NATS, E2E smoke list).
- Add `developer-portal/` to `PLAN.md`.
- Appeal widget tests, `profile_context_controller` tests, cross-feature E2E smoke entries.

---

## Batch-theme crosswalk (1–29)

| Recurring batch theme | Weakness left open |
|----------------------|-------------------|
| Penpot-deferred UI | Stickers/GIF, composer attach matrix, mobile · v3 chrome, desktop Room CTAs |
| R2-A04 remainder | Drawer stub label, stacked overflow, rail parity on mobile |
| Stickers/GIF | Backend enum only; client + Penpot both open |
| Billing | Test checkout URL; JWT tier; history empty; CloudPayments missing |
| Space tree | Reorder REST shipped; **pin** not started |
| CI secrets | Blocks staging validation of social/billing/notifications shipped in batches 23–29 |
| Social batches 23–26 | REST + UI tabs shipped; phone sync + camera QR are intentional stubs |
| Chat folders 17–21 | Backend + desktop rail strong; mobile drawer partial |
| Messaging 11–14, 27 | Delivery ack + content_type incremental; schedule/forward/read-state gaps |
| Auth 28–29 | Delete + sessions shipped; reset/restore/phone OTP open |
| Moderation 27 | Appeal form shipped; notification delivery + widget tests weak |

---

*Analyst 3 — weakness & gap hunter. Analysis only; `docs/todo/` not modified.*
