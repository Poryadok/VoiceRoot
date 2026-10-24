# Independent F13 production v5 source review

Reviewer: /root/f13_review. Date: 2026-10-24. Result: SOURCE ACCEPT for the exact bounded successor below; final K pins/code execution and actual producer completion remain separate.

## Exact source binding

Reviewed H `1038561dcff8413539da28355ee6a532d54fbcba`, tree `439963aab6c077ef65182b91d92355e041067494`, with sole parent `38330e20dfb23bb8be2298ba3e70182e01414481`. Independently reconstructed complete Git trees: identical path sets and identical entries outside exactly these two existing mode-100644 test files:

- `src/backend/messaging/internal/messageevents/jetstream_test.go`: after blob `fee6696610c4ca3f0e79e389914b8905c4f7274e`, 17346 bytes, SHA256 `827a06e8a46a1fe4e517b3b356c3293596ef28f874c55c96c44724c23e780fdc`.
- `src/backend/moderation/internal/grpcsvc/automod_scan_outcomes_test.go`: after blob `d547529e2d194457da0a2ae5445f1d4ef1bdc59c`, 4983 bytes, SHA256 `5fbc9e9e92d193045605018056d799f3653543d7b9fa8d017efd5fc5042cb4bc`.

Both before/after entries, lengths and SHA256 values, exact raw delta hash `d091d7a188a5dbcf6739fd06181c77e880a7ee8ba178a82b5da541a959bcdba3`, and all 1000 complete D6-to-H inventory entries were independently reconstructed and matched the draft packet. Draft record SHA256 was `5390fa3d82a98dbf5990bcb6cb8f146c19e209f6fc8c0926ae58b623bb270c24`; adding this review and changing status requires a newly bound final record hash.

## Semantic and evidence review

Messaging adds six subscriber FlushTimeout barriers before separate publisher creation. Existing delivery, payload and identity assertions remain. Moderation uses synctest virtual time while retaining deadline bounds, child cancellation, parent isolation, business result, exact terminal metric and recorder-count assertions; its additional expired case synchronizes on parent.Done(). Neither change edits production policy or production deadlines.

Independently verified all 17 draft artifact references (inventory plus 16 evidence objects), and byte-for-byte equality of all 15 transported original evidence files with their original local paths and declared lengths. Read the receipts, semantic review and local terminal logs. Messaging receipt `2bec02c840afb2deedeab36b0e295c6d9c00768a182fc18715e7c40739715d8d` binds the exact candidate file and passing targeted count100, package short count20, and targeted race count10. Its pristine local baseline also passed; the hosted timeout is not falsely presented as a locally reproduced RED. Moderation receipt `6e03e895cba6ba9e4e713e84525747e4f7fc28da86cc9dc04e8eefd93d913517` binds the exact candidate, targeted race count500, package short/race and vet. Its controlled expiration overlay is explicitly distinct from an unmodified-source RED. Independent test semantic review SHA256 `afdacf436990abe1c0996be89e3107edd20a870dc8e81cf71647bcc1c79cdf55` agrees with the inspected source diff.

## Authority boundary

Accept only this fixed two-file source successor. Preserve the immutable v4 record `af6dd22b23b9bfad060ef43cf7eb8703cabd4e07` and all earlier records, archive facts and evidence. Retain Native v3 record `f87ee933aa1f599284e1932e44bf32b4e2af48ba`, source `e409cd05c40e857b61109513eb2154f968970c02`, SHA256 `4ed3acd7cecfef97d03724115e57bf418600bd8e3210d992fd7ed1a325de8a67`; these test changes do not introduce Native projection authority. Full source equality outside the two named files preserves the prior protected source entries. Current Native authority still must be independently qualified against actual B/S/T.

The reviewed draft requires direct H-to-R metadata/evidence intake, complete inventory and evidence binding, original contribution gates, unchanged ten-path executable-carrier ceiling, and explicit v5 handling in materialization and corrected-F. No wildcard test permission, future same-path permission, historical runtime upgrade, or reused terminal success is accepted. No tests or services were run by this reviewer. Component receipts do not establish hosted acceptance or complete F13 producer/router success. The older 38330 execution is diagnostic evidence for that older source only.
