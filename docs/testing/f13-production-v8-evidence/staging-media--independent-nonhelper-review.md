# Independent non-helper staging repair review

Reviewer: bot_installation. Result: CLEAR for the bounded non-helper source delta in WT35 against 3a2b8e19b0f8fe7a6104b753340f62864e039326. No source edits or reviewer runtime tests.

Scope: installer and bridge release-policy forwarding/validation; Bot schema prerequisite ceiling; historical Story fixture qualification; exact bundled file inventory. Helper enrollment/restart hunks in shared files are excluded and belong to role_admission's separate review. The four copied configuration files are also excluded from this review.

The installer admits only the explicit release_policy key and invokes the existing strict validator before installation mutation. The validator checks the immutable object, exact schema/profile/owners, content digest and embedded source binding. Installation does not claim candidate approval: preparation separately validates against approved.source_sha before image selection. The compiler still requires policy for current templates; missing/malformed input is not synthesized. Existing upgrade paths retain old private policy bytes. Tests assert invalid values cannot initialize recovery/install state, candidate mismatch cannot compile/prepare, and the supplied policy survives installation.

Bot008 creates membership evidence tables and does not alter interaction-token uniqueness. Extending the read-only prerequisite ceiling from seven to eight is bounded; SQL and authority reread are unchanged. Tests retain dirty/duplicate/type/extra-field vetoes, accept exactly canonical versions one through eight, and reject nine.

Story positive fixtures now copy only the four migrations already pinned by actor_root.STORY_UP, checking each actual file hash. Production Story qualification is unchanged. A separate negative copies actual later migrations and proves the historical clean4 witness refuses before database observation. This avoids treating a historical witness as current-schema permission.

Bundle inventory increases exactly 84 to85 and explicitly asserts release_policy_wiring.py inclusion. Existing remaining inventory assertions persist. README accurately distinguishes policy transport and prerequisite verification from deployment authorization.

Frozen SHA256 (scripts/staging/nats-rollout-preservation/):

| File | SHA256 |
|---|---|
| installer.py | 4b7ee50c2a0a055f0eb2b8804077562ce390b559597bafcee71b1f7277f1abef |
| bridge_root.py | efa249bc5e1135e7267918a1daee0ec28d0bde25d0e17f23a37049d378224792 |
| bridge_integration_test.py | 64f1fd77cd554480cf9f1d3b8e206e79b47bba94ed75abed789326d8378592e3 |
| bridge_composition_test.py | 10de1bd9e3f998b9c3eaa417d424f42f8c2ae210bcba828dceb347a4c0525858 |
| bot_migration.py | d41d2c245ce2f1b5dce7c45b7f56a576b7fb7b1c38fc1713f162de522ce2770f |
| bot_migration_test.py | 7ee18efe742995b71342be73e2e8075cdd25f6e2601731f89c644951c977e375 |
| actor_root_test.py | cb339d750bc4262d139a3804d266c79e33c471d4514f0a4efe7740f543281225 |
| expired_public_test.py | 321cc00eadeca2fc1a9eeb438db0fc92f9b59665d6d268d6d31b91574c1ef8aa |
| README.md | 74e9454e9bd229c29d61490079a580b26365115e8c40b785408947fda07cd78c |

Runtime focused/full-suite results were pending at review. This is source qualification only, not deployment, live acceptance, certificate-content validation, or proof that the complete staging inventory passes.
