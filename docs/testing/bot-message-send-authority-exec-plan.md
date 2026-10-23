# Protected send-only Bot message authority — PR875

## Contract and scope

The hosted slash regression reaches message publication but ordinary Messaging
membership requires Chat visibility. A native installed Bot with only declared
`TEXT_CHAT_SEND_MESSAGES` has no `TEXT_CHAT_VIEW` grant. This repair supplies a
separate send-only authority; it grants no Role mask, history or member-list read.
DM behavior and ordinary SendMessage authorization remain unchanged.

Messaging.SendBotMessage accepts the existing message body plus a protocol1
binding: canonical nonzero bot, actor, Space, installation, installation operation
and Chat UUIDs, and positive signed-int64-compatible installation generation.
The exact wrapper is authenticated as service:bot with mTLS, method/body/request-ID
binding and replay prevention. Sender identity comes only from this verified tuple.
The nested message Chat must equal the binding Chat. Delayed/scheduled sends,
send-when-online and sender-changing posted-as-chat are rejected on this route;
the five-second authorization cannot be retained for later publication.

Messaging independently obtains Bot.CheckBotMessageSendAuthority from the exact
service:messaging route. Bot echoes the complete requested tuple and permits only
the current completed installation/head, live Bot, declared send scope, enabled
exact Space/chat whitelist and LIVE Space lifecycle. Missing or revoked authority
denies; storage/transport failures remain unavailable.

Space.GetMemberAdmissionAuthority adds an optional bot_message_send_binding.
Only the exact Messaging branch may use it, with matching Space/profile/chat and
without ordinary expected-generation/admission-operation fields. Space echoes it
only after validating the current native Bot installation head, publication,
eligible membership, LIVE lifecycle/available ownership and current Chat tree
relation. Role's existing nil-binding branch is unchanged. Missing echo cannot
qualify a send. Installation generation is distinct from membership generation.

All callbacks and the final Messaging write share the original monotonic maximum
five-second observation budget, started before callback dispatch; no callback or
lock wait resets it. Messaging retains content, moderation, communication, thread,
attachment and idempotency checks. A send admitted before a concurrent cancellation may complete within the original
five-second budget. Fresh observations and cancellation fences deny subsequent
attempts; this is bounded overlap, not a distributed hard post-ACK guarantee.
Static token expiry alone is not revocation authority. Reinstall does not revive an old tuple.
The producer acquires the existing shared Space guard, then Bot and whitelist
rows FOR SHARE in that order, and holds them through the actual bounded Messaging
RPC. Disable and uninstall serialize through their existing conflicting guards.
The authority callback uses a consistent plain MVCC read and must not reacquire
those guards, avoiding a cross-service lock cycle. The deadline includes initial
guard acquisition and the entire RPC; no transaction survives that bounded call.

## Owners and verification

- Bot author WT35: wire/generated types, producer, current Bot callback, tests and docs.
- Space author WT47: exact Space observation branch and shared receiver verifier.
- Messaging author WT49: protected receiver, independent observations, typed
  internal authorization, persistence/cancellation fence, scoped tests and docs.
- Root: integration/configuration; f13: independent protocol/source review.
- Same authors write tests and implementation. RED/GREEN must cover send-only
  success and disabled/revoked/reinstalled/foreign-chat/unknown-owner denials.
  Native Space PostgreSQL evidence and receiver standalone TLS tests are required.
  No full local Compose/Auth/Chat receiver-floor scenario is authorized.

## Progress

- [x] Source diagnosis and protocol boundary agreed with root.
- [x] Publish additive wire and generated types for the receiver consumer.
- [x] Bot producer/current authority RED/GREEN; Space exact observation owned WT47.
- [ ] Messaging receiver and cancellation-fence RED/GREEN (WT49).
- [ ] Independent review, scoped verification and integration.

Bot checkpoint: native PostgreSQL producer/disable/uninstall/current tuple and
single-connection saturation regressions pass. Exact callback mTLS/JWKS recovery,
replay/foreign URI, deadline, short/race/vet/lint and image build pass. The whole
grpcsvc run exhausted its aggregate eight-minute harness budget during another
PostgreSQL startup without prior assertion failures; root approved disjoint complete
test-inventory cohorts with unchanged limits. Integration remains root-owned.

## Operator wiring

Activate this route only in the full and phase0 Bot topology. Messaging uses the
dedicated messaging-bot-authority-client.crt/key leaf signed by the Bot lifecycle
client CA for Bot port9444; its exact URI is
spiffe://voice.local/phase0/messaging. This does not widen the lifecycle CA trust
set. The Bot JWKS endpoint continues using the ordinary Phase0 client trust.
Bot sends through the dedicated Messaging port9095. Messaging obtains Space
authority on port9099 and communication decisions on port9101. All five optional
BOT_MESSAGE_AUTHORITY_MESSAGING_* settings must be supplied together; missing
authority cannot enable sends. Initial Messaging JWKS retrieval is lazy to avoid
a startup cycle; every request still requires verified keys, TLS identity and replay
protection.
