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

For User and Notification, create a separate open config Journal under the
same resource context and namespace. Its immutable `backend_config_deployments`
contains the source SHA, cut invocation, sorted selected service names, exact
drained source objects, and the completed pre-cut observation and its immutable
observation plan. The consumer verifies the actual shared-ledger observation
nonce/digest and the original Deployment UID/resourceVersion, rechecks current
Secret/ConfigMap UID/resourceVersion, and renders only an explicit
`VOICE_RELEASE_PROFILE=development` addition. All other configuration, the
original image and zero replicas are preserved. A field already set, a missing
template-bound observation, a changed reference or an unsupported service
holds. The operator first calls `cut.bind_config_plan(config_journal)`, then
`backend_config_transition_deployment.apply_development(cut, config_journal,
service)` in the exact plan order. After a guarded UID/RV/spec CAS it waits for
the controller to observe the new generation while checking authority, guard
inventory and zero Pods. The cut's session probe runs before the patch and
again during successor acceptance. It records the result and immediately calls
`cut.accept_config_successor`.
Unknown responses, timeout or drift mark uncertainty and forbid replay.

The caller must qualify complete writer selection, Service selector exclusivity,
real endpoints, P3 keys, TLS trust and peers separately. This effect does not
qualify those assets, update the Realtime Deployment, authorize images or
establish Realtime startup. Realtime Deployment adoption still requires source-qualified
P3 asset, peer, schema and transport evidence and is not implemented here.
