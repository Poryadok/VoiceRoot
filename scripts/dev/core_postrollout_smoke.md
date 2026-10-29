# DEV post-rollout core smoke

The central operator calls `CoreSmoke(journal, fence, client, lock,
namespace_uid, release, token).run_dev()` only inside its active, held DEV
operation after the source-owned `full_cohort_release.verify_completed` has
qualified the final package, resumed workloads and required health. The same
typed release phase and token are rechecked at each effect and by
`verify_completed(phase, token)`. The release verifier is a separate central
dependency; this module fails closed until it is present and qualified.

The fixed public DEV API and recipient-scoped DEV mail fixture create two new
regular users, one new Space, its invite and the second user's membership. The
registration's initial verification message is read through the fixed,
no-proxy DEV mail service; the joiner's Space list is checked for the exact
new Space without retaining other list entries. The
private Journal records credentials and actual responses; the shared ledger
contains only nonces and hashes. Each remote effect has a durable pending
record; unknown or failed outcomes mark the owning fence uncertain and are not
retried. The accounts, Space and invite remain as operation-owned evidence.
No global Redis operation, deletion, purge or cleanup is performed.

This is DEV-only. Repository and TTenrec evidence reviewed for this slice does
not establish a recipient-scoped STAGING mailbox or an operation-owned verified
account source. STAGING requires an independently qualified credential/mail
adapter and its own fenced caller; the DEV fixture cannot authorize it. A
local HTTP fixture proves request sequencing and error handling, not a live
rollout, release verifier or real Kubernetes/TT authority.
