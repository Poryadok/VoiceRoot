# Realtime principal JWKS publisher

The fixed DEV publisher is a separate fenced create-only phase. It derives the
three publisher documents from the reviewed staging source, uses a pinned nginx
OCI index and verifies its unique linux/amd64 manifest and config bytes. The
target node identity and architecture are checked under the same operation
fence; a hostname selector keeps the Pod on that node. The Pod does not mount a
ServiceAccount token or the Realtime issuer signing keys.

The phase consumes the completed P3 handoff from its actual Journal and Fence,
rechecks the immutable public JWKS ConfigMap and publicly projected TLS
certificate against the recorded identities and trust root, then performs
absence checks and server dry-runs before any create. Each create has a durable
pending record; the result is bound to its UID, resourceVersion and complete
admitted content or spec. Rechecks include the owned ReplicaSet and Pod,
selected node, actual image ID and ready state. The Pod check relies on the
project's trusted Kubernetes controller and admission boundary; it does not
claim that post-creation observation prevents a compromised admission plugin.

The returned proof is create-only, not proof of HTTPS reachability or Chat
readiness. A conflict, drift or unknown outcome holds the common fence for
reconciliation; there is no name-only adoption, deletion or automatic retry.
Reuse in a later cycle requires the separately authenticated predecessor capture
and a fresh exact live-state check. No live state is changed by unit fixtures.
