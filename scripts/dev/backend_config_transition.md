# Realtime Service configuration transition

Before constructing the common `WriterCut`, the caller places the complete
admitted `voice-dev/voice-realtime` Service snapshot in the private immutable
journal intent as `backend_config_service`. It must also include the exact
Realtime Deployment in `writer_cut.controllers`. Once `cut.drain()` completes,
`ServiceTransition(journal, fence, client, lock, namespace_uid, cut)` may call
`apply_realtime_service()` once. It uses `realtime_p3_adoption.plan_service`
to add the fixed TCP lifecycle port while preserving the full Service spec,
checks UID/resourceVersion/full-spec CAS and then observes UID, target spec,
resourceVersion and, when present, the unchanged Service generation. The cut is reverified before and
after the effect; it does not change any Deployment or its replicas.

The private step `backend-config-service-voice-realtime` stores the operation,
cut invocation, source, target spec and patch, together with a unique nonce and
observed receipt. The shared ledger stores the corresponding details/receipt
digests in `backend_config_effects`; both are checked on each guard. An
unknown response or failed postcondition marks the fence uncertain, retains the
lock and requires reconciliation without replay. An already matching live
Service is not an owned completion.

The caller must qualify complete writer selection, Service selector exclusivity,
real endpoints, P3 keys, TLS trust and peers separately. This effect does not
qualify those assets, update a Deployment, authorize images or establish
Realtime startup. User and Notification configuration and Realtime Deployment
adoption require separate source-qualified policy/asset inputs and a cut
verifier that accepts only authenticated configuration successors.
