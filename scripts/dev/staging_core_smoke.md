# STAGING core smoke

This phase uses the two explicitly authorized existing test accounts. It logs
them in normally, creates one operation-owned Space and invite, joins with the
second account, and reads the new membership back. It does not register, reset,
delete or clean up accounts, Spaces, or unrelated data. Owned data remains.
An authentication challenge or ambiguous HTTP outcome is a HOLD, not a reason
to resend OTPs or retry an effect.

The operator may prepare the reviewed one-shot credential transfer with
`python -B scripts/dev/staging_core_credentials_transfer.py` from the reviewed
source. It verifies the fixed Windows source and host configuration, sends the
file via SSH stdin to a fixed exclusive private destination, and prints no
credential values. Any error or unknown response requires reconciliation;
do not retry or replace the destination automatically. The private source and
destination are ordinary trusted operator inputs, not a cryptographic assertion
about the human who supplied them.

The STAGING owner must invoke `StagingCoreSmoke(session, release).run()` inside
the existing held root operation scope, after source-owned full release and
health proof, and before `Session.complete`. `verify_completed(phase)` checks
the private result while the same authority is held. The typed
`staging_core_release` verifier and the actual caller are required dependencies;
the local HTTP fixtures alone do not qualify a rollout or grant authority.
If the same protected operation must be recovered, `recover_completed(session,
release)` reopens and validates only a completed receipt without HTTP effects;
an absent or pending receipt remains HOLD and is never replayed.
The fixed operation journal holds credentials, private HMAC key, tokens and
owned receipts: preserve its private custody and never print or export it.
