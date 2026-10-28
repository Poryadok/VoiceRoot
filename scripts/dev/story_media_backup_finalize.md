# Receipt-bound media backup manifest

`finalize(root, key, original_binding, original_manifest, receipts)` is a
post-export signing step. It copies its inputs, validates the closed operation
identity and restore-input digest schema, verifies the existing encrypted set,
then adds `dump_receipts` containing canonical SHA-256 hashes of exactly four
distinct `dump-terminal-only` receipts and re-authenticates the unchanged file
identities. It returns `(final_binding, final_manifest)`; neither key nor raw
receipt data is returned or placed in an artifact by this function.

This function validates consistency only. Before calling it, the transaction
must obtain the actual four receipts from its protected local journal, verify
each matching per-member attempt nonce against the live fenced ledger, and
recheck the real claim and continuous writer/transaction/object-store drain.
Afterwards it must guard again, bind the final manifest to the same journal and
use the final binding for custody. Constructing matching dictionaries or
matching hashes cannot establish that a dump ran or that writers stayed fenced.

The original binding contains only operation and resource identities plus
`restore_inputs` hashes of protected source catalog, role catalog and exact
migration inventory, pinned PostgreSQL and migration image references, the
migration binary hash, and PostgreSQL major. The operator must independently
verify and retain the corresponding metadata content and immutable artifacts;
hashes alone are not restore or compatibility proof. Source metadata and
receipts must not include database rows, role passwords or credentials.
Additional databases and MinIO/NATS preservation require their own coverage.

Linux unit tests are `python3 -B -m unittest story_media_backup_finalize_test`.
They use synthetic identities and receipts, with an actual encrypted bundle;
they do not contact the ledger, DEV, GitHub or an off-node restore target.
