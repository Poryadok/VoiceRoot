# Shared File tracked-write contract

This physical I/O protocol is shared by catalog imports and PDF/GIF derivatives.
It supplies neither actor authorization nor permission to publish a File. The
historically named `file_catalog_writes` is the single ledger; do not create a
second PDF ledger. Additive migration 11 adds `owner_kind` with default
`catalog_source` and closed values `catalog_source`, `pdf_thumbnail`, `gif_full`,
`gif_thumbnail`. The selector authority migration is separately reserved as 12.

## Store API

`TrackedWriteIntent` contains `Key`, `FileID`, `Generation`, `Kind`,
`ExpectedSHA256` (32 measured bytes), and positive `ExpectedSize` (int64). Catalog
source bodies are bounded by 50 MiB; derived objects by 5 MiB. Budgets are never
reported as actual size. Keys/generations are unique server-owned attempts.

- `DeclareTrackedWriteTx(ctx, pgx.Tx, intent) error` persists DECLARED inside the
  owner's current-generation/File transaction after actual output facts exist.
- `StartTrackedWriteTx(ctx, pgx.Tx, key, generation) (intent, error)` commits
  STARTED with that same owner-generation admission before the only PUT.
  `FilesStore.StartTrackedWrite` is a convenience physical transaction; callers
  requiring additional owner/actor generation checks use the Tx variant.
- `FilesStore.SettleTrackedWrite(ctx, key, generation, acknowledged) error`
  records the actual storage outcome on a fresh, bounded connection independently
  of caller cancellation, File deletion or actor/current-publication loss.
- `CanCollectTrackedObjectTx(ctx, tx, fileID, key)` and
  `CanCollectTrackedBlobTx(ctx, tx, blobID)` supply the shared physical predicate.
  They do not replace reference/alias/attempt/lifecycle ownership checks.

Declarations are immutable. A repeated exact DECLARED intent is idempotent;
changed bytes/owner/generation conflict. A second Start never creates a second
network attempt. Every physical cleanup route checks the same predicate after
its File/blob lock and a fresh snapshot, including direct GIF cleanup.

File, blob and applicable owner generation must still permit I/O at declaration
and start. PDF owns its generation checks in its own transaction and invokes no
catalog actor/import method. GIF attempts also match the current live job token.
No SQL transaction is held over the network PUT.

## Storage and outcome

`r2file.TrackedPut` contains `Key`, `Kind`, `ContentType`, `ExpectedSHA256` and
`ExpectedSize`; `S3R2Presigner.PutTrackedObject(ctx, declaration, body)` checks
actual bytes against the declaration and performs one synchronous SDK attempt,
following no HTTP redirects. `PutCatalogSource` retains its stricter source-key/
GIF checks and delegates to this same writer. No default SDK retry is substituted.

Only an actual successful single-attempt ACK moves STARTED to SETTLED. The
acknowledged argument is an internal executor result, never an admin/RPC fencing
assertion. Any error/lost ACK/crash stays STARTED or UNSETTLED and pins physical
collection without expiry. An UNSETTLED record cannot be promoted by a later
boolean assertion. No HEAD, timeout, age, session loss or local goroutine drain
proves an already submitted remote PUT cannot still commit.

After physical settlement, the owner independently rechecks current File/actor/
selection/generation authority before publication. If deletion or supersession
won, queue eligible GC and never publish READY. Unknown remote liveness remains
explicit reconciliation work, not a fabricated automatic cleanup success.

Before migration 11, only the existing catalog-source ledger shape is supported.
PDF/derived owner kinds fail closed rather than falling back to an ordinary PUT.
Old ordinary schemas with no ledger retain their old collection behavior; a
known catalog File without its ledger fails closed. No new retention entitlement,
TTL, source URL authority or durable message reference is introduced.
