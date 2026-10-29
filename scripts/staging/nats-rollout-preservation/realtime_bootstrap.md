# Staging Realtime database bootstrap

The root prepare bridge may invoke this fixed prerequisite only with its admitted
STAGING Session, source-capture receipt and operation lock. It captures the
PostgreSQL workload, volume, service and endpoint identities, the active marker,
the workload baseline and the existing application Secret before binding a
bootstrap target. The Secret, generated credential and journal remain in the
root-private operation directory. Existing Realtime role, database or key are
not adopted by this create-only path; the separately qualified existing path
must prove its historical anchor and current state.

The fixed effects are a restricted `realtime` login, `realtime_db` owned by that
login, and an additive UID/resourceVersion/full-data CAS for
`voice-app-secrets/REALTIME_DATABASE_URL`. Each effect has a durable pending
record before submission. SQL checks the captured PostgreSQL system identifier
and predecessor catalog within the same session, under an advisory lock. The
Secret update requires a no-writer check and exact readback. SQL and credential
material travel through private stdin; no password is put into the receipt.

Completion checks the role and database OIDs, preserved Secret content, service
routing, scoped login authentication and all protected bindings. Its receipt is
bound into the ordinary full rollout target; it does not claim migration success.
The existing migration planner owns version 0 to 1 after bootstrap. A partial
effect, changed identity, lost response or pending journal requires reconciliation
under the existing root/TT lock. There is no automatic retry, adoption, DROP or
reset, and a successful local fixture does not establish live STAGING state.
