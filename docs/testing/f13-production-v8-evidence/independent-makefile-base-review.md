# Independent Makefile context and event-base contract review

Reviewer: f13_review. Read-only source review and Git object inspection; no test execution or full-producer acceptance.

## Exact reviewed source

Old Native source: e409cd05c40e857b61109513eb2154f968970c02.
Old Makefile blob: 686039d87ce15b835db99608c8921378bc271af8.
Old raw SHA256: 219afd734b2d767485b04709092b37ec99127f6c87d156e37fc31b50fc999a79.

Reviewed merged source: a8d5c2c (Git-resolved source used for this review).
Reviewed Makefile blob: a611a91b479fc51b5600ec293bc1dce6bff58f1f.
Reviewed raw SHA256: 3a461f577654391b6957c6ec5470e5c706dcc4e721f29a7d80751ee117c8a7ec.

This is not the raw master2c8 Makefile: that blob is d3f93bf8f4f890ae875a18254b0493bf5b3dd840, SHA256 c55526a1d765b212335a6777e39ebf6764f6011c00399ed268b681043a99138c. The merged version preserves PR875 checks.

## Source verdict: CLEAR within this context boundary

The old commands and prerequisites remain reachable across policy, runtime, staging and proofs targets. The release-policy prerequisite moves to policy. Minio and public-storage checks move without disappearing. The PR875 production, e2e manifest and rollout checks remain. Standalone shards receive PYTHONDONTWRITEBYTECODE. Default local ci-script-tests retains the full historical contract prerequisite; explicit ci-script-tests-pr exports VOICE_CONTRACT_PROFILE=current. A current-profile result is not a full-history result.

This changes test invocation and reachability. It does not grant Native runtime authority or add a Native projection path. The accepted successor shape changes only this exact context entry; all 99 projection entries, remaining 471 context entries, 481 compiled entries and 89 complete dependency-directory inventories must independently match the preceding authority. Those complete final-H equalities remain the record constructor's obligation, not an inference from this one-file review.

## Fixed new event-base contract: accepted in principle

The combined successor may bind the exact integration of predecessor ecde2e7b29d93dd01b233cd3128b373137aa6715 and master2c8bdf9e2f420b724575096beb4fcf5cb4ea00be, in that ordered-parent order, followed by the complete reviewed first-parent source chain. Final H, every commit/parent/tree in that chain, complete source delta and D6 inventory, and exact contribution evidence must be frozen and checked. Actual R must directly follow H with only named metadata/evidence changes. All predecessor records and evidence remain immutable; post-R execution remains restricted to the existing ten named carriers.

Current PR/master-push event qualification must bind the exact new event base2c8bdf9e2f420b724575096beb4fcf5cb4ea00be and actual ordered B/S/T/source-result identity. Historical base288e6a4fb963b3d5775f885e14ecfda1b603a4ff and historical698d4f064f032ea74abf3ede2017100b26c8ee69 are separate historical facts and must not be rewritten. No arbitrary-base, schedule or manual-event extension is accepted here.

Native context v4 must independently reconstruct v4 to v3 to v2 to v1, retaining the old loader predicates. A same-R Native record can be referenced by exact path/raw SHA without a self-commit hash. Production and corrected-F consumers must verify that exact new Native authority rather than bypass the old Makefile mismatch.

Final source H is ef3dd7f3888bbe2792a9dcec836b2773c4e77962. Independently read H:Makefile and verified its raw SHA equals the reviewed 3a461f577654391b6957c6ec5470e5c706dcc4e721f29a7d80751ee117c8a7ec. This artifact does not independently approve its other contribution files or the as-yet-unreviewed final R8 packet. Complete final-H closure checks remain required before binding this review. CODE, bounded execution and fresh full-producer terminal evidence remain separate.
