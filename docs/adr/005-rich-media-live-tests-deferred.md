# ADR 005: Rich-media live tests deferred (stickers / GIF / voice messages / voice recording)

## Status

Accepted (2026-08-17); **amended 2026-08-24** (stickers/GIF **in v1 product scope**; lives still blocked on wire contracts + code).

## Context

[integration-tests-gap-plan.md](../testing/integration-tests-gap-plan.md) **P3.5** targets compose/Flutter lives for inventory IDs **TC-MSG-09** (stickers/GIF), **TC-MSG-10** (voice messages), **VC-10** (voice-session recording). Product plan Wave 3 required a **product scope ADR first** before writing those lives.

What the feature docs say:

| Surface | Spec | Documented behavior |
|---------|------|---------------------|
| Stickers | [text-chat.md](../features/text-chat.md) §Медиа | **v1 confirmed (2026-08-24):** system packs + user-uploaded packs; send/receive first-class; composer picker. Still **no** pack API / storage / message wire shape in code — implementation = [PLAN.md](../PLAN.md) phase **2** |
| GIF in chat | same | First-class chat GIF (not only file attach); distinct from Premium **GIF avatar** in [user-profile.md](../features/user-profile.md) / [subscription.md](../features/subscription.md). Provider-neutral wire is defined in Messaging/Chat; live provider selection remains an A6 activation decision |
| Voice messages | same + [privacy.md](../features/privacy.md) `allow_voice_messages` | «аудиофайл + встроенный плеер»; privacy gate exists; Messaging docs list attachment kind `voice_message` — **no** client record/send/player product path or live DoD |
| Voice recording | [voice-chat.md](../features/voice-chat.md) §Запись | **Local-only** on the initiator’s device (MP3 128 kbps); server does not store; indicator only for recorder |

Product decision: stickers/GIF **stay in v1** ([product-roadmap.md](../todo/product-roadmap.md)). [design.md](../todo/design.md) still lists composer GIF / Stickers / Voice message as missing UI. Inventing wire contracts only for green lives would violate [TESTING.md](../TESTING.md) TDD — expand feature/microservice docs in the implementation PR, then add lives.

## Decision

1. **Product scope:** stickers + chat GIF are **in v1** (system + user packs). Do **not** strike them from text-chat.
2. **Do not add** Gateway compose or Flutter `*_e2e_live_test` stubs for TC-MSG-09, TC-MSG-10, or VC-10 until pack/send contracts exist in `docs/` **and** enough code to assert.
3. **v1 live-test scope (when unblocked)** — only what specs then define; minimum expected tracks once features land:
   - **TC-MSG-09:** send/receive a sticker **or** GIF as a first-class chat message path (not generic file attach alone), per expanded text-chat (+ any pack/catalog docs).
   - **TC-MSG-10:** send voice note as audio attachment with `voice_message` semantics + privacy deny via `allow_voice_messages` where specified.
   - **VC-10:** **client/device** smoke that local recording starts/stops and produces a local MP3; **not** a Gateway/server compose live (no server persistence in current voice-chat.md).
4. Gap-plan **P3.5** and inventory IDs stay **deferred / blocked on implementation contracts** (not on «do we want stickers?») with a link to this ADR.

## Consequences

- Soft-launch / smoke CI does **not** gate on sticker/GIF/voice-note/recording lives until contracts + code exist.
- Implementation unlocks lives: expand Messaging/Chat/File docs in the feature PR → green TC-MSG-09; inventory flips to `[exists]` / `[partial]`.
- Generic File upload / shared-media lives remain separate and must not be relabeled as TC-MSG-09/10.
