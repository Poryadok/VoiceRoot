# Bot protected native Space installation — PR875

## Scope and decision

Implement accepted finite source decision18 for fresh or known-retained native
Space installations. Sources: features/bots.md, microservices/bot-service.md,
existing Role/Messaging typed removal protocol. Legacy rows remain LEGACY_UNKNOWN.
Global Bot deletion, legacy adoption and Space P3 cleanup activation are excluded;
historical receiverstage12 and Auth172 stops remain in effect.

Migration008 stores retained heads, independent Add/Cancel operations, monotonic
generations and exact request bindings. No RPC runs inside a database transaction.
Incomplete installation remains invisible; cancellation revokes visible authority
before cleanup. Only matching current CAS acknowledgements complete work.
Authenticated retries resume durable pending work after restart. This slice has
no autonomous recovery sweep.

Chat authority observations are persisted before activation. Cancellation freezes
the installation tuple and full sorted original observation set. Role/Messaging
requests and typed receipts persist before acknowledgement. Missing, malformed or
pending receipts cannot release a generation. Completed effects are reused.
Raw installation and new whitelist insertion cannot bypass the migrated journal.
Space owns membership; Bot does not call Chat.AddMembers for Space chats.

## Protected runtime

The dedicated listener accepts only InstallBotInSpace and UninstallBotFromSpace;
all streaming and other unary methods are denied. The ordinary listener cannot
mint the private administrator capability. Admission verifies Gateway delegated
credentials, request hash/ID/method, Redis replay and actual Auth Bot session floor,
then obtains Role SPACE_MANAGE_BOTS permission. Outbound calls use Bot canonical
issuer, fresh JTI and persisted operation IDs over verified mTLS.

Required ingress variables: BOT_INSTALL_GRPC_LISTEN, BOT_INSTALL_TLS_CERT_FILE,
BOT_INSTALL_TLS_KEY_FILE, BOT_INSTALL_TLS_CLIENT_CA_FILE,
BOT_INSTALL_GATEWAY_JWKS_URL, BOT_INSTALL_GATEWAY_JWKS_CA_FILE and
BOT_PRINCIPAL_REPLAY_REDIS_URL.

Outbound prefixes: BOT_SPACE_MEMBERSHIP_, BOT_INSTALL_ROLE_, BOT_INSTALL_CHAT_,
BOT_INSTALL_MESSAGING_, BOT_INSTALL_AUTH_. Each requires GRPC_ADDR, TLS_CA_FILE,
TLS_CERT_FILE, TLS_KEY_FILE; optional TLS_SERVER_NAME defaults to endpoint host.
BOT_SPACE_MEMBERSHIP_SOURCE_GENERATION must match retained migration008 source.
Alternatively BOT_SPACE_MEMBERSHIP_SOURCE_FILE reads a canonical UUID from the
original external artifact; configuring both forms is rejected. Compose uses
the project-scoped bot_membership_source volume at /native-bot-evidence/source.uuid.
The same helper runs from CLI migration and compose-db-init. It qualifies a new
source only from an empty database before migration, then publishes the exact
new identity atomically. Retained starts verify the original artifact and never
recapture a missing file from the database. Interrupted provisioning fails closed.
BOT_SPACE_MEMBERSHIP_SOURCE_KIND is OWNED_FRESH or KNOWN_RETAINED. Partial/blank
configuration fails startup. Startup closes owned listeners, clients and Redis;
server failure terminates the process. Lazy JWKS fetch avoids publisher cycles,
with bounded last-good validity. No public protobuf fields were added.

## Reuse and verification

Adopted the frozen Isaac16 skeleton and finite retained-source design. Ported
typed receipt validation and Chat observation codecs from historical5c5c94df4
(including b3aece746/03fdcb656 ancestry). Broad global-delete/P3 producer excluded.

Author tests use Linux golang1.26.5 and owned PostgreSQL16. Canonical sha256 hash
test first failed all five original bare-hash constraints, then passed correction.
Native tests exercise prepare/replay, stale ACK after cancel, unresolved cleanup,
lost Messaging reply, malformed receipt, restart with saved Role result, successor,
raw whitelist denial and immutable evidence. Bot cleanup test receiver clients
are controlled signed fixtures, not cross-service deployment proof.

Full Bot short suite and vet passed. Focused race proof and final integration are
recorded in dispatcher evidence. Independent static review inspected admission,
CAS/replay, typed cleanup and startup ownership; no additional critical finding.
Its request-call immutability suggestion was applied.

Root owns actual receiver integration, Gateway/Compose and live CI. No deployment
or PR merge here. Tests do not establish restore custody or global deletion
readiness. Preserve unresolved evidence until separate contracts are qualified.
