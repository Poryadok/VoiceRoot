# Phase0 fixture directory enumeration — independent review

Result: **CLEAR**, final author runtime verification GREEN on unchanged source. Exact reviewed file: `scripts/ci/phase0-fixture_test.py`, SHA-256 `4a358406e826b70832f774e69658d444690e708e0fef0bf82f669545b7cc91ab` in WT43. Baseline `c1c340c809aa619a4ee18230b99356959fdb4a40`. Reviewer `/root/role_admission`; no source edit, reviewer test execution, live Compose or stopped Auth/Chat scenario performed.

The test retains its UID65532-shaped fixture; other Compose checks assert this ownership. The former host `iterdir` fails on service-owned mode0700 Chat/Space directories in nonroot CI. The added assertion helper handles only this ownership case through a subprocess: asserts gid65532 and mode0700, clears supplemental groups, sets gid then uid65532, and only then evaluates the same nonempty-directory assertion. Root invokes the child directly; nonroot uses existing `sudo -n -u root` numeric-drop pattern, requiring no UID65532 passwd entry. Other directories retain the direct assertion. The existing run helper bounds the child at90s and require_success suppresses child output.

All fixture bind sources still undergo existence and either nonempty-directory or positive-file-size checks, followed by read_only validation. No bind-source case is skipped, no generator directory mode is weakened, no chown/chmod/restorer change occurs, and no secret content or filenames are printed by the new child. Production code is untouched.

Final runner evidence: immutable complete 7388-tracked-file snapshot, UID1000 with passwordless sudo. Original baseline reproduced exactly two Chat/Space PermissionErrors; RED log `phase0-permission/red.log` SHA-256 `f22d09c52ff63b7e649577dc5b8051cbbfe99f76cba813ccc10827d9a9af0320` (author report). Reviewer independently read and hashed these final logs under `D:/Git/Voice/tmp/pr875-repair/phase0-permission/`:

- `focused-green.log`: 1/1 PASS,10.427s; SHA-256 `03552dfdcce379d7e84f0cd3abbcbae04ae330e317a61c013c5e80795fe1db82`.
- `full-verified.log`: 78 tests,48.716s, OK with one existing root-only cleanup skip under UID1000; service-UID fixture executed. SHA-256 `f45e5fbcab6fe216e6c24b14767d822aed04343404dbb7021b73d918feaca143`.
- `helper-negative.log`: empty service-owned directory rejected, nonempty accepted; owner/group65532 and mode0700 unchanged before/after. SHA-256 `4b7e1e134c7517fc91ff3ed0e0189d67a88feec7e05467221694e00d9b485549`.

An earlier full attempt used an incomplete source copy and is explicitly excluded. These are scoped offline fixture checks, not hosted acceptance or live service activation.
