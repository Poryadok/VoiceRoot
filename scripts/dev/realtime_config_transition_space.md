# Space HTTPS evidence

`SpacePhase(journal, fence, client, lock, namespace_uid, package)` records a
pre-cut `run(token)` under the same operation as a concrete `PackagePhase`.
The private intent `realtime_space_https` contains `source_root`, `space_pod`
and an absolute fresh private `destination`. The phase invokes the source-owned
public-input and HTTPS capture, stores the public JWKS in its private journal,
and records only the receipt digest in the shared ledger. A pending or unknown
attempt is not replayed automatically.

The capture verifies the public CA, exact peer certificate and SNI over a
bounded Kubernetes port-forward to the observed Space Pod. It compares
Service/Endpoints and workload identities around the read and accepts only the
source-defined public RSA JWKS shape. This proves the observed Pod HTTPS
response through that tunnel; it does not prove Service dataplane routing or
future client connectivity. Responses using unsupported HTTP framing hold.

`verify(phase, cut, token)` reopens the protected phase receipt after the cut,
checks the original Space Deployment UID, generation and full spec against the
WriterCut admission, and rereads public Secret and Service identities. It does
not exec a drained Pod. Both functions require the real shared fence and package
authority; synthetic test results and bootstrap issuer-plan receipts do not
establish actual Space serving or P3 readiness.
