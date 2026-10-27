# Configured User/Space cold-start fixture

`coldstart_proxy.py` forwards real HTTP/2 bytes. Normal User port19090 and Space
port19091 forward only while that target's actual HTTP `/health` returns200.
User peer port19092 forwards to the same actual User9090 regardless of health.
It never serves an authority RPC. Run it on a new private Linux Docker network
with aliases `user` and `space`, no published host ports. This explicitly models
ready-gated discovery; it is not Kubernetes rollout/EndpointSlice evidence.

Build actual User and Space Linux binaries from the selected source (the Go
workspace needs sibling modules), plus `go build ./testdata/coldstartprobe` from
User. Give each case new containers/aliases and reciprocal configured addresses:

- User: `USER_GRPC_ADDR=:9090`, `SPACE_GRPC_ADDR=proxy:19091`, real
  `AUTH_GRPC_ADDR`, `USER_REDIS_ADDR`, canonical migrated `user_db` DSN.
- Space: `SPACE_GRPC_LISTEN=:9090`, `USER_GRPC_ADDR=proxy:19092`, canonical
  `space_db` nonowner serving-role DSN.
- Both: explicit bounded `GRPC_DIAL_TIMEOUT`; selected documented release
  profile and every collaborator enabled by that profile. No static authority
  grants, missing bindings, or protected ordinary-port fallback.

Qualify stores before counting RED. Fresh Space must be provisioned by the
source-bound real `native-migrate --mode provision` and its packaged pinned
native driver; retain the genuine first-controller JSONL immediately. Follow
`docker/postgres/compose-migrate-dbs.sh`, then the canonical
`provision-space-runtime-role.sql`; never force a version or borrow another
database's installer receipt. Use the resulting nonowner runtime credential.
Apply User's canonical migrations and use the real documented Auth service and
its migrations/JWT material, not a dummy transport standing in for Auth.

Run original binaries first: both reciprocal dial deadlines must fail after all
noncycle prerequisites qualify. Then run the changed pair simultaneously and
in each staggered order. Before Space starts, the `transport proxy:19092` probe
must observe Ready, `deny proxy:19092` must observe actual User Unavailable, and
User health must fail. Both health endpoints must converge after Space starts.
Changing only Space back to normal `proxy:19090` is the early-Serve-only negative
control: neither becomes healthy. Each absent configured peer must keep health
closed, reach its deadline and close the process/early transport.

`space-reconnect proxy:19091` retains the real production User co-membership
adapter/channel. With optional principal integrations disabled, real Space must
deny a forged raw Social marker before interruption, return Unavailable while stopped,
and deny again after its restart. Restart Space only after the probe prints
`SPACE_UNAVAILABLE`; require all three phases and exit0. This proves bounded
denial/reconnect, not positive principal authority or a full privacy workflow.

Component checks: Linux `go test -race -count=1 -run TestStartup .`, User and
Space `go test -short ./...`, and pkg `go test -short ./socialprincipal ./privacy`.
Short suites intentionally skip database fixtures. Startup tests separately use
actual mTLS for closed admission, wrong CA/name/missing client identity, and
ordinary capture non-exposure. They do not replace protected-on qualification.
Retain source/binary/image/manifest hashes, native receipt, logs, case receipts,
execution identity and exact owned-resource cleanup. Never delete unknown
containers, networks or volumes.
