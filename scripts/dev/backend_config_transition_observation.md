# Guarded pre-cut configuration observation

Create a private journal with the shared `resource_context` and `namespace_uid`,
and immutable `backend_config_observation` containing `source_sha` equal to the
fence commit and exactly two ordered target objects for `notification`, then
`user`. Each target has `workload`, `pod_name`, `pod_uid`, `deployment_uid`,
`container_id` and `restart_count`, independently pinned by the operator.

`Observation(journal, fence, client, lock, namespace_uid).run()` reserves an
attempt under that fence and invokes the fixed read-only runtime probe for each
target using its bounded private transport. Its private receipt contains only
sanitized categories and identities; the ledger records the attempt nonce and
canonical receipt digest. A mismatch or unknown outcome after reservation marks
the fence uncertain and requires reconciliation without automatic replay.

The receipt is an observation made under this operation. It does not establish
the chosen source as release authority, which Secret version a process loaded
at startup, provider credential validity, transport readiness, or permission to
change any workload. Before a configuration patch the operator must revalidate
the protected receipt, relevant object identities and the live writer cut.
