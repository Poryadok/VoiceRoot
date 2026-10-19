# BE-011 native party timeout

Existing BE011c147 request and accepted332c149-c150, contract
3b8aaeaf1674c021883d22f6335d9f08a523f2716a2c0213b17b88822c0e2362.
Owner145; no new product task or authority. The previous RunNativeOnce used
scalar timeout state/Redis/events. A real PG/Redis RED reproduced missing native
generation/party/RELEASE/facts. Implement the complete searching-party terminal
transition with exact common persisted deadline and DB time after lock waits;
keep pending-match recovery and cancellation separate.

Implemented MM027 dedicated frozen timeout outbox, FIRST RC expiry, complete
member/source/origin validation, all-member state/generation/party/RELEASE/facts
commit, exact replay, leased delivery and canonical MatchTimeout publisher.
Native Sweeper invokes it instead of scalar cleanup. Restricted LOGIN checks
allow append/delivery-column updates and deny frozen column/table mutation.

Own exact component fixture is Git8f9a3fa backend plus only this timeout delta,
not pending root files. Actual Linux PG/Redis/NATS race checks cover complete
party expiry, rollback, concurrent sweepers, unknown/mixed/null identity, CLOSED,
future DB deadline/after-lock expiry, pending reservation wait, old qualified ACK,
cancellation winner and exact lost-PubAck/SQL-ACK replay. Separate actual LOGIN
RED nil to GREEN refusal covers column-only payload privilege. DOWN tests cover
empty RC roundtrip, already-pinned RR and a noncooperating first unknown fact
committed while DOWN waits for AEX. Scoped vet passes.

Earlier test fixtures tried deleting retained origins and used an unqualified
queue ACK; both were rejected correctly. The corrected fixtures use real
pre-install typed history without origin and a genuine private projection proof.
The common-deadline fixture uses statement_timestamp, not a per-row volatile
clock. Historical failed logs stay in the packet; no failed run is called PASS.

Root Run/cmd/image/factory/close and full client-Gateway-MM cross-process
qualification remain separate unfinished BE011 work. Current root-only working
files are excluded from this coherent component. No installed authority, whole
feature readiness, current MASTER/CI or merge offer follows from these checks.
