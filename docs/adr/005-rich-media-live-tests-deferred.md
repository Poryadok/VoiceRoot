# ADR 005: Rich-media live tests deferred (stickers / GIF / voice messages / voice recording)

## Status

Accepted (2026-08-17)

## Context

[integration-tests-gap-plan.md](../testing/integration-tests-gap-plan.md) **P3.5** targets compose/Flutter lives for inventory IDs **TC-MSG-09** (stickers/GIF), **TC-MSG-10** (voice messages), **VC-10** (voice-session recording). Product plan Wave 3 required a **product scope ADR first** before writing those lives.

What the feature docs actually say today:

| Surface | Spec | Documented behavior |
|---------|------|---------------------|
| Stickers | [text-chat.md](../features/text-chat.md) §Медиа | One line: system packs + user-uploaded packs — **no** pack API, storage, premium rules, or message wire shape |
| GIF in chat | same | One word: «да» — **no** provider, search, or send contract (distinct from Premium **GIF avatar** in [user-profile.md](../features/user-profile.md) / [subscription.md](../features/subscription.md)) |
| Voice messages | same + [privacy.md](../features/privacy.md) `allow_voice_messages` | «аудиофайл + встроенный плеер»; privacy gate exists; Messaging docs list attachment kind `voice_message` — **no** client record/send/player product path or live DoD |
| Voice recording | [voice-chat.md](../features/voice-chat.md) §Запись | **Local-only** on the initiator’s device (MP3 128 kbps); server does not store; indicator only for recorder |

[product-roadmap.md](../todo/product-roadmap.md) has **no** initiative for chat stickers/GIF/voice notes. [design.md](../todo/design.md) still lists composer GIF / Stickers / Voice message as missing UI entry points. Inventing contracts for green lives would violate [TESTING.md](../TESTING.md) TDD / «не изобретать поведение вне features».

## Decision

1. **Do not add** Gateway compose or Flutter `*_e2e_live_test` stubs for TC-MSG-09, TC-MSG-10, or VC-10 until product scope is written into `docs/features/` (and shipped enough to assert).
2. **v1 live-test scope (when unblocked)** — only what specs then define; minimum expected tracks once features land:
   - **TC-MSG-09:** send/receive a sticker **or** GIF as a first-class chat message path (not generic file attach alone), per expanded text-chat (+ any pack/catalog docs).
   - **TC-MSG-10:** send voice note as audio attachment with `voice_message` semantics + privacy deny via `allow_voice_messages` where specified.
   - **VC-10:** **client/device** smoke that local recording starts/stops and produces a local MP3; **not** a Gateway/server compose live (no server persistence in current voice-chat.md).
3. Gap-plan **P3.5** and inventory IDs stay **deferred / blocked on product** with a link to this ADR until features expand.

## Consequences

- Soft-launch / smoke CI does **not** gate on sticker/GIF/voice-note/recording lives.
- Expanding text-chat (or a dedicated feature doc) + design/UI + optional roadmap item is the unlock; then a follow-up PR adds real lives and flips inventory to `[exists]` / `[partial]`.
- Generic File upload / shared-media lives remain separate (already covered where applicable) and must not be relabeled as TC-MSG-09/10.
