# Todo Audit — Consolidated Findings

**Date:** 2026-09-02  
**Inputs:** `analyst-1-completed-inventory.md`, `analyst-2-correctness-review.md`, `analyst-3-weaknesses-gaps.md`  
**Verifier:** consolidator (code/docs spot-check on current workspace tree)

---

## Summary

| Metric | Count |
|--------|------:|
| Raw findings across 3 analysts (incl. duplicates) | ~95 |
| **Deduplicated themes** (unique issues after merge) | **42** |
| Verified & actionable (todo updated or already tracked) | **28** |
| Rejected / downgraded (false positive or overstated) | **8** |
| Needs-human (secrets, legal, design defer) | **6** |
| Doc-only hygiene (mass `[x]` retention policy) | deferred |

**Headline:** Batch loop (#74–#114) delivered real, mergeable work. Main doc debt is **stale Critical/High bullets** (shadow-ban, mm_ban S2S, moderation consumer, `message.forwarded`) and **index drift** (`docs/TODO.md` §Топ дыр). One **must-fix product gap** verified: `ForwardMessage` bypasses shadow-ban / `ghost_only` (send path fixed). Deploy gap: `MODERATION_GRPC_ADDR` missing in k8s configmaps.

---

## Classification (deduplicated)

### Must-fix — spec violation / bug

| ID | Theme | Evidence | Todo action |
|----|-------|----------|-------------|
| M1 | `ForwardMessage` no `IsShadowBanned` / `ghost_only` | `SendMessage` sets `ghostOnly` + suppresses publish; `ForwardMessage` ~1107–1177 publishes `message.sent` without shadow check; `insertForwardCommentary` same | Updated `backend.md` § High Messaging forward guards |
| M2 | `insertForwardCommentary` no moderation/shadow gate | `messaging_grpc.go` ~1182–1206 | Same bullet |
| M3 | `MODERATION_GRPC_ADDR` absent staging/prod k8s | In `docker-compose.yml`; not in `deploy/` grep | Already in `backend.md` § Critical Messaging (verified) |
| M4 | Billing checkout stub + CloudPayments `Unimplemented` | Known P0 | Already tracked `backend.md` § Critical Subscription |
| M5 | JWT `subscription_tier=free` without `AUTH_NATS_URL` | Auth beans | Already tracked + `ci.md` |
| M6 | Space `DeleteSpace` / `TransferOwnership` / audit `Unimplemented` | space grpcsvc | Already tracked |
| M7 | Space tree pin RPCs missing | migration gap | Already tracked P0 |
| M8 | Bot `lookupInteraction` → always `CHAT_TYPE_CHANNEL` | `interaction.go:541` | Already tracked `backend.md` § High Bot |
| M9 | Delete-account tombstone incomplete (DM label, ListChats hide) | auth + chat | Already tracked |

### Should-fix — quality / partial slices

| ID | Theme | Todo action |
|----|-------|-------------|
| S1 | Moderation sanction notifications: consumer acks, empty `recipientProfileID` | Updated Critical + High Moderation bullets |
| S2 | Phone sync UI stub (`syncPhoneContacts(const [])`) | Added `client.md` open item |
| S3 | QR add-friend paste-only (no camera) | Added `client.md` open item |
| S4 | R2-A04 mobile shell remainder (drawer stub, stacked chrome) | Already `client.md` |
| S5 | Stickers/GIF zero Flutter composer | Already tracked |
| S6 | `VOICE_SPEAK` / `VOICE_MUTE_OTHERS` not on speak/mute | Already `backend.md` § Role |
| S7 | Analytics admin search/voice dashboard types | Already `admin.md` |
| S8 | Matchmaking platform + peer ban fail-closed (#73) | Added `backend.md` § Matchmaking |
| S9 | Forward path vs send: attachment validate gaps | Merged into M1/M2 bullet |
| S10 | DM inbox bucket when `SOCIAL_GRPC_ADDR` unset | Already implicit in Social wiring; not new |
| S11 | Password-reset screen missing | Already `client.md` |
| S12 | Restore-account UI missing | Note on delete-account item |
| S13 | Appeal sheet widget test gap | Low priority; not added (test debt) |

### Nice-to-have / polish

- Penpot · v3 deferred frames; idle presence client-only; VoiceListSkeleton gaps; prod selective deploy; distributed tracing; `subscription.events` bus — already in todo or P2 in analyst-3.

### Already tracked (no new todo line)

- Folders / Quick Access / archive RPCs + UI (batches 15–21)
- Message requests R3-A04, notifications `message_request`
- Appeals Gateway + Flutter (27a); appeals **business rules implemented** in `appeals.go`
- Sessions/revoke UI (29b); delete-account UI (28b)
- Pin limit **5** (`MaxPinsPerChat`, PR #70)
- Social friend-invite fail-closed (#72)
- Delivery ack publisher + Messaging consumer (batches 11–12)
- `message.forwarded` NATS publish exists (`PublishMessageForwarded`)

### Needs-human

| Blocker | Owner |
|---------|--------|
| Staging secrets (FCM, Paddle, `AUTH_NATS_URL`, TOTP, Resend) | **Вы** — `ci.md` § Critical |
| Prod DNS / AASA / assetlinks placeholders | **Вы** |
| Appeals policy edge cases beyond code validation | product/legal |
| Penpot · v3 registry + stickers/GIF frames | design |
| `git pull` / hook-blocked history on audit machine | ops — re-verify HEAD vs `master` |
| Tier-2 E2E PR-gating policy change | team policy |

### Rejected / downgraded (why)

| Analyst claim | Verdict | Reason |
|---------------|---------|--------|
| Shadow-ban "не режет fanout" (blanket) | **Downgraded** | `SendMessage` sets `ghost_only`, suppresses `message.sent`; gap is **forward path only** + k8s `MODERATION_GRPC_ADDR` |
| `social.events` zero publisher | **Rejected** | `social/main.go` wires `JetStreamPublisher`; friend events publish |
| Appeals lack business rules | **Rejected** | `appeals.go`: ownership, 7-day window, `AlreadyExists` |
| `mm_ban` sanction local-only | **Rejected** | `sanctions.go` calls `ApplyPlatformMMBan` / `RevokePlatformMMBan` |
| `message.forwarded` NATS missing | **Rejected** | `PublishMessageForwarded` in forward handler |
| `GuestConvertNatsEventIntegrationTest` false positive | **Rejected** | Test calls `convert-guest`, asserts subject |
| E2E smoke "omits ws_resume/delivery/notifications" | **Downgraded** | Tests listed in `e2e-features.yml`; issue is **tier-2 not PR-gated**, not omission |
| П.15 bots v2 fully shipped `[x]` | **Downgraded** | Slash picker shipped; inbound webhook, Portal CSRF, full deferred platform — partial |
| Forward skips slow-mode | **Rejected** | `ForwardMessage` calls `Moderation.EnsureCanSend` (slow-mode + timeout) |
| Developer Portal batch 6 "missing from docs" | **Downgraded** | Covered in `admin.md` vitest bullets; PLAN map gap noted as P2 |

---

## Changes applied to `docs/todo/`

| File | Change |
|------|--------|
| `docs/TODO.md` | §Топ дыр: removed stale folders/game-editor/sessions/delete-account rows; refreshed blockers |
| `docs/todo/backend.md` | Shadow-ban partial truth; moderation consumer delivery stub; mm_ban S2S wired; `message.forwarded` done; forward shadow-ban gap; MM platform-ban fail-closed (#73) |
| `docs/todo/client.md` | Open items: phone sync real pipeline, QR camera scanner |
| `docs/todo/product-roadmap.md` | П.15 downgraded from `[x]` to partial |

**Not changed (by policy / scope):** `docs/PLAN.md` (index says do not edit); mass deletion of ~58 `[x]` batch items (hygiene sweep deferred).

---

## Remaining human-only blockers

1. Staging/prod secrets and `AUTH_NATS_URL` for real JWT tier + billing + push  
2. Alertmanager / observability staging DoD  
3. Penpot · v3 + stickers/GIF design frames before full UI parity  
4. Appeals product/legal beyond implemented 7-day/duplicate rules  
5. Confirm workspace HEAD matches `master` (local `git` hook blocked pull during audit)  
6. Policy: tier-2 E2E as required PR check vs master-only nightly  

---

## Git / PR note

Branch `audit/todo-consolidation` + commit/PR requested but **local Cursor hook blocked `git` commands** in consolidator environment. Apply commit manually:

```powershell
git checkout -b audit/todo-consolidation
git add docs/TODO.md docs/todo/*.md tmp/todo-audit/consolidated-findings.md
git commit -m "docs(todo): consolidate batch-loop audit findings"
git push -u origin audit/todo-consolidation
gh pr create --title "docs(todo): batch-loop audit consolidation" --body "..."
```

Merge only (no rebase) when CI green.
