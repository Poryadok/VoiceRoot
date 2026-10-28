# Media database backup primitives

`story_media_backup.py` provides streaming AES-256-CBC/PBKDF2 encryption,
per-member HMAC and an HMAC-authenticated canonical manifest binding. It is
an offline primitive, not a capture operator, qualified restore receipt or
permission to migrate. The required members are `file_db`, `story_db`,
`messaging_db` and password-free PostgreSQL roles. The caller supplies a fresh
32-byte key, private 0700 owner-only directory, canonical operation binding and
`producer(member, output)`. The producer must stream the requested dump into
`output` and return only after successful process exit; failure leaves a partial
private set for reconciliation and returns no manifest. Deadlines, source
identity, claim and writer drain are the caller's responsibility.

`verify` authenticates the manifest and every member. `decrypt` authenticates
a private ciphertext snapshot before writing plaintext to the supplied sink.
If decrypt or restore fails, discard the entire isolated restore destination;
no automatic live restore or retry is provided. The caller must establish key
custody and the identity of an independently observed off-node failure domain.
The authenticated binding is only as trustworthy as its protected producer.
Separate database dumps do not by themselves establish a cross-service cut.

Before a consumer admits any migration it must independently establish and
continuously maintain the real operation claim and full writer/transaction and
object-store request drain, bind source namespace/Pod/PVC identities, collect
server version and catalog, capture every dump and check each exit, protect and
read back an off-node copy, and actually restore roles and databases into an
isolated compatible PostgreSQL. It must compare catalog and retained data,
run the exact source migrations and applicable binary/invariant checks, and
bind the resulting evidence to the same operation. MinIO PVC/credentials and
NATS require their own backup and isolated restore evidence; these four members
are not a full application backup. No such production adapter is installed by
this module.

The synthetic integration test in `story_media_backup_integration_test.py`
requires a Linux Docker daemon, the pinned PostgreSQL image in the test, Docker
CLI available to the test runner, a read-only checkout at `/repo`, and Docker
socket access. It creates its own labelled, networkless containers and verifies
their IDs before cleanup. It applies the old and new SQL directly, rather than
the production migration runner, and does not establish off-node custody,
actual environment data, object-store contents, NATS state or binary readiness.
Run the unit and integration tests in an isolated Linux runner:

```sh
cd /repo/scripts/dev
python3 -B -m unittest story_media_backup_test story_media_backup_integration_test
```
