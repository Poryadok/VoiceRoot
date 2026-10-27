# Subscription benefit matrix v1

## Purpose and version

`voice.subscription-offer.v1` freezes the recorded A7 offer inventory at source
baseline `f40a7ddf5ad1f9d22441635773f7abdf2e7baaf2`. The machine-readable
[offer manifest](subscription-offer-v1.json) and this matrix are one contract.
Checkout and plan management consume the same product and benefit IDs; they must
not advertise a subset as the complete offer. This is a documentation contract,
not shipped enforcement, a runtime capability response, or live billing approval.
Every row below still requires server/client E2E evidence before paid activation.

Do not change values, scope, benefit IDs, or exclusions silently under v1. A
successor offer version must link the canon decision and the changed rows;
consumer implementation/evidence can advance without changing this inventory.
An unresolved implementation seam is not permission to omit its benefit. No
benefit is excluded by this document. Story rows retain their explicit future
scope and require a canon scope decision before inclusion in an A7 checkout.

## Shared rules

Premium is account-owned and applies across all its profiles; Space Pro is
Space-owned and bought by its owner. Space Pro grants no personal Premium
benefits. Both cost $5/month; annual subscription has a 20% discount, no trial,
and the documented no-refund policy. Additional clean usernames are separate
purchases, not extra included Premium names. Provider/market selection remains
[subscription.md](subscription.md#платёжные-провайдеры); no provider is activated here.

`ACTIVE` and `GRACE_PERIOD` grant the same benefits strictly before
`entitled_until`. Scheduled cancellation stays `ACTIVE` until paid period end.
The [accepted lifecycle contract](../testing/subscription-lifecycle-convergence-exec-plan.md)
owns trusted claims, revisions, boundary resolution, deletion/restore and replay;
none of the rows permits client-supplied tier authority. Numeric/bool GetLimits
keys below refer to the [existing JSON projection contract](../microservices/subscription-service.md#getlimits-json-benefit-projection).
An ID without that key is an inventory identifier, not a new API field.

## Personal benefit inventory

Sources: **S** = [Subscription cosmetics and functional improvements](subscription.md#косметика-профиля),
**U** = [User profile](user-profile.md#настройки-профиля-редактирует-пользователь),
**M** = [Multi-profile monetization](multi-profile.md#монетизация),
**T** = [Text chat social mechanics](text-chat.md#социальные-механики),
**F** = [File limits](file-storage.md#лимиты).
Consumer RPC names link to the service API contracts:
[User](../microservices/user-service.md), [Chat](../microservices/chat-service.md),
[Messaging](../microservices/messaging-service.md), [File](../microservices/file-service.md),
[Voice](../microservices/voice-service.md), [Space](../microservices/space-service.md),
[Notification](../microservices/notification-service.md),
[Auth](../microservices/auth-service.md) and [Story](../microservices/story-service.md).
Each scenario is a required future proof, not a PASS. “Gap” names missing
consumer semantics/API support that an implementation slice must resolve from
canon; it never invents a wire field or asset catalog.

| Benefit ID / GetLimits key where present | Free → Premium/grace; source | Server consumer and existing contract surface | Flutter consumer and required E2E |
|---|---|---|---|
| `premium_badge` | unavailable → ★ by name in all chats; S | User profile presentation; `GetProfile` / `GetProfiles`; extended paid presentation gap | Chat/profile renders ★ for entitled profile across account profiles; free has none; refreshed inactive state removes paid display |
| `animated_avatar` | static only → GIF avatar; S, U | User `UpdateProfile`, File upload/processed media; paid/avatar wire support gap | Profile edit → save GIF → second client sees animation; free direct write denied; test A6 media readiness |
| `profile_banner` | unavailable → background image; S, U | User `UpdateProfile` / `GetProfile`, File bytes; extended paid field gap | Save banner → another client opens profile; free write denied |
| `custom_status` | unavailable → custom status; S, U | User `UpdateProfile` / `GetProfile`; durable metadata, distinct from Redis presence | Edit → reopen/second client shows status; free bypass denied |
| `emoji_status` | unavailable → emoji status; S | User profile metadata; extended paid field/API gap | Set status → second client displays same emoji; free bypass denied |
| `premium_username_count` | 0 → 1 per account; S, M | User username validation/update; allocation of included name to a profile remains unspecified | One included clean name across profiles; reject a second included name; separate-purchase UI must not imply it is included |
| `exclusive_stickers` | unavailable → access; S | Chat `InstallStickerPack`; File assets, Messaging `SendMessage`; A6 content dependency | Catalog → entitled install → send → peer sees sticker; free install denied; no fabricated exclusive catalog |
| `exclusive_emoji` | unavailable → access; S | Chat/Messaging emoji selection/use; exclusive catalog, ownership and enforcement surface gap | Entitled selection/use → peer renders; direct free use denied once canonical catalog/use contract exists |
| `custom_app_icons` | unavailable → custom app icons; S | User `GetSettings` / `UpdateSettings` where synced; paid setting/platform support gap; native app owns applied icon | Entitled settings selection applies on a supported platform; free selection denied; platform support must be explicit |
| `exclusive_chat_themes` | unavailable → exclusive themes; S | User settings and Chat presentation; catalog, persistence scope and paid selection gap | Choose theme → reopen target chat; free cannot select paid theme; define persistence from canon before implementation |
| `message_reaction_count` | 1 → 3 per user per message; S, T | Messaging `AddReaction` / `RemoveReaction` | Free second / paid fourth distinct emoji prompts limit; paid three visible to peer; remove/re-add and duplicate retries do not consume extra slots |
| `file_download_priority` | ordinary → CDN priority; S | File `GetFileURL` and download/CDN path; priority signaling/integration gap, no throughput guarantee | Download through actual priority path for paid vs free; rendered download works in both; show no invented speed promise |
| `file_upload_bytes` | 52428800 → 209715200 bytes (documented 50/200 MB labels); S, F | File `RequestUpload` / `ConfirmUpload` / `CheckQuota`; GetLimits existing wire sizes | Client rejects oversized file before upload; direct server request also rejects; exact boundary accepted; upload → message → peer download/hash |
| `joined_spaces_and_groups` | 100 → 1000 Spaces + group chats combined; S | Space `JoinByInvite` and Chat `CreateChat` / `AddMembers`; combined count authority, concurrency and scope across profiles remain a contract gap | Join/create through both surfaces at combined cap; concurrent bypass denied; UI explains limit; no DM-only or Spaces-only counter substitution |
| `stream_quality` | 360p/30fps → 720p/30fps; S, [Screen share](screen-share.md#качество-и-адаптация) | Voice `StartScreenShare` / `GetJoinToken`; Auth trusted personal claim; media quality enforcement seam | Start stream with allowed setting → peer receives within cap; direct free paid-quality request denied; adaptive lower quality remains valid |
| `profile_count` | 2 → 5; S, M | User `CreateProfile` / `ListMyProfiles`; account scope | Create up to cap across profiles; next create fails on server and client; concurrent create cannot exceed cap |
| `file_retention_days` | P90D → unlimited live non-E2E references; F | File exact `retention_account_id` references, lifecycle inbox/recalculation/GC | Free expiry placeholder; paid reference remains; downgrade gives live reference effective-time + P90D; renewal cannot resurrect expired data; forwarding keeps separate owner policy |
| `e2e_file_retention_days` | P90D → P90D in every tier; F | Same File reference/GC path; E2E exception | Paid E2E still expires; client never promises unlimited E2E storage; no cross-reference blob deletion while another live reference exists |
| `profile_downgrade` | free pair → paid 5 → free pair; [Multi-profile downgrade](multi-profile.md#downgrade-и-renewal-accepted-a7-target) | User downgrade-cycle selection and entitlement inbox, Auth switch eligibility; accepted lifecycle target | Cancel/grace chooses primary + eligible secondary; no early freeze; inactive applies saved/fallback pair offline; renewal clears only subscription overlay; stale choice cannot refreeze |
| `personal_grace_reminders` | no failed-payment grace → D1/D3/D7 within 7-day grace; [Notifications](notifications.md) | Subscription scheduler → `subscription.grace_reminder` → Notification inbox/channel outbox; primary-profile resolution | Payer primary sees one logical in-app reminder and applicable push; replay collapses; recovery/expiry cancels undelivered reminders; ordinary routing/quiet hours apply; no burst of missed windows |

## Space Pro inventory

Source for all limits/customization: [Subscription Space Pro](subscription.md#space-pro--5мес-на-спейс-платит-владелец),
[Spaces limits](spaces.md#лимиты-участников), [Voice limits](voice-chat.md#лимиты).
Space lifecycle/reminders follow the same accepted revision/boundary contract as
personal subscription; the payer's Premium state cannot replace Space Pro.

| Benefit ID / GetLimits key where present | Free → Space Pro/grace | Server consumer and existing contract surface | Flutter consumer and required E2E |
|---|---|---|---|
| `space_member_count` | 50 → 5000 | Space `JoinByInvite` / membership admission | Actual member join at cap succeeds, next denied; other client's roster/count updates; paid payer alone does not uplift another free Space |
| `voice_room_participants` | 32 → 128 simultaneous per room | Voice `JoinVoiceRoom` / `GetJoinToken`, Space entitlement projection | Actual concurrent admissions at room cap; next denied; trusted expiry boundary removes paid uplift; unrelated rooms remain independent |
| `space_tree_nodes` | 50 → 500 text + voice combined | Space `UpsertTreeNode` / `CreateVoiceRoom` and tree persistence | Mixed text/voice creation reaches cap; next denied; all applicable creation paths share enforcement; category-count semantics remain a gap rather than inferred |
| `space_banner` | unavailable → banner | Space `UpdateSpace` / `GetSpace`, File image bytes; banner paid-field gap | Owner saves banner → another member sees it; free direct mutation denied |
| `space_custom_emoji` | unavailable → custom emoji; numeric quota unspecified | Space customization and Chat/Messaging use; canonical upload/catalog/use surfaces gap | Owner installs custom emoji → member sends → peer renders; unauthorized/free mutation denied; A6 asset rules apply |
| `space_downgrade` | paid caps → free caps after final expiry | Space/Voice entitlement projections and membership/resource growth decisions | Grace retains paid limits; expiry never kicks/deletes existing members/resources; new growth denied while free cap exceeded; recovery permits paid growth again |
| `space_grace_reminders` | D1/D3/D7 for purchaser, same 7-day grace | Subscription scheduler, Notification channel outbox, User authoritative primary resolution | Purchaser primary sees Space context, deduped reminders; another member gets none; recovery/expiry/deleted payer cancels remaining sends |

## Recorded scope decisions and unresolved seams

[Stories monetization (future)](stories.md#монетизация-будущее) records these
additional subscription ideas. [PLAN A7](../PLAN.md#a7--provider-ready-monetization)
places the full stories editor in a later planning cycle. They are retained
here so future scope decisions cannot silently remove recorded benefits; this
inventory does not move them into the current checkout or mark them excluded.

| Recorded ID | Explicit behavior / missing decision | Consumer / required future server-client proof |
|---|---|---|
| `story_anonymous_view` | Name absent from viewers, total count increments; A7 inclusion conflicts with future feature placement, needs explicit scope decision | Story `MarkViewed(anonymous)` / `GetViewers`, Flutter viewer: paid anonymous view hides name but increments count; free bypass denied |
| `story_extended_effects` | Extended filters/effects; paid catalog unspecified; future editor scope | Story/File media ingest + Flutter editor; catalog and enforceable API need definition before picker → publish → viewer proof |
| `story_extended_video` | Paid video longer than 60 seconds; paid maximum unspecified; future editor scope | File processing + Story creation + Flutter capture/upload; define maximum before boundary, bypass and playback proof |
| `story_custom_sticker_packs` | Subscription **or** separate purchase; exact product allocation unspecified | Story editor/media + Chat pack contract where applicable; define allocation/use surface before install/select → publish → viewer proof |

The open [backend contract gaps](../todo/backend.md#subscription) also cover
membership aggregation scope/authority, included username allocation, cosmetic
catalogs and persistence, priority signaling, Space emoji surfaces and ownership
transfer billing semantics. Do not invent those contracts from the boolean
GetLimits projection. Core sticker catalog optionality in Text chat does not
remove the exclusive-sticker promise from the A7 offer; A6 asset readiness and
paid enforcement remain required inputs.

## Implementation evidence and review handoff

For every inventory row, a consuming implementation must record an immutable
server/client test reference and result for its entitled and free/expired paths,
same source revision, consumer/API and offer version. Include grace equivalence,
cancel-at-period-end, boundary expiry and redelivery/restart where state is
projected. Existing tests for GetLimits serialization alone do not satisfy this.
The current status of **all rows is evidence pending**; this contract publishes
no E2E PASS. Billing-disabled regression must keep the free product usable.

Review consumers: User/Auth for personal identity/eligibility and claims;
Chat/Messaging for packs/reactions/membership; File for upload/priority/retention;
Space/Voice for scoped limits; Notification for reminders; Flutter and checkout
for complete offer rendering and each observable flow. Independent review must
check source anchors, missing rows, no implicit exclusions, and no fabricated
APIs/values. Live providers, merchant setup, deployments and acceptance remain
outside this contract-documentation slice.
