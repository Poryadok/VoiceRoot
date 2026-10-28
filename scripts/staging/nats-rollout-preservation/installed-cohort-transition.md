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
