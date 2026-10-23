# ExecPlan: fresh production Native50 installer

## Purpose

Provision a new Space database through the actual immutable44,45,50 migrations and existing trusted transition APIs. Every Space deployment adapter uses the same production command. Unknown existing or dirty databases HOLD; this work does not activate providers or solve the full Ban budget.

## Context

Author Isaac-26-SH owns task NATIVE50-PRODUCTION-INSTALLER-26 and WT31 lease012f19165b50066ab6107e4af21905bf, branch feature/isaac26-native-production-installer, exact base e61025fec7b639bb048c30b398f02c637d07fb8e. Consumer Isaac-2 owns PR875/WT43 and integration/publication. Decision18 SHA29f3d7bba706d360e79fc6a9b92b3a262b832240ee44ef36921fa4a2b091b8e0; Native7 proposal SHA43f87df23d15f834a63ab1a702d2bf690d62407812a3f15627e523193f7b57fc. Sources: docs/OPERATIONS.md database ownership; docs/DEPLOYMENT.md migration-before-app contract; docs/TESTING.md Go/contract checks; migration README; existing TransitionNativeMembershipCatalog45/50 provenance contracts. Saved f21 causal report and current9a CI establish generic-up failure before tests.

## Scope

Exact new paths: space/cmd/native-migrate/{main.go,main_test.go}; space/internal/nativeinstaller/{runner.go,receipt.go,source_manifest.go,runner_test.go,runner_integration_test.go}; space/internal/store/{native_installer_source.go,native_installer_source_test.go}; space/Dockerfile.native-migrate (under src/backend).

Exact existing adapter hunks: docker/postgres/Dockerfile.compose-db-init builds the command; docker-compose.yml only compose-db-init build/source config; docker/postgres/initdb.d/01-init-databases.sh and scripts/staging/init-postgres-databases.sh remove only Space early creation; docker/postgres/ensure-compose-schema.sh excludes Space generic baseline and patch; docker/postgres/compose-migrate-dbs.sh branches Space before generic logic; scripts/dev/compose-migrate-all.sh delegates only Space; deploy/templates/migrate-space-db-job.yaml uses installer; scripts/staging/apply-migrate-jobs.sh and scripts/dev/migrations.py bind Space image/source and always run live verification instead of SQL-only reuse; scripts/staging/apply-migrate-jobs_test.sh plus new scripts/dev/native-space-installer_test.py qualify those branches. Docs limited to migration README, DEPLOYMENT and this plan.

No edits to migrations, transitions, existing pins/line107, providers/application main, audit fixture, rowjson or other DB routes. Additional build wiring needs exact extension ACK before edits. Source manifest includes the complete selected ordered SQL inventory and compiled qualifier/transition identity. Runtime files must match trusted compiled inventory; no caller hash authority.

## Milestones

- [x] Exact base, ownership/allowlist and resource checkpoint ACKed by2/root.
- [x] Preserve generic-up RED; tests first for fresh-only command, source binding and adapter routing.
- [x] Implement installer, narrow read-only Store verifier, real image and all Space adapters.
- [x] Real PG16 then18 fresh chain and negative/interrupt/verify controls; existing adapter/render contracts.
- [ ] Freeze exact source/proof for independent10; resolve findings before normal own-branch commit/push and source offer to2.

## Detailed Steps

1. Read real transition/full verifier and adapter consumers. Ask Native7 read-only for boundary review; one writer26.
2. Add acceptance tests for generic current Space routing defect, exact ordered phases/receipt linkage, source substitution and unknown/dirty refusal. Capture meaningful behavior RED, not missing-import/compile RED.
3. Implement a compiled fixed SQL inventory/source manifest and pinned migrate binary identity supplied by trusted build. CLI reads admin credential from environment without logging it, owns CREATE DATABASE for absent target, holds cooperating-installer advisory lock, verifies administrative identity/session/custody across phases, emits ordered non-secret events and refuses lost window/cross-process resume.
4. Actual migrate goto44, full compiled verifier plus M44-created seal; goto45 and existing Transition45, matching old seal/identity; goto50 and existing Transition50, matching trusted45; final full clean50 verifier. Never write epoch directly. Verify mode is read-only inspection, not historical adoption; adapters must retain original install evidence.
5. Route all Space adapters before generic creation/force/up/legacy patch. Staging/DEV require an explicitly source-bound immutable installer image selected by the release controller; do not invent/publish a registry tag.
6. Build one real production image and run sequential private PostgreSQL16/18 under2ACK budgets. No host ports, Docker socket, privileges, shared networks/data or DEV/STG. Capture exact SQL/source/executable/image/target identities and logs. Existing source guards remain unchanged.
7. Run focused unit/contract tests and bounded affected Go checks, shell syntax/render contracts, preserve non-Space paths. Freeze manifest and independent10 source/proof review. Stop at coherent reviewed own-source delivery;2 alone mutates the PR.

## Validation

Real generic-up RED is reproduced with the actual selected migrations. GREEN requires actual fresh creation, real migrate44/full seal facts, Transition45 receipt, real migrate50/Transition50 line107 and fresh full50 qualification on PG16/18. Negative controls cover precreated empty/unknown44/45/dirty50, inventory/source mismatch, wrong role/seal/identity, writer/frame/competitor, interruption and verify-only drift/replacement. Adapter tests prove Space never reaches force/rawup/legacy patch or SQL-only completed-job skip and non-Space behavior stays intact. Any unavailable required evidence remains an explicit gap, not PASS.

## Progress

Pre-write checkpoint completed. Architecture/source hashes independently matched. Native7 supplied read-only API facts; author26 remained the sole writer. Consumer2 additionally approved exactly two Dockerfile-specific deny-by-default ignore files: `src/backend/space/Dockerfile.native-migrate.dockerignore` and `docker/postgres/Dockerfile.compose-db-init.dockerignore`. Both root-context images built successfully with only their required inputs, excluding `.git`, `.treehouse`, `tmp` and unrelated checkout data.

Behavior RED: the original Space DEV renderer used generic migrate up and the original Compose adapter had no early Native branch. The actual pinned driver with all85 unchanged SQL files failed fresh generic up at M50 because trusted45 did not exist. Initial receipt controls also exposed acceptance of mismatched target/parent seal/non50 facts; linked completion validation made those controls pass.

Actual source-bound PG16 tagged tests passed in65.715s: fresh creation/full44/real Transition45/real migrate50/Transition50/full50, retained verification, existing-target refusal, and evidence-write failure at all eight committed output boundaries. Every interrupted run remained HOLD on retry and its partial events could not become completion evidence. The fixture checked the created database OID before its own cleanup. A real PG16 metadata drift check also failed closed. The later source refinements retained actual creation identity and made the inspection transaction read-only; those refinements were included in the tagged run and production image proof.

The pre-review runtime-qualified production image `sha256:8bb3b79422fecf6b0463923523e59a7e59a8a1960f32675893d74d2dd6ec0d53` and Compose-init image `sha256:ae0886f73a89eed09a94ee08e5f308b0acf419b6ca97267c465248021df05096` contain byte-identical installer SHA256 `bcdf4b9e62febc35599a9bd13d5721b8db915744b8b3414248cd86822c60f151` and pinned migrate SHA256 `352d0e23fe687a567706756b38cf6adaf2075c2de17c2df4022ec548d8feaf27`. These are private proof images, not published deployment references.

The single final PG18 run used the actual Compose-init entrypoint, schema adapters, compiled installer and all85 exact Space SQL files. Other service migration directories were intentionally absent from this component fixture. Fresh Space0→44→45→50 passed in5.234s and emitted all eight ordered events, genuine changed transition receipts and final clean50. Retained evidence verification passed in1.782s; existing50 without evidence and actual native catalog function metadata drift each returned HOLD. Final-image source-selection, missing SQL inventory and substituted driver controls failed before any DB connection. Focused Go short tests and vet passed in26.188s; existing staging/render contracts passed under Node24/Python3.11, and DEV/native adapter contracts passed. No full Compose, hosted CI, real Kubernetes deployment or provider acceptance is claimed.

All owned PG16/PG18/client containers were removed with their anonymous volumes, and the owned internal network was removed; final name-filter readback was empty. No live runtime resource remains. Canonical logs, exact command/result records and resource inspection/cleanup receipts are under `tmp/swarm/isaac-26-native-installer` in the main checkout. Independent source/proof review is the next gate before own-branch publication.

Independent10 review of frozen tree955e1342 found P2 receipt-limit truncation and P1 keyword TLS/query target ambiguity. The successor rejects over-limit evidence explicitly and normalizes accepted URI/PG-environment inputs once before mutation; admin and child now share explicit authority/TLS with fixed database paths. Unsupported keywords/service indirection/options and duplicate/encoded connection selectors reject. Consumer2 approved exactly one additional `space/go.mod` requirement for test-only `github.com/lib/pq v1.10.9`; existing sums remain unchanged. Parser-backed tables use actual pgx and pinned pq parsers, including verified TLS with a generated CA, weaker ambient TLS, encoded/duplicate keys and foreign database selectors. Focused Linux-only cmd/nativeinstaller short tests plus vet, including the complete parser table, passed in10.312s. Successor production image `sha256:1d899caf68070ed5c5af9f7148fe0b17412f6224a6db330679a759ccc7c5dbbb` and Compose-init image `sha256:10c86dca17c50b6af73b5cbca17c3d87335f99ff83c7f56b4239b5fb317a5f3f` built successfully and contain identical corrected installer SHA256 `f5a1acdfbdac18d4096b5f06fd52648f253747ca554c08af7f6dbc6dd347416f`; the pinned migrate remains unchanged. Exact successor terminal/image bindings are retained in the canonical proof packet. No additional PostgreSQL run is performed: the earlier runtime proof remains bound to its historical image/tree, and the bounded reader/input changes receive parser/unit/build qualification.

One attempted host Windows `go test` was a boundary error. Its already-terminal failure is retained as excluded evidence; only the subsequent private Linux regression receipts count. There was no direct test.exe command, no surviving nativeinstaller.test.exe process in readback and no global process kill. All subsequent tests use the bounded private Linux source-check, without PostgreSQL.

## Decisions

Fresh CREATE DATABASE is the provenance producer; endpoint emptiness never grants adoption. Advisory locks serialize cooperating installers but do not claim to fence hostile privileged DDL; owning controller maintains the maintenance window. Successful live50 inspection is labeled verification only and cannot fabricate historical receipts. Automatic crash recovery is excluded.

Compose/dev repeat custody uses the owning local controller's original successful JSON-lines artifact retained with restrictive permissions and its observed image identity; the compiled verifier compares the actual executable/source/SQL/driver and target identities before live read-only verification. The Compose init reader mounts that original artifact read-only. Direct first-boot Compose emits Docker logs; when that controller has not retained original completion evidence, a later boot HOLDs rather than adopting existing50. Kubernetes DEV/staging retain original completed Job UID, pod owner UID and actual platform image digest together with original completion logs in an immutable evidence ConfigMap; SQL-only old Jobs cannot be adopted. No signing key, receipt service or database marker is introduced as a new trust authority.

## Risks And Follow-Ups

Mandatory hosted CI, original full107/307/Ban budgets, actual providers/all-writer/activation/P3 and deployment acceptance remain consumer gates. Build publication/provenance is a release-owner dependency; no deployment is performed here. Native fullsource qualification may expose an existing immutable transition/pin incompatibility; preserve its failure and stop dependent GREEN rather than weaken it.
