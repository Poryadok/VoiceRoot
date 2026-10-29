# Realtime P3 create-only handoff

`Session(journal, fence, client, lock, namespace_uid)` is a bounded consumer for
the seven P3 objects described by the reviewed missing-only issuer. The caller
supplies a separate private `Journal` under the same live operation Fence and
ledger as the central executor. Its immutable intent includes `resource_context`,
`namespace_uid`, and `realtime_p3_create` with `issuer_source_sha`, absolute
`source_repo`, and `issuer_suffix`. The suffix must equal the 32-hex suffix of
the complete `media-c1-...` or `media-c2-...` Fence operation ID. The candidate
source in the Fence is independently admitted by the central executor; the
issuer source is a separate reviewed pin, compared against the six fixed issuer
cohort files in that candidate and its working tree.

After all source, root-custody and human availability prerequisites are ready,
`prepare()` verifies authority and current public CA/peer identities, refuses
existing target objects, and records a durable pending then ready handoff. The
ready record includes the operation, namespace, issuer/candidate identities,
PID and process start ticks, and socket inode. The human root exporter must
independently verify the owned operation and that PID/start time before sending.
The fixed private socket is `operations/<suffix>/packet.sock` under
`/home/pmd/voice-realtime-p3`; the process and directory are owned by pmd.
`finish()` receives one root-authenticated seqpacket and sealed memfd, validates
the packet and cryptography, performs all server dry runs, then creates each
absent object once after a durable pending record. It checks exact object content,
UID and resourceVersion, and returns a public-identity receipt only after the
terminal journal/ledger record and repeat observations. Packet contents and
private keys never enter the journal, command arguments, environment or logs.

Any uncertain handoff or create retains the shared lock for reconciliation;
there is no automatic replay, adoption, or deletion. The 300-second receive
bound applies only to an arranged transfer, not to waiting for user availability.
The receipt is create-only evidence: it does not establish Redis authentication,
peer connectivity, database/schema readiness, Deployment readiness, or complete
P3 admission. Actual source authority, private issuer custody, and the central
executor's operation and runtime checks remain required before live use. The
Linux fixtures use synthetic Kubernetes authority and do not constitute live
operation evidence.
