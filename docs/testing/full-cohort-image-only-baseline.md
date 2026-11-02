# Full cohort image-only baseline qualification

Status: integration contract; not a DEV or STAGING deployment result.

The first full-cohort operation may encounter a fresh, same-claim DEV
observation that has an exact image map but no authenticated source commit.
That absence must not be replaced with the intended candidate commit. The
ordinary image updater continues to require its existing source ancestry
check.

The full-cohort path must bind its current claim, window and operation to a
fresh complete actual workload image inventory and require equality with the
resource's same-claim observation. Candidate source and the immutable full
package must come from the successful current-master selection and build
capture. Legacy image provenance remains unknown unless independently proven.

Before replacing a completed permanent executor row, capture that actual row,
its ConfigMap UID and resourceVersion and the relevant authenticated TT
history into the private operation journal. The same compare-and-swap that
begins the new row must bind the resulting descriptor. A lost response or
changed row requires reconciliation with the lock and journal retained.
Previously completed full deployments and other qualified protected TT source
floors remain evidence even when a later prepare observation has omitted a
commit. Every such applicable floor must be an ancestor of the candidate;
ambiguous or contradictory history holds rather than inventing a source.

The database plan must additionally compare each actual current ledger and
source-qualified target, rejecting a target below the observed version. Its
isolated restore and candidate compatibility evidence are separate required
checks, not consequences of image equality or ancestry. Only a genuinely
successful full rollout establishes a new common source for all selected
applications. A later cycle makes a new current-master selection and fresh
baseline; promotion within a cycle retains its already qualified immutable
package even if master moves.

Required tests include a valid image-only observation, changed image map,
stale claim/window/operation, a known non-ancestor floor, a conflicting
completed ledger or TT deployment event, a lower database target, and a lost
committed begin response. No test may represent an unknown legacy image as an
authenticated commit.
