# Realtime configuration transition

`render_realtime` and `render_chat` are source-bound rendering helpers, not
authority to patch Kubernetes. They take already-drained Deployments, preserve
zero replicas and the existing images, and produce full-spec UID/RV JSON Patch
tests. Realtime rendering delegates its fixed P3 configuration to the existing
planner. Chat adds only the documented Realtime listener, trust mounts and replay
references. Unknown or conflicting configuration is refused. A real operation
must validate the supplied asset, schema, transport and ConfigMap information
against its authenticated producer journals and live identities; the planner's
syntactic receipt checks alone do not establish any of these facts.

`realtime_config_transition_redis.observe` is a read-only, fenced observation
of the fixed DEV Redis Service, endpoint and selected ready Pod, plus the empty
P3 replay password reference. It verifies unauthenticated PING and two sample
`ACL DRYRUN` checks without writing Redis keys, while discarding inherited
`REDISCLI_AUTH`. It rechecks resource identities around the commands. The
result does not prove permission for every possible replay key, publisher or
peer network health, or a completed configuration transition.

The actual configuration phase remains unavailable until the central consumer
binds these fixed transformations to the WriterCut plan, real migration and P3
asset journal receipts, the source-qualified publisher producer, and complete
transport/replay qualification. Existing Space HTTPS issuer inputs must be
revalidated under the operation; these helpers do not enable its unrelated
lifecycle groups. All fixtures are synthetic and are not live readiness proof.
