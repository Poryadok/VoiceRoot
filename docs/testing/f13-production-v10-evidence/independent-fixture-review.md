# Independent Realtime bootstrap fixture review

Verdict: final review CLEAR for the exact candidate below, with independently inspected runner RED/GREEN logs.

- Reviewer: /root/role_admission (independent, read-only source review).
- Author: /root/bot_installation.
- Baseline: 9ef1a2b147b00e26a3d98b09ac01e75df1409702.
- Candidate: WT50, docker/postgres/compose-realtime-migrations_test.py.
- SHA-256: ae275579b5f0d5c1ef91542f17d581e1380f9a85b324f1f720d15031ab38c997 (independently checked).

The four hosted failures in run40133785610/job120991089752 occur before Realtime assertions: the host-shell fixture does not provide the mandatory installed Bot provisioning helper. Production Dockerfile.compose-db-init installs Python and copies that helper to the exact path; .dockerignore includes it, and Compose mounts the native Bot evidence volume. The observed error establishes an incomplete fixture boundary, not a production image packaging failure.

The candidate supplies explicit Bot and Space command adapters. Both validate exact invocation arguments and record ordered calls; the Bot adapter also supports exit 2, which the added negative asserts prevents Space invocation and Realtime migration. The copied production migration shell only replaces the image-installed Bot helper pathname. Realtime migration decisions remain the real shell implementation. Bootstrap still calls the copied shell through the existing installed-command relocation seam.

All original fresh/existing/repeated/dirty/nonempty-untracked assertions remain. The tests still reject force/baseline behavior on dirty or untracked Realtime state. Positive tests additionally require Bot then Space invocation, including repeat startup. Disposable fixed database environment removes ambient credential/config variation.

This scope matches docs/TESTING.md's stateful command-fixture contract. Adapters are explicitly not native custody simulations. Native Bot/Space provisioning semantics and actual Compose startup remain separately owned checks. No production source or permissions were changed. Reviewer ran no runtime tests.

## Final runtime evidence

Luna executed the Linux baseline and exact candidate overlay. The reviewer read and hashed the resulting logs under D:/Git/Voice/tmp/pr875-repair/realtime-bootstrap:

- red.log: SHA-256 bc775ac62710b9804d52d6a5ae8f219a6b8b720cc17c38f114afdbc9c83380eb. Baseline: all four tests fail (1.141s), each caused by the missing installed Bot helper, including dirty/untracked assertions that never reach the intended Realtime boundary.
- candidate-green.log: SHA-256 68d8250d0f3b2b4bf1f38bd012ba397c53656afe49c6e5fe36779bb15ff55e8e. Exact candidate: all five tests pass (2.188s), including the new upstream failure-propagation negative.

Root confirms candidate SHA unchanged and the production migration/bootstrap shells unchanged. This closes the bounded fixture regression; it does not assert live Compose startup or native provisioning acceptance.
