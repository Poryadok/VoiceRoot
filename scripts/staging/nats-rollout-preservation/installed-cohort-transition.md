# Installed 81-member cohort transition

The public installed manifest observed on STAGING has canonical binding
`78f5532cfc06766e5482caf402a6a0d30c7d47255f1fc441a43149132859546b`.
Its 78 source files match the exact paths at Git commit
`d9a4a292fd18f734e18cdcfab5876fa036392e6a`; the three binary build identities
are not established by that observation. The transition preserves their exact
observed hashes. A changed or differently installed predecessor is refused.

The finite successor contains the member set of the TT fencing successor, with
source from `9aa2e4d17beec62c5a853437e48b65239538a856`, the three unchanged
installed binaries, and the reviewed transition's own `installer.py`. Its
85-member projection excluding the installer has SHA-256
`d4c2c9f639975f66c6cc2caeac713d3e447d76cebf7f7d3b1edb5673f5da884a`.
The existing TT predecessor reconstruction must also yield full binding
`0a81983170a46924ed21194e005087a999748a7e13e6762d6eefa76ed526fc21`.
This is an intermediate TT-capable helper, not the later GIS successor.

Before use, independently review the exact source and binary inputs, complete
manifest, generated bundle and launcher. Record the full target binding and
both artifact digests externally. The launcher digest alone is not source
authority if selected by the same unreviewed candidate. Use a trusted root
wrapper with a separately fixed launcher SHA-256 and size, as in the existing
root-capture procedure in `README.md`; the captured launcher verifies its
embedded exact bundle digest, size and complete member manifest. The route is
`rollout-root.sh --install --upgrade-idle-installed-cohort-v1` after that
independent binding. No caller-selected predecessor or target hash is accepted
by the installer. Its self-byte check is only integrity, not independent
authorization of its source.

The existing root operation lock, active marker, terminal retirement, empty
inbox/processing, request and TT-resource journal, code inventory, and preserved
history checks still apply. Completed request/resource history and old code are
retained; unknown or active resource state is rejected. A failed or interrupted
upgrade requires the existing exact receipt, staged code and preserved backup to
agree before continuation. Abrupt process death or power loss after the inbox
is closed can leave its mode at `0700`; the full route then refuses retry because
it requires `1730`. This is a fail-closed reconciliation boundary, not an
automatic resume. Do not change the mode, delete history, or broaden the guard
to manufacture eligibility; use a separately reviewed recovery procedure.
This source route does not prove actual host idle state, authorize a particular
binary build, establish root access, or install the later GIS successor.

## Direct GIS successor

The separately reviewed direct route targets the same observed 81-member
predecessor and skips the intermediate TT successor. Its provisional source
snapshot is `e2b5e42abedaaaf50e9007c435e9c88cd4f48420`, including the GIS
update and ZIP64 custody code. The exact 87-member successor consists of the
TT member names plus `nats-rollout-preservation/gis_existing_update.py`, the
same three observed binary bytes, and the reviewed running installer. Its
86-member projection excluding installer has SHA-256
`d64f32b7383e06cd454d2f302ebce656ef7dfcb8b97b3012f20824f2b74615ef`.
The root entry is `--install --upgrade-idle-gis-installed-cohort-v1`. This
source snapshot is a candidate, not by itself final source or CI authority;
if any source byte changes, reconstruct the projection and repeat review.

The same independently fixed wrapper-to-launcher-to-full-bundle authority,
live idle checks, TT journal and inventory guards, history retention, and
inbox `0700` reconciliation boundary apply. The direct route does not accept
another installed predecessor or authorize arbitrary 87-member bundles.

## Full 23 prerequisite successor preparation

The earlier 87-member GIS successor is an intermediate source checkpoint; it
must not be installed solely to reach the later full 23 prerequisite helper.
The future direct transition is tied to the observed 81-member predecessor,
not to an assumed intermediate installation. Its final source and member
inventory must be frozen and independently reviewed before its entry is used.
The provisional candidate is based on `83950c631442553a551cdacdf659a996ab2a3cf3`
and contains 94 members, including the protected prerequisite and Realtime
bootstrap modules and the observed binary pins. Its 93-member projection excluding its independently
authorized running installer is
`54aa6eca4a2f7435224fac6e3ac5f4953601122d91fded31eb7dd3953fc62ff4`.
The fixed entry is `--install --upgrade-idle-full23-installed-cohort-v1`;
any change to its source members requires a new projection, exact bundle and
independent review. This candidate does not establish eligible source or CI.

For this exact legacy predecessor, new private history paths
`installed/source-authority`, `installed/realtime-bootstrap` and
`installed/prerequisites` must all be
absent under the existing operation lock before closing ingress, immediately
before the code swap, and immediately after it. Any existing file, directory,
or symbolic link at any path requires reconciliation; nothing is deleted
or adopted. Established checks continue to protect reviewed source workspaces
and rollout history. The old helper can have `installed/source-archives`; the
transition neither requires that cache to be absent nor replaces it.

If a new history path appears after the swap, the command reports a blocking
failure but code may already have changed and ingress is restored to `01730`.
This is not rollback or evidence of completion. Restrict external access and
reconcile the actual code, receipt, preserved predecessor and private history
before any continuation. The operation lock is cooperative and does not stop
an unrelated root writer.
