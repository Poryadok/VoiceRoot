# Bot native installation intent

## Purpose and context

Finish the already started Bot producer boundary consumed by Space native kind6.
Bot must persist an installation identity before any remote membership add;
retries, cancellation and reinstall must preserve that exact identity.
Sources: bots.md install/removal, bot-service.md, DATA_MODEL.md Space membership,
ARCHITECTURE_REQUIREMENTS.md service principals, BE-170 c3/c4 and the explicit
implementation-before-merge continuation. This does not complete global DeleteBot.

Producer247 owns Bot persistence/handlers, additive Space request wire and its
generated mirrors. Receiver186 owns Space protected handler/native publication,
generation fence and terminal receipts. Base is master candidate78bc80b9 plus
the frozen wire union4f620ea0 (including183 DTO2944), then normal master
candidate5113a511 merge. Latest notified master target is fd43a284; consume its
coherent head before final handoff, not old develop874.

## Scope and contract

AddBotMember carries nested BotInstallationIntent: operation_id, bot_id,
installation_id, space_id, actor_profile_id and positive int64-range generation.
RemoveBotMember additionally binds install_operation_id. Top-level Space/profile
must equal nested identity. Bot signs service:bot, space audience, exact method,
operation ID and canonical full-request hash; each retry obtains a fresh JTI.
No raw metadata or configurable human default Role authorizes this operation.

The persisted producer states are PREPARING, INSTALLED, CANCELING, CANCELED.
One active intent per Bot/Space; Space advisory guard precedes the Bot row lock.
An uncertain add retains its immutable intent and key. Local activation follows
remote acknowledgement only. Cancellation persists its own stable key before
RPC, retains the original add identity, and blocks reinstall until acknowledged.
Reinstall allocates a new installation ID and the next Bot generation. Space's
own ordinary anti-ABA generation is a separate authority.

## Milestones and steps

- [x] Real PostgreSQL regression proves old handler sends before durable intent.
- [x] Freeze additive wire402e878f with186/183/69; generate from current union.
- [x] Add migration008, durable prepare/complete/cancel store transitions and tests.
- [x] Consume protected Space add/remove with exact signed requests and deadlines.
- [x] Wire a distinct mTLS installation client; no fallback to the raw client.
- [ ] Execute persistence/restart/duplicate/cancel/reinstall positives and negatives.
- [ ] Consume real186 receiver for late-add/cancel fence proof, review and121 handoff.

## Validation

Own Linux Docker only, native source/cache, private network and no host ports.
Real PG tests inspect the committed state at the RPC boundary and after restart.
Negative cases include changed pending payload, disabled Bot/frozen Space,
cancel-before/while-add, lost response, stale completion, duplicate cancel and
generation reuse. New migration DOWN must lock/check before dropping retained
intent authority; a two-connection test must discriminate the first writer race.
Run affected Bot tests/race/vet and wire gates. Keep whole DeleteBot, historical
pin coverage, shared rollout and acceptance open.

## Decisions, progress and risks

186 acknowledged the request shape;183/69 released the current wire files and
confirmed4f620ea0. Caller/server raw membership behavior is not native proof.
Retain ordinary group/whitelist behavior and current initiating manage-bots
permission admission. Do not mint a human default Role or delete unrelated data.
Legacy unqualified in-flight installation effects require explicit release
reconciliation; this local implementation does not fabricate that evidence.

Real PostgreSQL RED:6.098s at old remote call before any intent; initial GREEN
5.758s. Five recovery/store/signature tests passed with zero skips. A second
RED5.448s proved missing Role bypassed public admission; fail-closed correction
and current-permission negative test passed. Actual Linux gRPC/mTLS test passed
with required Bot client certificate and rejected wrong Space server name.
The observed two-connection migration DOWN wait blocks behind an uncommitted
first writer, then refuses to erase its committed authority. Purge compatibility
RED3.549s found one retained installation snapshot; the new table is now erased
within the existing terminal Space purge transaction, preserving its permanent
admission head. No global P3/rollout qualification follows from these tests.

Full Bot suite exposed outdated migration lists in fixtures; updated to008.
Its existing lock-order test initially spent its10s budget in PG startup (failed
at CreateBot before lock interleaving); move the unchanged10s oracle budget
after startup. Whole suite, race subset, vet, independent review and real186
receiver combination remain open. No native receiver effect is proved by the
controlled Space client used in producer component tests.
