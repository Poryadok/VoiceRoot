# DEV delegated peer issuer

The issuer payload has an independently reviewed source pin. Integration,
actual source publication, installation and live authority remain separate
prerequisites; the pin alone does not authorize production operation.

The intended ordinary operator is `pmd`. A fixed socket-activated service
retains root custody of the existing media CA and accepts only the three fixed
profiles `creator`, `space`, and `role`. The operator's protected phase
and typed profile receiver establish the current claim, namespace, package,
lock, existing CA, and complete nine-name metadata. Each profile needs a fresh
preflight after any preceding create. Existing Secrets must be independently
qualified and are never replaced by this issuer.

The service independently verifies the candidate master CI and installed
source bytes, rechecks the fixed TT and Kubernetes metadata, and generates
only the absent names in the selected profile. It sends leaf material in a
sealed descriptor to the same verified operator process. The receiver checks
the packet and current authority before acknowledging. A missing response or
any failure after the durable reservation requires reconciliation; do not
retry. Neither an ACK nor issuance demonstrates runtime readiness.

A one-time reviewed bootstrap must install the exact source bundle and systemd
units before ordinary operation. The bootstrap must authenticate its own code
and the exact bundle digest before executing installer code with privilege.
No installer, signing operation, or cluster mutation is performed by these
source tests. The existing CA private key remains in its original root-only
archive. STAGING has a separate issuer, CA, namespace, and installation path.
