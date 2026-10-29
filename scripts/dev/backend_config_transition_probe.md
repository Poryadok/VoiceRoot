# User and Notification runtime configuration observation

`inspect(workload, pod_name, run_bytes, guard)` is a read-only bounded probe
for `user` or `notification` in `voice-dev`. It uses the existing guarded
workload snapshot to check Deployment → ReplicaSet → Pod ownership and the
same running container before and after observation. `run_bytes` must capture
stdout/stderr privately and apply bounded command deadlines; `guard` is the
operator's real shared fence. Only the central operator may run this probe in
its owned prepare window.

For the fixed source-derived release, capture and Notification provider keys,
the probe emits only fixed presence/profile/toggle categories and literal or
Secret/ConfigMap source identities (UID, resourceVersion and referenced key).
Secret and ConfigMap listing requests expose key names only, never values.
The ordered `env_sources` list includes every envFrom object's identity and
prefix, even when none of the selected keys exists, with only the selected
field names that exist. This allows later checks to detect changes to sources
that initially contributed no selected field.
These are current API object identities observed around the probe; they do not
prove which object version supplied a running process at its startup.
Explicit env overrides envFrom, and later envFrom sources override earlier
sources. Duplicate explicit variables, unsupported or optional sources, unknown
runtime origins, guard loss, container drift and resource identity drift hold.
The current program checks `/proc/1/exe` and examines `/proc/1/environ` inside
the container, emitting only the bounded category, never the variable value.
For toggles, non-ASCII bytes are unsupported and cause HOLD rather than an
approximation of Go's Unicode whitespace or case-folding behavior.

`present` means nonempty bytes, not valid credentials or a live sender. It does
not qualify provider keys, certificate files, mounted policy contents, endpoints,
startup behavior or an alpha release. The established DEV profile is development;
the operator must preserve actual configured providers and toggles. This
observation is not an admission receipt or evidence of an actual DEV call.
