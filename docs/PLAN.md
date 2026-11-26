# Voice — план продукта и исполнения

> Каталог заявленных возможностей — [FEATURES.md](FEATURES.md), канон поведения — `docs/features/*`, архитектуры — [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md). Активный scope, порядок и release gates задаёт этот файл.

Срез проверен 2026-09-04 на commit `2c1a908a29e89bcdc077dc590fbb8b4db9ae14c5`. Он описывает состояние этого commit, а не обещание о будущем `master`.

## Стратегия

Voice сначала должен стать убедительным бесплатным продуктом для небольшой группы: Web → Windows, вход по email или как гость, личные и групповые чаты, invite-only Space, голос и полный цикл поиска команды. Неполные возможности скрываются флагом или не выводятся в основной сценарий.

Default admission для внешнего alpha — server-signed одноразовый invite с expiry/revoke и fail-closed configurable cohort cap; email и guest registration требуют invite. Владелец перед `G1` принимает только предложенный размер когорты, а не проектирует механизм.

Приоритет задают несколько законченных пользовательских потоков, а не сумма строк в TODO. Главный дифференциатор — матчмейкинг, связанный с голосовым ростером, временным чатом команды и post-match историей. Billing, mobile, публичный каталог, bots и визуальная полировка не должны задерживать проверку этого ядра.

Работа идёт одновременно по двум независимым веткам:

- `H` — короткие действия и решения владельца, которые нельзя честно заменить кодом агента;
- `A` — автономные продуктовые этапы. Они должны выполняться без ожидания `H` на local/compose, fake или sandbox окружении.

Зависимость от владельца ставится на release gate, а не на весь engineering milestone. Если задача смешанная, агент сначала доводит код, тесты, конфиг и инструкцию до состояния «нужен один конкретный ввод или один acceptance run».

Текущий WIP плана:

| Состояние | Milestone | Правило |
|---|---|---|
| Active | `A1` | Единственный активный milestone последовательности `A`. Fleet дробит его на независимые service/client/contract/verification задачи и исполняет их максимально параллельно в отдельных worktree. |
| Parallel active | Game integrations | Отдельно согласованный единый спринт в `codex/game-sdk-federation-docs`; собственные worktree, integration queue и rollback point. 2026-10-02 владелец разрешил merge уже проверенных implementation checkpoints в `master` через PR с зелёным CI. Полная приёмка спринта, staging rollout и live provider/device tests остаются отдельными открытыми gates. |
| Next | `A2` | Входит в WIP только после интеграции и полного vertical DoD `A1`. |
| Parallel active | `A4` | Evidence 2026-11-20: epoch `A4` holds in_progress/review/merging STG-ceremony cards; LAN root ceremony = owner's euid-0 decision after root cause resolved — media-target authority-family drift 23 vs 24 (gameintegration era) fails closed every idle route (`docs/audits/2026-11-20-lan-retirement-authority-drift.md`, card `OPS-STG-MEDIA-AUTHORITY-FAMILY-DRIFT-23-24-V1`); do NOT rerun the frozen `--install --upgrade-idle-installed-cohort-v1`, unblock = new reviewed baseline capture (`VERIFIED_GATED_TARGET_BASELINE`) is owner's decision — mapped 2026-11-21 (`docs/audits/2026-11-21-lan-reviewed-baseline-successor.md`, card `OPS-STG-REVIEWED-BASELINE-SUCCESSOR-V1`): no new repo tooling owed, sole repo-side precondition = first code-touching master merge fires gated `staging-stack-lock` + promotes 24 images at the new head (3 UIV3 Flutter PRs in flight); then owner euid-0 stages B–C (fresh capture bind → baseline terminal → cohort closure); however 2026-11-21 satisfiability audit shows zero catalog-era `_ci` green full-graph master pushes, so before any LAN baseline capture the full-validation debt family (`ci-gate`, `full-validation-aggregation`, `local-ci-parity`, Space/Role proof debt) must be fixed first (`docs/audits/2026-11-21-lan-capture-source-green-push-satisfiability.md`, TT `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`); 2026-11-22 debt-family root causes mapped (`docs/audits/2026-11-22-ci-debt-family-root-causes.md`): class A = hosted runner image bump (20261101.186→20261111.196) rolled unpinned Postgres fixture under Space/Role M50/M51 ban-authority proofs — fixture/toolchain pin PR in flight (card `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT`, claimed `Codex-CI-Debt-FixA`), class B = native50 transition chain red since 2026-11-11 needs tripping-branch proof before product fix (card `CI-NATIVE50-INSTALLER-TRANSITION-CHAIN` available), `ci-gate`/`full-validation-aggregation` leaf propagators auto-green with leaves, a1/t31 hosted family retry-class monitor-only; capture-eligible all-green full-graph master push = next target after leaves flip; LAN ceremony stays debt-gated |
| Queued | `A3`, `A5–A7` | Не получают code WIP до закрытия предыдущего milestone; разрешены лишь чтение канона и подготовка, непосредственно разблокирующая Active. |

`H`-задачи не занимают слот `A`: владелец и агенты могут параллельно готовить access, decisions и activation evidence. По отдельному решению владельца игровой спринт ниже выполняется параллельно `A1` в собственной ветке и integration queue. Это не меняет порядок `A2–A7`.

> 2026-11-19 TT-fleet audit (`docs/audits/2026-11-19-tt-fleet-audit.md`, tracker rev 23202): WIP plan above holds — `A1` is the only Active, GAME is Parallel active (45/81 TT cards `done`), `A3–A7` Queued. Drift flag, owner decision: TT epoch `A4` already has `BE-019` in_progress (assignee astrie) plus 8 review / 2 merging / 11 `stg_ready_for_deploy` while PLAN keeps A4 Queued. Audit made no status transitions.
> 2026-11-20 TT-fleet+CI actuals (`docs/audits/2026-11-19-tt-fleet-audit.md + TT rev 2026-11-20 actuals`, `docs/audits/2026-11-20-night-ci-dispatch-samples.md`, rev/claim `b73c1c98…` STAGING gen14/rev55): STG ceremony в flight — DEV delivered, GHCR publication 24/24 at `:d4af0992`, package cards `stg_ready_for_deploy`, LAN installed-cohort root `waiting_for_human` (root cause 2026-11-20 = authority-family drift 23/24 per `docs/audits/2026-11-20-lan-retirement-authority-drift.md`: do-not-rerun, unblock is owner's reviewed-baseline decision), new open CI class card `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120` available — 1 red sample vs 2 fresh green full-profile samples.
> 2026-11-21 TT-fleet audit (`docs/audits/2026-11-21-tt-fleet-audit.md`, TT rev 23536, tip 7d7808a7): WIP plan holds — `A1` Active, GAME Parallel active, `A2` Next, `A4` Parallel active (TT epoch `A4` title still `Queued` — reconciled via `epoch update A4`, no card transitions), `A3/A5–A7` Queued. STAGING fence gen 14/rev 57 (LAN ceremony attempt 2026-11-21 failed closed, recorded in fence record), 86 `stg_ready_for_deploy` HOLD behind the LAN root owner decision; CI contract drift CLEAN at tip (66/66); GHCR gate 24/24 at :d4af0992. 0 new cards — actionable gaps all on existing cards/owner decisions.
> 2026-11-21 reviewed-baseline successor research (`docs/audits/2026-11-21-lan-reviewed-baseline-successor.md`, card `OPS-STG-REVIEWED-BASELINE-SUCCESSOR-V1`): reviewed route needs no new verifier tooling — tip data model already accepts a fresh 24-name baseline authority (`operator_retirement.py:1360` baseline branch; `:1228` dynamic); no capture binds today (tip runs lack `staging-stack-lock` artifact, `d4af0992` lock fails ci.yml-blob equivalence, retention 14d); first code-touching master merge (UIV3 Flutter PRs incoming) fires the gated job and makes capture bindable; owner euid-0 stages A–C staged; STG `:d4af0992` HOLD persists until cohort closure.
> 2026-11-22 CI debt family root causes (`docs/audits/2026-11-22-ci-debt-family-root-causes.md`): full-validation debt family dekomposed — class A = hosted runner image bump (20261101.186→20261111.196) rolled unpinned Postgres fixture under Space/Role M50/M51 ban-authority proofs (green→red boundary run pair 43205490394/43389396732, Space/Role Go+migrations byte-identical, compare changed only 2 CI-script files) — fixture/toolchain pin PR in flight (card `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT` claimed by `Codex-CI-Debt-FixA`, in_progress), class B = native50 transition chain red since 2026-11-11 with tripping-branch proof owed before any product fix (card `CI-NATIVE50-INSTALLER-TRANSITION-CHAIN` available), `ci-gate`/`full-validation-aggregation` leaf propagators auto-green when A/B flip (gap `unresolved=14`), a1/t31 hosted family = retry-class monitor-only; night authoritative verdict #6090 (run 44850194289, head `7d7808a7`): 133 jobs 10 reds all inside documented family, zero new classes (TT c4 on `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`); tonight dispatch #6105 (run 44971137142) at tip `230166b84bc8` in_progress. Persistent reds predate !1190/!1191 — base-policy BLOCKED of code PRs is debt-caused, not merge-caused. LAN capture prerequisite unchanged: all-green full-graph master push after leaves flip; STG `:d4af0992` HOLD persists.
> 2026-11-22 class-A correction (`docs/audits/2026-11-22-ci-debt-family-root-causes.md` Korrekcia section; TT `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT` c2/c3, `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` c6/e8): pin route for class A DISPROVEN empirically — class-A fix lane delivered NO PR (7 Postgres configs RED at testcontainer position, compose-db-init position green = pin-16 `972518e6`); runner bump absent from job logs (both boundary jobs log 20261101.186); ban-catalog pins re-recorded old→new. Class A is OWNER-gated: policy decision (a) per-position pins / (b) attver-aware normalization / (c) exclude volatile `att_options` owed in `src/backend/space/internal/store/account_ban_catalog.go`. Capture chain: owner decision → class-A pins PR → class-B native50 chain (proof worker in flight, card claimed `Codex-CI-Debt-FixB`) → first all-green full-graph master push = capture target; STG `:d4af0992` + fence `b73c1c98` gen14/rev57 HOLD persists.
> 2026-11-22 TT-fleet audit (`docs/audits/2026-11-22-tt-fleet-audit.md`, TT rev 23588, tip `eb341dd8` = PR #1190 merge): WIP plan holds — `A1` Active, GAME Parallel active, `A2` Next, `A4` Parallel active (TT epoch `A4` title already reconciled 2026-11-21 — no epoch action), `A3/A5–A7` Queued. Census vs 2026-11-21: available 900 (−4), in_progress 56 (+6: UIV3 claims + `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` created in_progress), review 45, merging 7, `stg_ready_for_deploy` 86 HOLD, wfh 3 owners recorded, done 77, blocked 94, legacy 0; NEW actionable finding: `GI-T58` ownerless `in_progress` (GAME T58, stale since 2026-11-01 erasure-review refutation; prior audit claimed 0 ownerless) → new card `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1`. Open PRs `#1191` (UIV3 worker card, BLOCKED debt-red) + `#1192` (other session's LAN-capture docs PR, owned-enrollment-only infra red). CI contract drift CLEAN at fresh tip (checklist covers 66/66 live ci.yml jobs, verifier 34 names ⊆ live, zero adds/removals since `bc9e4909`); STAGING fence gen 14/rev 57 HELD unchanged; GHCR gate 24/24 at both `:d4af0992` (fence candidate 7/7 exact digests) and newest product pin `:eb341dd8` (tip push 44934578838 fires gated `staging-stack-lock` + promotes 24 images data-preserving — capture-source all-green precondition still debt-red, root gate unchanged). 1 new card, 10 receipt comments.
> 2026-11-22 TT-fleet inventory audit (`docs/audits/2026-11-22-tt-fleet-inventory-audit.md`, TT rev 23622, fence read rev 23634, tip `85bbba48` = PR #1196 docs merge): WIP plan holds — `A1` Active (16 in_progress), GAME Parallel active (2 in_progress + `GI-T58` ownerless persists → comment on existing card `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1`), `A2` Next (8), `A4` Parallel active (epoch title already reconciled 2026-11-21 — no epoch action), `A3/A5–A7` Queued, UI-V3 28 in_progress under in-flight claims. Census 1290 (available 898, review 45, merging 7, stg_ready 86 HOLD, wfh 3 owners recorded, done 77, blocked 94, dev_ready 17, dev_test 2, legacy 0; delta vs 23588 status-math only). Merging pool 7 develop-era shas re-proven NOT-IN master (claims honest, `#1101`/`#1112` CLOSED). CI contract drift CLEAN (66/66, 0 adds/removals since `bc9e4909`); open UIV3 PRs `#1191/#1193/#1195` — required sets green except registered debt/`owned-enrollment` classes; master protection single gate `ci-gate` strict:false. GHCR digest gate 24/24 at product pin `:eb341dd8` AND fence pin `:d4af0992`, digests byte-stable vs prior records and HELD STAGING candidate. Fences: STAGING gen14/rev57 HELD (LAN fail-closed recorded, wait `ready:false`), DEV free gen96/rev375 `DEV-CHFIX-62B6FCA-20261121-V14` delivered. 0 new cards, 16 receipt comments, docs-only PR merged.
> 2026-11-23 TT-fleet inventory audit (`docs/audits/2026-11-23-openchamber-fleet-inventory-audit.md`, TT rev 23672, capture tip `0d43054a` = PR #1199 merge, delivery base `2285c041`): WIP plan holds — `A1` Active, GAME Parallel active, `A2` Next, `A4` Parallel active (TT epoch `A4` title already reconciled 2026-11-21 — no epoch action), `A3/A5–A7` Queued. Census 1290 (available 897, in_progress 62, review 45, merging 7, stg_ready 86 HOLD, wfh 3 owners recorded, done 77, blocked 94, dev_ready 17, dev_test 2, legacy 0; delta vs rev 23622 status-math only). Merging pool 7/7 PRs MERGED re-validated with fresh gh mergedAt → receipt comment on existing card `OPS-MERGING-CARDS-STAGE-RECONCILE-20261120` (available), no duplicate. 1 NEW actionable finding: 76/77 `done` cards carry zero structured records (test/accept/recheck/impact/deployment events empty; records subsystem live only for OPS-CLICKHOUSE-OOM pair + fences) → new card `OPS-DONE-CARDS-EVENTLESS-RECEIPTS-20261123` (available, P2; 76/77 kept `done`, no fabricated receipts, reopen only via `task invalidate` with evidence). Fences: DEV free op 52 PASS `DEV-CHFIX-62B6FCA-20261121-V14` (`stale:false`), STAGING gen14/rev57 HELD claim `b73c1c98` (`stale:true` — LAN ceremony 2026-11-21 FAILED CLOSED `controller.Blocked operator_retirement_binding_invalid`, chain at tip `fd97cec7`, zero cluster mutation, fence `next_action` = human sudo inspector on node). CI contract drift NET ZERO at both anchors (66/66 checklist↔ci.yml, 0 `.github` diff `0d43054a..2285c041`); full-validation debt family unchanged (registered in `docs/todo/ci.md`; last recorded full-graph master push 44934578838 at tip `eb341dd8` red at `ci-gate` + registered siblings — today's tip push not separately enumerated, limitation recorded). GHCR digest gate PASS 24/24 at package pin `:d4af0992` (full 23+renderer table in audit file, 7 changed tag-pin exact vs HELD candidate, 16 content-address resolves candidate==observed drift-0, renderer present, negative sanity probe clean) — STG rollout READY at package pin, rollout itself honest HOLD (all-green full-graph master push precondition still absent; PRs #1193/#1195/#1197 merged during the pass). Actions: 1 TT comment + 1 new card, docs-only PR merged.
> 2026-11-24 CI authoritative full-profile gate verdict (agent `CI-Gate-Verdict-20261124`, model qwen3.8-flash-next-coder-iq1_m, `docs/audits/2026-11-24-ci-gate.md`): dispatch run #6141 (`workflow_dispatch`, `profile=full`, publish inputs EMPTY — publication не принадлежит gate dispatch) на head `6f652d06` — `133 jobs = 111 success / 12 skipped / 10 failure`, verdict from FULL graph per verdict reading rule; 9 reds inside documented debt/flake families + propagators (`voice-history-audit` v7 ceremony not run, Space/Role proof debt `backend-go (role)`/`backend-go-integration* (space)`, `native50`, `role-space-receipt-integration`, `compose-e2e`/`local-ci-parity`/`full-validation-aggregation` `unresolved=14`; `ci-gate` SKIPPED не red); exactly 1 NEW UNREGISTERED class: `backend-auth-image` job `138112232518` = external Maven Central HTTP 429 throttle in Auth image PR-verify Docker build (`RUN mvn … dependency:go-offline` → `[FATAL] Non-resolvable parent POM spring-boot-starter-parent:pom:3.5.0 … status code: 429` → buildx exit 1) — NOT `auth-java` product class (retired green 2026-11-20, siblings green same run); GHCR digest gate at head NOT satisfied 2/24 (`web` + `nats-hub-config-renderer`; 24/24 persists at `:d4af0992` HELD STAGING и `:eb341dd8`, zero pins touched); green re-runs for receipts: matchmaking trio, gameintegration trio + controlledgame twins, `admin`, a1 pair, t31/compose-https pair; `owned-enrollment` did not run; actions: registry entry in `docs/todo/ci.md`; TT card `CI-BACKEND-AUTH-IMAGE-MAVEN-429-20261124` + 3 receipt comments NOT executed — TT backend at `192.168.0.109:30990` returned `internal_error` on all graph routes (3 retries), no fabricated receipts, owed at healthy TT contact; no product code/cluster changes, no retries, no history audits; docs-only PR merged.

> 2026-11-23 class-A landing (lane `codex/ci-space-ban-attver-normalization-20261123`, `docs/todo/ci.md` residual-debt closure): OWNER decision (b) attver-aware normalization applied to `src/backend/space/internal/store/account_ban_catalog.go` — structure-digest projection `to_jsonb(a) - attrelid - attoptions` normalizes strictly-historical attver/counter-equivalent fields (incl. rebase clause) OUT of the hash; semantic fields (attnum contiguity, defaults, constraints, ACLs, triggers) stay in hash; extra accepted pins forbidden; pins re-recorded ONLY from db-init source-bound position: PG16 `178743ba429c368bb1690d53162689b943fc8c095d9f73a7d24f02376d7ed34c`, PG18 `685105e033283665f440de9cb4bc149c630163c4074b8be5a521e53b418a63a8` (integration test untouched, fail-closed kept). Post-edit local harness graphs both GREEN with r2 image (`SOURCE_REVISION=6dd4506e…`): PG18 `compose-db-init` dbinit exit 0 run_id `215eea24-68c0-425b-b978-cd505cc73521`, receipt chain created→migrated44→trusted44→migrated45→trusted45→migrated50→trusted50 (tx1318)→transition51 `native_catalog_rowjson_51` (tx1319)→final v51 seal `9291f78a…`, "compose migrate OK"; PG16 run_id `317e1183-f01c-40fd-9bb8-e2087015c4bd`, exit 0, final v51 seal `5f281a6b…`. Final-position (schema-51) normalized digests EQUAL pins and position-invariant (P0≡P1 across LOCK/ROLLBACK): PG16 `178743ba…` @PGVER 160016, PG18 `685105e0…` @PGVER 180007; attver stamp census always 51 attoptions-only fragments per fixture, 0 in normalized projection; raw cur digest is position-variant (PG18 `7e5e6df8…` final ≠ `6d4c1d6c…` death) — why normalization was mandatory. class-B proof merged in !1201 `47bdf3f5` — leaf auto-greens when class-A flips. Capture chain now unblocked: class-A PR merged → first all-green full-graph master push = LAN capture target; STG `:d4af0992` + fence `b73c1c98` gen14/rev57 HOLD persists.
> 2026-11-24 class-3 consumer-lease batching (same lane `codex/ci-space-ban-attver-normalization-20261123`, PR #1207): Ban-phase per-profile round-trips collapsed at the class-A landing position — member-authority + qualification reads batched to ONE consistent DB-snapshot read per relation (`profile_id = ANY($n[])`), pending-admission retirement = one batched SELECT replicating `FindCurrent` obstruction shape + unchanged per-entry `invalidateMemberAdmission` writes, qualification INSERT verification = one batched SELECT reusing the identical per-entry validation core (absent/duplicate row or field mismatch => `ErrAccountBanUnavailable`, account via `uuid.Parse` fail-closed, duplicate rows fail-closed on raw `pgx.ErrTooManyRows`), suppression INSERT / roster revocation UPDATE / timeouts DELETE now three whole-cohort statements (first three-`;`-bundled single `tx.Exec` rejected live SQLSTATE 42601 — `tx.Exec` prepares single statements; cohort `ANY($2::uuid[])` form proven by the existing `DELETE FROM space_members` pattern). 5s/2s observation leases, per-entry freshness semantics and per-entry `saveEffectiveHead` SHOW+CAS-write+clock revalidation byte-UNCHANGED (hot `DELETE FROM space_members` cohort statement 2.53s warm = irreducible server trigger cascade). Gate: Retry50 marked pair mirror `_f` 5.4677969s FAIL → 4.9469441s PASS (`error=<nil>`, 53ms margin); full store suite green 992.718s. Local fresh-consumer proof abandoned — mirror `_g`/`_h` fresh chains share the container PG heap (band 4.95–5.81s identical code, page-pressure inflation), hosted nsi `TestAccountBanOver100Cohort50` on the pushed head is the authoritative gate; if hosted stays red, iterate (next levers: 104 qualification INSERTs → one cohort INSERT VALUES batch). class-B leaf (`!1201 47bdf3f5`) auto-greens when class-A flips; capture chain: class-A merged with hosted nsi + `ci-gate` green at head → first all-green full-graph master push = LAN capture target; STG `:d4af0992` + fence `b73c1c98` gen14/rev57 HOLD persists.
> 2026-11-24 class-A CI verdict + fleet snapshot (`docs/audits/2026-11-24-ci-classA-verdict-audit.md`, landed via PR #1211 merge `a030f0b5`; index/WIP rows added retroactively by the 2026-11-25 fleet audit pass): class-A M50 account-ban structure digest normalization LANED via PR #1207 (merge `a88a922d` = CI-scripts-only delta, zero image churn, hosted nsi green, `ci-gate` green on the push); class-B leaf green in hosted nsi; GHCR `:d4af0992` 24/24 re-verified vs candidate maps; STAGING `:eb341dd8` gate complete, target stays `:d4af0992` HOLD; capture-source chain unblocked at class-A.
> 2026-11-25 TT-fleet inventory audit (`docs/audits/2026-11-25-tt-fleet-inventory-audit.md`, agent OpenChamber-Fleet-Inventory-20261125, worktree `tmp/tt-fleet-inventory-audit-20261125`, tip `a88a922d` = PR #1207 merge): **TT backend UNREACHABLE** (`connection_error` on brief/graph/task-list/epoch — endpoint down, not the 11-24 `internal_error` class) → TT census/fences/epoch/wfh/claims axes BLOCKED fresh, carried from landed 11-22/11-23/11-24 docs; PR reality: single open PR #1208 (2026-11-24 OpenChamber inventory audit — checks green `ci-gate SUCCESS`, unmerged; its README/PLAN anchors clash post-#1209/#1210 → landing is owner's re-resolution, report-only); docs drift: classA audit file on master without index/WIP rows → FIXED this pass; CI drift CLEAN (66↔66↔66 live/verifier/checklist↔tier table, `ci.yml`/verifier/path-filters/gate-policy byte-identical `bc9e4909..a88a922d`; tip push run #6151 in progress, #6150 at `4d360801` last completed full-graph verdict reds = registered debt families + `backend-auth-image` Maven-429 registry entry; #6137/#6135 supersede-cancel report-only); GHCR gate HOLD `:d4af0992`: fence candidate 7/7+renderer byte-exact fresh, newest all-24-complete pin `:eb341dd8` byte-equal vs recorded 11-22 map, NO tags `:7382c731..:a88a922d` (#1202–#1207 gated publish did not fire — debt-red gate, families carded `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT`/`CI-NATIVE50-INSTALLER-TRANSITION-CHAIN`); fences/epochs/wfh carried (STAGING HELD gen14/rev57 claim `b73c1c98` stale, DEV free gen96/rev375 claim `6b500f31` PASS, wfh trio owners recorded, `done`=77 eventless cleanup carded `OPS-DONE-CARDS-EVENTLESS-RECEIPTS-20261123`); actions: docs-only PR landed (audit + README/PLAN rows for the dangling classA file), TT writes owed verbatim in the audit doc (outage card `OPS-TT-BACKEND-UNREACHABLE-20261125` + receipt set at healthy contact), zero product/cluster/fence touches.
> 2026-11-25 TT-fleet inventory audit -v2 (`docs/audits/2026-11-25-tt-fleet-inventory-audit-v2.md`, agent OpenChamber-Fleet-Inventory-20261125-v2, worktree `tmp/tt-fleet-inventory-audit-20261125-v2`, tip `5f401034` = PR #1213 merge): **TT backend still UNREACHABLE** (fresh 5 attempts `connection_error`, incl. 120s timeouts — receipt payload owed verbatim lives in the 11-25 v1 doc; zero TT writes); GitHub-side fresh at the new tip: Wreck probe `0/26` (sole in-flight claim `Wreck-147-34-17`, delivery M50 proofs, via PR-body receipt), `pr_open=1` → PR #1208 (`owned-enrollment` FAILURE registered-infra class, carried, landing owner-side), `done=77`/eventless 76 (+ `OPS-CI-RULES-GATE-VERDICT-6152-20261125` now `available` from PR #1213 — honest, its evidence `docs/registry/ci/2026-11-25.md`), wfh=3 trio + fence (STAGING HELD stale `b73c1c98`, DEV free) carried; CI contract drift **CLEAN zero**: live 66 jobs / checklist 66-66 / verifier 34⊆graph, `bc9e4909..5f401034` diff = checklist rebuild only + 2 TESTING lines, tip push runs green (#6146 ci-gate, #6137 full 0 failed/27 passed, #6089 ci-pr, deploy #6064 success); GHCR digest sweep fresh: `23/24` at pin `:eb341dd8` byte-equal vs 11-22 map + **auth tag-absence drift** (`auth:eb341dd8` "manifest unknown" now, 11-22 recorded `885d1ce2…`; latest auth pin `:5f401034` = `d9d48422…`, monitor-only, product fence digest `5a12dc03…` intact), fence `:d4af0992` re-verified 7/7+renderer byte-exact vs 11-23 map, negative probes `admin@…`= `25e8bb69…`, `chat@a88a922d`=`1a28b72c…` → gate **HOLD `:d4af0992`**; actions: docs-only PR landed (this audit + README index row), zero new cards (all findings carried/covered, TT writes impossible), epochs untouched (v16 T13→`GI-267` latest, V17 unopened), zero product/cluster/fence/candidate touches.

> 2026-11-26 TT-fleet inventory audit (`docs/audits/2026-11-26-tt-fleet-inventory-audit.md`, agent OpenChamber — Fleet Inventory 20261126, worktree `tmp/tt-fleet-inventory-audit-20261126`, tip `ab309265` = PR #1217 merge): WIP plan holds — `A1` Active, GAME Parallel active, `A2` Next, `A4` Parallel active (epoch title reconciled 11-21 — no action), `A3/A5–A7` Queued. **TT backend UNREACHABLE third day** (`connection_error`/`ETCPREFILED`, `--timeout=120s` no backoff, raw LAN `http_code=000` incl. `/api/health`, no local `3099x` listener) → per-status census BLOCKED all statuses `0` successful calls; done↔deployments/pr_open↔PR/wfh/fence/epoch/journal axes BLOCKED receipts, carried state (`done=77`/eventless 76, wfh trio, STAGING HELD gen14/rev57 `b73c1c98` stale, DEV free); PR reality: sole open PR #1208 `CONFLICTING`/`DIRTY` vs `6f652d06` (registered anchor-clash, owner-side landing). CI drift NET ZERO job-name (`66↔66` tip↔`bc9e4909`, renames none, verifier blob `7ff776e6…` unchanged, `34⊆graph`); `.github/**` delta = checklist rebuild + step-level nsi additions only (nsi timeout `30→60`, legacy24/legacy25/native51-retry/catalog-call steps via !1216/!1217) — job-level contract intact; `gh pr checks 1217` nsi+`backend-auth-image` green (429 retired on green), sole FAILURE `owned-enrollment`; protection `["ci-gate"]` strict:false vs Tier-1 `4` names (report-only). GHCR digest cross-check fresh: **`:d4af0992` `24/24` byte-exact vs recorded candidate map → gate HOLD verified intact**; recent pins tag-sparse (tip `2/24` `web`+renderer, `:bfefc167` `23/24` `auth` absent, `:291b118a` `2/24`) = GHCR tag-retention class → monitor row registered `docs/todo/ci.md`, monitor-only; negative probe clean, zero GHCR mutations. Actions: docs-only PR (audit + README/PLAN rows + ci.md monitor), zero cards (outage payload verbatim, not creatable), zero code/fence/candidate touches.

> 2026-11-26 TT-fleet inventory audit v2 (`docs/audits/2026-11-26-tt-fleet-inventory-audit-v2.md`, agent OpenChamber — Fleet Inventory 20261126 v2, worktree `tmp/tt-fleet-inventory-audit-20261126-v2`, tip `7fcc22cc9` = PR #1220 merge): WIP plan holds — `A1` Active, GAME Parallel active, `A2` Next, `A4` Parallel active (epoch title reconciled 11-21 — no action), `A3/A5–A7` Queued. **TT backend UNREACHABLE fourth day** (fresh receipts: `context deadline exceeded` on `--timeout=90s task list --status available`, `connectex` ConnectEx on `projects list` — census `0` successful, all TT axes BLOCKED, zero writes, v1 outage payload stands, no duplicate card). PR reality: exactly one open PR #1208 `CONFLICTING`/`DIRTY` (`05eb6318` 11-24-audit branch vs `6f652d06`, owner-side landing, report-only); merged chain #1215–#1220. CI job-name drift v1→v2 tip **NET ZERO**: window `f5db0517..7fcc22cc9` = 3 docs files `+51/−1`, zero CI/source files, blobs byte-identical (`7ff776e6:` verifier, `3afca89e:` ci.yml, `60901ebf:` checklist, `f6e14bf5:` gate-policy), phantom registry paths still absent (F9), protection `["ci-gate"] strict:false` vs Tier-1 four (F7 unchanged). Tip master-push run #6174 (`45740179154`) COMPLETED `66 = 2/0/64` zero reds; `gh pr checks 1220` `7 SUCCESS / sole FAILURE owned-enrollment (registered, not gate-required) / 56 SKIPPED`; full-profile authority stays #6170 `133 = 118/11/4` zero new classes; no new `docs/todo/ci.md` rows (monitor rows landed via #1218/#1219). STG gate digest sweep fresh at `:d4af0992`: **`24/24` byte-equal vs the 11-22 recorded map** (live = 11-22 = 11-19 = v1 agreement; self-correction report-only: 11-23 audit table's `:d4af0992` column is an outlier `8 OK/16 DIFF`, no tag mutation); tip tag `0/24` expected-absent retention class, negative probe `manifest unknown` clean — gate HOLD `:d4af0992` stays operational (stale fence `b73c1c98` gen14/rev57 + LAN fail-closed chain), not digest mismatch. Actions: docs-only PR (audit + README row + this WIP row), zero cards, zero code/fence/GHCR/epoch touches.

Scope разделён явно:

- `alpha scope` — `A1–A4` и перечисленный в `G1` функциональный slice `A5`;
- `post-alpha committed scope` — остаток `A5`, `A6–A7`, затем отдельные milestones для mobile, verification, bots и stories по данным `G1`; эти фичи не deferred, но не получают WIP до entry review после alpha;
- `deferred` — release activation федерации вне отдельно согласованного игрового спринта; G0–G4 не включают federation deployment.

## Проектирование игровых интеграций

По запросу владельца подготовлен [proposed target игровых интеграций](features/game-integrations.md):
Game API, Unity/Unreal SDK, MMO communities, [игровые боты](features/game-bot-interactions.md)
и [game federation](architecture/game-federation.md). Документация изначально
готовилась без запуска runtime; позднее владелец отдельно запустил параллельный
спринт, не изменяя A1 и G0–G4. По уточнению владельца
план разработки — **один спринт целиком**, включая sdk-account с обеими
конвертациями, внешние API/контракты, мессенджер, ботов, MMO communities и игровую федерацию.
Assets/SDK для Unity, Unreal и других движков, developer CLI/portal и публикуемые
инструменты интеграции — отдельная задача вне этого спринта. Приёмка Voice
использует внутренние test clients и controlled game adapter, а не готовые assets.
Внутри него — подзадачи с зависимостями и общей приёмкой, а не релизные этапы.
Федерация входит в implementation scope только как часть этого owner-approved
игрового спринта; это не разрешает её release activation. `G0–G4` исключают
federation deployment, её DB/migrations, readiness и alerts. Спецификацию и
текущую реализацию следует оценивать по этой feature-ветке, а не по старому
описанию в `master`.
Все недостающие Game Integration Service, federation runtime, хранилища и
Voice Node bundle входят в работу спринта. G01–G13 и Q01–Q12 команда закрывает
конкретными техническими решениями в owning docs до зависимого кода.
Спринт имеет отдельную integration queue: владелец очереди принимает PR,
сохраняет последний принятый commit как rollback point и готовит revert в этой
feature-ветке при неудачной интеграции. Отдельно разрешённая синхронизация
remote `master` → feature-ветку уже выполнена в PR #549. 2026-10-02 владелец
отдельно разрешил merge уже проверенных implementation checkpoints в `master`
через PR с обязательным зелёным CI и merge commit. Это снимает прежний merge
hold для этих наработок, сохраняя весь незакоммиченный WIP в исходном worktree.
Staging rollout и live/real-provider tests этим решением не разрешены.
Gate `A1` acceptance со staging остаётся отдельным и открытым; этот спринт и
его PR не меняют его состояние. Live provider/device tests также не
входят в разрешённую приёмку: используются локальные/fake/sandbox checks и
controlled game adapter. Ни один из этих checks не считается подтверждением
live gate. Состояние `A1` и его собственные gates/rollout не меняются.
Подзадачи, открытые решения и acceptance —
[game-integrations-acceptance.md](testing/game-integrations-acceptance.md).
Исполняемая декомпозиция —
[game-integrations-exec-plan.md](testing/game-integrations-exec-plan.md).
Q11 owner/operator/provider and clean bootstrap contracts are frozen in those
documents; empty-DB/host and real-Google runtime evidence remains a prerequisite
before production admission. Q12 capacity/RPO/RTO figures are provisional
qualification targets only, pending T08/T93 measurement.
Статус `proposed` в FEATURES не означает обещание релиза или готовность runtime.

## PLAN и TODO

| Источник | Назначение |
|---|---|
| Этот файл | Глобальный порядок, границы этапов и release gates |
| [TODO.md](TODO.md), `docs/todo/*.md` | Инвентарь локальных дефектов, рисков и доказательств |
| [FEATURES.md](FEATURES.md) | Каталог возможностей; scope/status берётся отсюда, из этого PLAN и feature docs, а не из checkbox count |
| `docs/features/*` | Каноническое продуктовое поведение и feature DoD |
| [TESTING.md](TESTING.md) | Требуемые уровни проверки |

Датированный аудит на указанном SHA нашёл 545 открытых и 152 оставленных выполненных пункта, хотя [TODO.md](TODO.md) требует выполненное удалять. Все 100 design-пунктов относятся к Penpot/design parity, причём 88 — атомарный missing-buttons audit; также найдены stale-записи и точные дубли. Это разовое доказательство проблемы, не второй живой счётчик: актуальный инвентарь остаётся только в `docs/todo/*`.

Поэтому количество открытых пунктов не является процентом готовности. `Critical` и `High` в TODO означают локальную тяжесть дефекта или риск конкретной фичи, но не место фичи в глобальной очереди.

Правила дальнейшего ведения:

- feature-sized результат живёт здесь; RPC, миграция, кнопка, тест и отдельный дефект — в TODO;
- пункт TODO должен называть наблюдаемый gap, источник контракта и проверяемый DoD;
- дубли объединяются, stale-записи удаляются, выполненные записи удаляются после переноса нужного контекста в код или канонический документ;
- мелкая задача берётся вне активного milestone только при риске security, privacy, data loss, падении обязательной проверки или если она разблокирует вертикальный поток;
- чистка backlog выполняется небольшими пакетами параллельно и никогда не подменяет создание продукта.

## Честный статус

Статусы относятся к текущему заявленному функциональному объёму, но не включают явно future/post-v1 возможности, federation и финальную Penpot-полировку:

- `shipped` — текущий основной scope реализован и проверяется;
- `core-live` — главный пользовательский поток работает, но остаётся существенное расширение или hardening;
- `partial` — один или несколько центральных потоков ещё не замкнуты;
- `stub` — центральный путь фейковый или его нельзя безопасно показать;
- `deferred` — не входит в текущую очередь.

| Фича | Статус | Состояние и крупнейший остаток |
|---|---|---|
| [Текстовый чат](features/text-chat.md) | core-live | DM, группы, каналы, треды, folders, Quick Access, archive и pins ≤5 работают; plain-text list preview есть, но delivery/media labels, group/channel read-модель, send options и rich composer неполны. |
| [Пересылка сообщений](features/forward-messages.md) | core-live | Attribution, copy-as-new и commentary работают; нужна одинаковая проверка всех rich payload и будущих типов контента. |
| [Войс-чат](features/voice-chat.md) | partial | Join/call signaling и часть organizer UI есть; actual-media E2E, command/mute/speak enforcement, движение между комнатами, roster/lifecycle и active-session UX не замкнуты. |
| [Шара экрана](features/screen-share.md) | partial | API/signaling core есть; actual-media E2E, source picker, system audio и staging RTC acceptance не закрыты. |
| [Сторис](features/stories.md) | partial | Create/feed/LFP ядро есть; нужны audience/privacy, editor и media lifecycle. |
| [Регистрация и контакты](features/auth-and-contacts.md) | partial | Email, guest, sessions и reset UI работают; delete пока soft/deactivation path, а phone/OTP, erasure/tombstone и часть hardening не закрыты. |
| [Верификация](features/verification.md) | partial | Source-scoped Twitch+YouTube lifecycle, durable Auth→User sync/CAS и Settings выбранного профиля реализованы; production OAuth callback/resume и live-provider acceptance остаются. |
| [Профиль пользователя](features/user-profile.md) | core-live | Avatar, bio, status, switch и banner field работают; нужны animated media, premium gating и полная persistence. |
| [Множественные профили](features/multi-profile.md) | core-live | Create/switch/delete/frozen/downgrade picker работают; нужны entitlement lifecycle, primary-profile flow и regression coverage. |
| [Друзья и контакты](features/friends.md) | core-live | Requests, block, DM gate, contacts/favorites и QR paste работают; нужен phone-book hash pipeline и остаточный fail-closed hardening. |
| [Статусы присутствия](features/presence.md) | core-live | REST, WS в общем чате и auto-idle работают; нужны friend fan-out, game detection и privacy edges. |
| [Приватность](features/privacy.md) | partial | Основные visibility и block gates работают, но viewer-aware presence/last-seen fan-out и `show_read_receipts` ещё не обеспечены; discovery и inactive-profile границы тоже неполны. |
| [Спейсы](features/spaces.md) | core-live | Create/join/tree/invites, gRPC audit read, additive audit-filter/ingestion proto и durable ownership store/freeze/terminal foundation работают; target public transfer/scheduled-delete и полная Gateway/Flutter lifecycle/audit-вертикаль, catalog/templates, audit filter/writer runtime и entry requirements остаются открыты. |
| [Ролевая модель](features/roles.md) | core-live | Custom roles и основные send/join deny работают; voice organizer и остальные text/voice permissions требуют единой проверки на всех входах. |
| [Матчмейкинг](features/matchmaking.md) | partial | Solo/space queue и catalog работают; нет voice-roster party, полного post-match цикла и cleanup. |
| [Каталог игр](features/game-catalog.md) | core-live | Seed, game selection и staff moderation работают; нужна полная current-вертикаль modes/roles и recent/popular UX. |
| [Репорты / модерация](features/reports.md) | partial | Report/appeal endpoints и admin resolution core есть; sanction enforcement/delivery across transports и полный appeal loop не доказаны end-to-end. |
| [Шифрование](features/encryption.md) | core-live | Opt-in E2E DM и key backup работают; нужны понятный fingerprint/key-change и recovery UX. |
| [Онбординг](features/onboarding.md) | partial | Screens/flags и часть coach marks есть; async progression может зависнуть, а обязательный переход к Space discovery не замкнут до готовности каталога. |
| [Accessibility](features/accessibility.md) | core-live | Semantics, keyboard/focus и text-scale baseline есть; нужны physical screen-reader acceptance и high-contrast polish. |
| [Локализация](features/i18n.md) | partial | EN/RU UI baseline есть; locale persistence/profile sync и локализация backend messages неполны. |
| [Боты](features/bots.md) | partial | Registry/install, slash, webhook/polling и runtime core есть; нужны chat-type/scopes/lifecycle hardening и полный live bot flow. |
| [Навигация](features/navigation.md) | core-live | Folders, Quick Access, archive, requests и desktop shell работают; нужны mobile chrome и остаточные row/state gaps. |
| [Поиск](features/search.md) | partial | Global/in-chat query, ACL и исключение encrypted payload работают; exact message jump, historical backfill, свежие projections, rich-content mapping и admin gate неполны. |
| [Deep links / Sharing](features/deep-links.md) | partial | Invite parsing/compose работают; остальные Space/chat/message/profile targets, prod well-known/signing и physical mobile acceptance не закрыты end-to-end. |
| [Платформы](features/platforms.md) | partial | Web и Windows проверяются в CI; Windows daily-driver и mobile release path ещё не завершены. |
| [Обновления клиентов](features/updates.md) | partial | Version/force-update paths есть частично; desktop delivery и реализация заданного mobile OTA path открыты. |
| [Уведомления](features/notifications.md) | partial | In-app, quiet hours и message-request core есть; sanction delivery пока push-oriented, а contracts/presentation, реальные push, типы и grouping неполны. |
| [Подписка](features/subscription.md) | stub | Внутренние entitlement/grace/events частично есть, но реальный checkout и provider lifecycle нельзя считать готовыми. |
| [Хранение файлов](features/file-storage.md) | partial | Backend upload, R2, retention и SHA verification работают; основной non-E2E download/expired URL UX, dedup, async processing, transcode и previews неполны. |
| [Наблюдаемость](features/observability.md) | partial | Код и provisioning baseline есть; live staging, полная telemetry chain, P1 routing и restore evidence не приняты. |
| [Продуктовая аналитика](features/analytics.md) | partial | ClickHouse ingest имеет restart-durable source delivery и dedup в official reads; direct gRPC ingest остаётся memory-only, а consumer health, dashboard/query semantics и coverage неполны, но analytics не gate бесплатного alpha. |
| [Федерация](features/federation.md) | deferred | Runtime implementation разрешена только внутри отдельного Game Integrations sprint; release activation и deployment остаются deferred, `G0–G4` федерацию не включают. |

## Технический дизайн сейчас

Отсутствующий, устаревший или не утверждённый Penpot frame не блокирует функциональную реализацию. До отдельного visual-approval этапа агент строит технический UI по следующему порядку источников:

1. `docs/features/*` и сервисные контракты;
2. [screen-controls.md](design/screen-controls.md);
3. [brand.md](design/brand.md), tokens, a11y и i18n;
4. существующие `VoiceTheme`, `src/frontend/lib/ui/core/*` и уже shipped UI;
5. утверждённый Penpot — как дополнительная ссылка, если он не конфликтует с каноном выше.

Технический дизайн — не заглушка. Он обязан закрывать рабочий путь, loading/empty/error/offline/permission/entitlement states, responsive layout, keyboard/focus, semantics, i18n и тесты. Нельзя оставлять fake action, demo data, raw colors, обход backend-контракта или кнопку без честного состояния.

Агент сам выбирает обратимые локальные детали: token spacing, существующий component/icon, dialog против sheet по текущим паттернам, нейтральный текст. Документированные data/API, permissions, privacy/security и billing contracts агент реализует автономно. Остановка и решение владельца нужны только для неописанной или противоречивой продуктовой семантики, а также для breaking/необратимого решения о data model, destructive recovery или крупной IA.

Финальные Penpot registry/GAP/export/pixel-parity и общий визуальный pass находятся в `H5`; до него Flutter может считаться функционально готовым, но не visual-approved.

## H — задачи владельца

Эта ветка короткая и не является очередью для агентов. Владелец получает только минимальный пакет: рекомендуемый default, альтернативы, точный вопрос или нужное значение и уже подготовленную проверку.

Каждый decision handoff содержит один ID, один точный вопрос, рекомендуемый default, не более двух альтернатив, затронутый slice/gate и цену задержки. Владелец не разбирает raw TODO и не получает пакет из несвязанных решений.

### H1 — Продуктовые решения без канона

**Состояние:** по требованию; не блокирует документированные части `A`.

**Действие:** выбрать вариант только для реально конфликтующего или отсутствующего поведения. Текущие alpha-семантики email activation, composable Space entry policy и удаления аккаунта/Space зафиксированы. Следующие owner decisions появляются только при входе соответствующего scope: phone/SMS provider и anti-abuse flow, а также крупная mobile IA.

**Разблокирует:** конкретную узкую ветку реализации; остальные части milestone продолжаются.

**Готово / доказательство:** решение записано в соответствующем `docs/features/*` или ADR, после чего у агента есть проверяемый DoD; [product-roadmap.md](todo/product-roadmap.md) может только ссылаться на этот канон.

### H2 — Внешние аккаунты, legal и credentials

**Состояние:** не нужны для local/compose и fake-provider DoD; нужны только перед соответствующим release gate.

**Действие:** создать или подтвердить merchant/provider accounts и передать через штатный secret store значения для billing, Resend, Cloudflare R2, encrypted off-cluster backup destination, optional GIF-search provider, Firebase/FCM, APNs, OAuth providers и реального Alertmanager destination. Staging и production получают разные account/key/bucket/endpoint sets; внутри каждого environment отдельно задаются User/File buckets так, как требует deploy manifest. Для R2 и backup target утвердить lifecycle/retention/versioning/immutability policy из подготовленного checklist. Для Windows предоставить code-signing identity/certificate; для mobile — Apple/Google developer distribution и signing assets. Для платежей также подтвердить legal/tax/offer setup, product/price IDs и отдельные sandbox/live webhook secrets и URLs.

**Разблокирует:** production attachments, подписанный Windows build, live email, push, verification, alerts, mobile distribution и paid beta.

**Готово / доказательство:** secret names и IDs из подготовленного agent checklist существуют в каждом целевом environment; validator отклоняет shared/mismatched staging/production resources и перепутанные User/File buckets, значения не попали в Git или лог.

### H3 — Домены, доступы и wiring внешнего окружения

**Состояние:** отложено до готового release candidate.

**Действие:** предоставить доступ к staging и `production/voice-prod`, утвердить production DNS/TLS/firewall, persistent storage class/PVC policy для stateful workloads и независимый off-cluster backup target. По [DEPLOYMENT.md](DEPLOYMENT.md) staging используется только для integration/regression/demo: сначала на нём собирается evidence, затем внешний invite alpha выпускается в production. Отдельное долговечное alpha-окружение допустимо только после описания его durability/privacy/backup/lifecycle policy в DEPLOYMENT. Агент заранее готовит manifests, validators, dry-run и точные команды.

**Разблокирует:** внешний Web/Windows alpha, prod deep links и staging evidence.

**Готово / доказательство:** evidence разделено по gate. Для `G1` проверены production DNS и end-to-end TLS для API/Web/LiveKit/JWKS: Cloudflare Full (strict) либо явно утверждённый эквивалент, origin certificate/HTTPS и origin allowlist; stateful workloads не используют ephemeral storage, off-cluster encrypted backup достижим по минимальным credentials; staging и production используют раздельные проверенные R2/backup credentials и endpoints; также проверены immutable Windows artifact/update feed и минимальные доступы. AASA/assetlinks, mobile domains и signing относятся только к `G3`.

### H4 — Физическая приёмка и go/no-go

**Состояние:** только на release candidate.

**Действие:** агент или QA выполняет все доступные автоматические и физические проверки и приносит готовый evidence report. Владелец предоставляет только недоступное агенту устройство/account при необходимости, назначает on-call/rollback owner и принимает go/no-go. Проверяются только включаемые платформы и возможности.

| Gate | Минимальная versioned physical matrix |
|---|---|
| `G1` | non-empty supported Windows OS/arch и Web browser/version matrix; clean-machine trusted Windows install и old→new update из signed production feed с fallback; cold start; two-client real mic+speaker call; network loss/reconnect; PTT/tray/hotkeys; принятие cohort cap и предложенных alpha RPO/RTO |
| `G2` | live purchase/cancel scheduling и entitlement visibility; expiry/grace/downgrade остаются sandbox/test-clock проверкой |
| `G3` | TalkBack/VoiceOver, background voice, App/Universal Links и real push на целевых устройствах |

**Разблокирует:** соответствующий внешний gate; mobile и paid не блокируют бесплатный Web/Windows alpha.

**Готово / доказательство:** checklist хранит SHA, OS/device, network profile и pass/fail каждого пункта; владелец получил готовый отчёт и записал решение о выпуске или конкретном дефекте.

### H5 — Финальный визуальный дизайн в Penpot

**Состояние:** сознательно отложено; не блокирует `A1–A7`.

**Действие:** когда продуктовые потоки устраивают владельца, одним пакетом утвердить visual direction, tokens и reference screens для auth/onboarding, chat, Space/voice, matchmaking и settings/profile. Не более двух review rounds на пакет; детальная registry/GAP/export/parity-проверка остаётся агентской.

**Разблокирует:** статус `visual-approved` и финальную brand polish, но не functional alpha.

**Готово / доказательство:** владелец утвердил visual direction, tokens и ключевые Penpot screens/states; список утверждённых snapshot зафиксирован для последующего agent parity pass.

## A — автономная работа агентов

Milestone — законченный пользовательский результат. Его можно дробить на небольшие PR и параллельные сервисные треки, но нельзя объявлять готовым по числу закрытых TODO. `Dependencies` ниже никогда не требуют незавершённого `H`: live activation вынесена в release gates.

### A1 — Пользователь может ежедневно переписываться

**Результат:** два новых пользователя без ручной настройки проходят onboarding, находят друг друга и стабильно общаются в Web.

**Объём:** email/guest/session/reset, friends/request/block, DM/group/channel/thread, realtime reconnect, global inbox catch-up, history catch-up per selected chat, unread/read state, archive/folders/Quick Access, минимальный account soft-delete и базовые attachments. Для A1 soft-delete означает немедленный отзыв sessions, запрет новых DM в обе стороны, скрытие удалённого peer из новых snapshots и один локальный неперсистентный terminal marker «Пользователь удалён» в уже загруженной истории. Полная 30-дневная erasure/pseudonymization, durable cross-service tombstones и restore UX остаются будущим scope `A4`. Auth перестаёт ходить напрямую в `user_db` для profile/verification/phone flows и использует User contract. Phone-book discovery — post-alpha/G3 mobile scope: A1/G1 Web/Windows не запрашивают контакты и не отправляют phone hashes. Исправляются видимые profile/chat switching regressions.

**Dependencies:** существующие Auth, Chat, Messaging, Realtime, Social, User и File contracts; внешние providers не нужны.

**DoD:** новый пользователь проходит путь без CLI и hidden token; Auth не требует credentials к `user_db`, межсервисные profile operations идут через contract и fail closed; после reconnect клиент завершает global paginated inbox snapshot и не стирает cache при неуспешной странице, DM/group/channel сохраняют per-member unread→read; attachment переживает service restart и скачивается с тем же hash; полная история приходит через cursor только per selected `chat_id`; block/privacy deny не fail-open; после account soft-delete старые sessions недействительны, send в обе стороны запрещён, fresh snapshot скрывает DM, а локально загруженная история показывает один local non-persisted terminal marker «Пользователь удалён»; empty/error/offline состояния не маскируют потерю данных.

**Verification:** unit/integration по затронутым сервисам, Flutter widget tests и live multi-account compose E2E для login → contact/request → DM/group/channel → per-member unread/read → attachment upload → restart/reconnect → global inbox snapshot (`main` / `requests` / `archive`) → history for selected chat → download/hash → block/delete.

### A2 — Invite-only Space с рабочим голосом

**Результат:** группа создаёт закрытый Space, настраивает дерево и роли, входит в голосовые комнаты и безопасно управляет жизненным циклом Space.

**Объём:** Gateway/Flutter vertical slice для join/leave/transfer, text/voice tree, invites, role checks на всех входах, roster/session events, движение между комнатами, join/speak/mute permissions и понятные RTC states. Audit log получает доступный owner/admin path. Delete flow следует зафиксированному контракту: owner confirmation, 7 дней hidden/frozen recovery, затем идемпотентный cross-service purge и attachment GC; это узкий последующий slice, не blocker остального `A2`.

**Dependencies:** `A1` identity/chat foundation; local LiveKit и test media достаточно.

**DoD:** owner не заперт в Space; invite/member/role transitions согласованы; запрещённое действие не проходит через альтернативный transport; два test-клиента реально передают и получают media track, а не только signaling; пользователь видит connecting/reconnecting/permission/device errors; session cleanup детерминирован. Audit read path сохраняет lifecycle entries и доступен только owner/admin по каноническому ACL.

**Verification:** Space/Role/Voice integration tests, Flutter widget tests и compose E2E create → invite → join → text → test-media exchange → move → role deny → audit read/persistence → transfer/owner leave.

### A3 — Полный цикл поиска команды

**Результат:** Voice демонстрирует своё отличие: solo или текущая voice-party находит команду, общается во временном text+voice контексте и завершает матч с историей.

**Объём:** snapshot party из voice roster, queue reset при изменении состава, search/match events, accept/decline/timeouts, temporary squad chat/voice, cleanup, post-match rating/history и `mm_ban`. Catalog остаётся источником games/modes/roles.

**Dependencies:** `A2` roster/session events; текущий game catalog.

**DoD:** состав party нельзя подменить client payload; одновременные join/leave/cancel идемпотентны; участники найденной команды обмениваются сообщением и test-media во временном text+voice контексте; отказ/timeout/завершение освобождают ресурсы, а после cleanup старый контекст отклоняет доступ; история и санкции видны в следующем поиске.

**Verification:** deterministic service tests, race/idempotency cases и multi-client compose E2E solo + party → match → temporary chat send/receive + test-media exchange → result → history → cleanup → authorization deny/ban.

### A4 — Репозиторий готов к безопасному внешнему alpha

**Результат:** репозиторий содержит staging-compatible harness, с которым оператор после выдачи доступов может развернуть exact SHA, доказать безопасность базовых потоков, восстановить данные и получить сигнал о P1.

**Объём:** fail-closed privacy/S2S, report → resolve/dismiss → sanction → appeal, полный account deletion lifecycle поверх A1 soft-delete (30-дневное restore window, затем необратимая PII/credential/profile-media erasure или pseudonymization и cross-service tombstones), alpha admission guard, versioned release-profile/capability/client-compatibility, operational-tier и durable-store manifests, secret/config validation, repeatable migrations, backup/restore automation, Loki/request-id chain, Prometheus/Grafana, actionable alerts, degraded-mode checks и минимальные runbooks для deploy/canary/rollback, restore, Redis/NATS triage и P1 ack/escalation. Full per-service runbooks и chaos program остаются вне alpha gate. Product Analytics не входит в launch observability gate.

**Dependencies:** `A1–A3` для полного product smoke; инфраструктурный код проверяется раньше на disposable/local environment.

**DoD:** автономный harness закрывает пять доказуемых блоков:

- Capability/admission: exact-SHA manifest и config validator запрещают dev defaults/test checkout; operator/admin mint path аудируется; email и guest проходят invite mint → atomic single-use redeem с expiry/revoke; replay, concurrent redeem и cap race отклоняются. Из каждой disabled-записи генерируется negative UI/REST/WS suite. Federation отсутствует в release images/rollout/readiness.
- Client compatibility: каждая заявленная OS/arch/browser tuple получает smoke; risk-based sampling применяется только к расширенному regression и записан в matrix. Previous/current Web+Windows проверяются с previous/current backend, version/forced-update/update-URL flow; backend rollback совместим с уже обновлённым клиентом.
- Deploy/recovery: на disposable environment повторяются deploy/migrate, expand-contract, canary с ненулевым synthetic sample и измеримый rollback trigger. Backup из source восстанавливается только в isolated validation namespace/cluster с synthetic tenant/object и никогда не пишет обратно в source. Manifest покрывает authoritative DB/object/event stores из [DATA_STORES.md](DATA_STORES.md), проверяет cross-domain invariants и rebuild исключённых cache/projections. Drill измеряет RPO/RTO относительно agent-recommended provisional thresholds; владелец принимает их только в `G1/H4`.
- Operations: Role и Matchmaking имеют Tier 1 SLO/dashboard/alerts/degraded mode; все Tier 0/1 scrape targets и datasources healthy; login/DM даёт request-id chain Gateway → Messaging → NATS → Realtime; P1 проверяется через alert sink.
- Product smoke: mail sink/fake provider доказывает register/verification/reset и expired/bounce; затем идут A1 DM/group/channel unread/read + attachment restart/hash/delete/restore, report → resolve/dismiss → sanction enforcement + WS/UI delivery → appeal без ручного `sanction_id`, account delete → immediate deny/hide → restore и test-clock expiry → irreversible erasure/tombstone, A2 Space/voice и A3 temporary chat/test-media/history/cleanup/authorization deny.

**Verification:** CI/full на exact SHA, config/render tests, migration rerun, disposable backup→restore, canary observation/rollback-trigger drill, compose/staging-compatible smoke и review [OPERATIONS.md](OPERATIONS.md) / [DEPLOYMENT.md](DEPLOYMENT.md).

### A5 — Web/Windows становится daily driver

**Результат:** в test-signed alpha-like Web/Windows окружении пользователь устанавливает или открывает Voice, восстанавливает session и выполняет ежедневные chat/Space/profile flows; публично доверенный zero-warning путь завершается только в `G1`.

**Объём:** versioned supported Windows OS/arch и Web browser/version matrix; Windows packaging/startup/update и native Realtime path с agent-generated test signature, immutable production-compatible artifact manifest, sandbox update feed и disposable trust setup; system tray, voice while hidden и global PTT/hotkeys; responsive technical shell; folders/Quick Access/archive/requests/search; profile switch/manage; presence/privacy; notification center и read state; безопасный Space catalog/templates. Включается только то, что имеет честные loading/error/permission states. Субъективная visual polish относится к `H5`.

**Dependencies:** `A1–A4`; live push не требуется для Web/Windows free alpha.

**DoD:** test-signed install/update/cold start/reconnect сохраняют session в disposable trust setup; public trusted signature и clean-machine zero-warning install относятся к `G1`. Closing window to tray не обрывает voice; global PTT/hotkeys работают и не срабатывают в запрещённом состоянии. Keyboard позволяет открыть chat, folder/archive, search, profile и Space; notification center и read state согласованы; invisible/privacy gates не обходятся; catalog не раскрывает private Space/profile; unsupported mobile-only или premium действия скрыты либо объяснены.

**Verification:** Flutter CI, widget tests стабильных technical contracts, Windows install/update/start → tray/background voice → global PTT/hotkey smoke и Web/Windows E2E session restore → navigation/search → notification/read → presence/privacy → profile switch → Space catalog/join.

### A6 — Rich communication и media pipeline

**Результат:** общение поддерживает ожидаемые современные форматы без ручной обработки и не создаёт неконтролируемое хранение.

**Объём:** silent/scheduled/when-online delivery, stickers/GIF, voice notes, rich forward validation, file dedup/references, async processing, safe previews, image/video/GIF/PDF transformations, retention/quota и screen-share source picker/system audio там, где платформа позволяет. Third-party GIF search использует provider-neutral adapter и deterministic local fixture catalog; system/user-upload GIF не зависит от live provider.

**Dependencies:** `A1` messaging/file foundation и `A2` media session foundation; premium quota тестируется seeded entitlement.

**DoD:** каждый тип контента имеет единый validation/ACL path; retry идемпотентен; processing status виден клиенту; orphan cleanup и retention доказаны; forward не обходит attachment rules; unsupported capture явно недоступен. Без выбранного provider/key live GIF search выключен через capability manifest, а adapter полностью проходит fake contract suite.

**Verification:** contract/property/integration tests, deterministic GIF-provider fake, processor failure/retry cases, storage lifecycle tests и Flutter E2E composer → send/forward → preview/download. Live provider activation — отдельный release-capability check после точечного `H1` выбора и `H2` key.

### A7 — Provider-ready monetization

**Результат:** бесплатный продукт не зависит от платежей, а engineering для paid beta полностью готов на fake/sandbox; live-включение отдельно требует `G1`, merchant/legal setup из `H2` и acceptance/go-no-go из `H4`.

**Объём:** provider-independent subscription state machine, versioned paid-offer/capability manifest, checkout adapters, webhook signature/idempotency, cancel/resume/renew/failure, grace/downgrade/freeze/unfreeze, `subscription.events` и Auth JWT tier. При входе milestone в WIP агент сам материализует и version-freeze полную canonical benefit matrix из [subscription.md](features/subscription.md), по умолчанию включая все записанные benefits: User/Flutter cosmetics, banner/status/username/themes/icons; Messaging — до трёх реакций; Chat/Space — лимиты; File — размер/priority; Voice — stream quality и room limit; multi-profile freeze/unfreeze; Space Pro member/channel limits, banner и custom emoji; Notification — grace reminders. Exclusive stickers/emoji, animated media и custom emoji опираются на `A6`. `H1` нужен только при реальном конфликте канона или предложении исключить benefit; такой вопрос не останавливает остальные slices. Исключённый benefit сначала получает явное scope-решение и обновление канона, а не просто исчезает из UI. UI использует технический дизайн и не показывает test URL как продукт.

**Dependencies:** `A1–A6` и стабильные limits/contracts из активных features; fake provider и seeded entitlements достаточны для автономного milestone.

**DoD:** entitlement меняется только из доверенного lifecycle; replay/out-of-order webhook безопасен; downgrade не теряет данные и предлагает выбор; все consumers сходятся после redelivery; каждый benefit в paid-offer manifest имеет server/client E2E и совпадает с checkout/plan management; disabled billing оставляет полностью рабочий free product.

**Verification:** provider contract suite с fake/sandbox, webhook replay/order tests, NATS redelivery E2E, benefit-matrix coverage, grace/downgrade scenarios и Flutter checkout/management tests без реальной карты.

После `A7` новый цикл планирования выбирается по данным alpha: mobile release, расширенная verification, bots ecosystem, полный stories editor и прочие current-scope расширения. Они остаются в feature docs/TODO, но не конкурируют с активными milestone по числу checkbox.

## Release gates

| Gate | Назначение | Требования |
|---|---|---|
| `G0 — product proof` | Локальная демонстрация ценности без внешних аккаунтов | `A1–A3`; зелёный multi-client compose E2E |
| `G1 — free invite alpha` | Ограниченный Web/Windows запуск без billing и mobile push | `A1–A4`; из `A5` — production signed artifact/feed, supported client matrix, old→new update/fallback, tray/background voice/PTT, stable session/reconnect, responsive shell, keyboard critical path и notification/read consistency; из `H2` — Resend, R2 и encrypted off-cluster backup credentials/policies, Windows signing и реальный alert destination; `H3` и соответствующая часть `H4`; staging qualification пройден, затем на `production/voice-prod` для того же exact SHA доказаны persistent storage, invite admission/cap и negative signup, capability/ops/client-compatibility manifests с Role+Matchmaking Tier 1, negative disabled-path и degraded-mode smoke, email register/verification/reset delivery, attachment upload→restart→download/hash→delete/restore, expand-contract deploy/canary с ненулевым synthetic sample и SLO observation window, server rollback after client update, target-generated full-profile backup восстановлен в isolated validation target без записи в production с принятыми alpha RPO/RTO, healthy Tier 0/1 scrape+dashboards, request-id chain, P1 receipt и multi-client smoke A1–A3 из `A4` |
| `G2 — paid beta` | Включение Premium/Space Pro | `A7`; на том же exact G2 RC SHA повторены G1 regressions/free path с billing on и off, target canary/rollback; ops-manifest повышает Subscription и критические entitlement consumers до Tier 1 и доказывает dashboards/alerts/P1-P2 receipt для provider/signature/payment errors, webhook lag и NATS redelivery/convergence; durable-store manifest/restore повторён с `subscription_db` и provider reconciliation source, сохраняя active/cancel-scheduled/grace entitlements и идемпотентный webhook replay без двойного доступа/списания; offer manifest согласован с [subscription.md](features/subscription.md), каждый продаваемый benefit имеет E2E; provider/market matrix разрешает checkout только для отдельно включённых провайдеров, и для каждого live path доказан `purchase → signed webhook → entitlement → cancel scheduled` с entitlement до period end; sandbox/test clock доказывает period-end и failed-payment grace → downgrade/freeze; sandbox/live используют разные webhook secrets/URLs, а negative cross-environment event test доказывает отказ; merchant/legal часть `H2` и go/no-go из `H4` |
| `G3 — mobile beta` | Android/iOS distribution, push и universal links | gate не активен до следующего planning cycle: после `G1` сначала создаётся отдельный autonomous mobile milestone с DoD; затем нужны signing/push из `H2`, prod links из `H3` и device acceptance из `H4` |
| `G4 — visual-approved` | Финальный единый визуальный уровень | `H5`; versioned matrix активных platforms/screens и loading/empty/error/permission states; agent parity pass; exact-SHA visual report, где каждый diff исправлен или имеет явный owner waiver; cross-platform QA |

`G1` не ждёт `G2–G4`. Phone auth, paid tier, mobile push, public discovery и незавершённые rich types должны быть выключены или честно ограничены, а не изображать готовность.

## Как агенты исполняют план

- В последовательности `A` одновременно активен ровно один milestone. Отдельно согласованный игровой спринт идёт параллельно `A1` в собственной integration queue, без merge в `master` и staging deployment до приёмки `A1`. Каждый outcome дробится на независимые `T-*` задачи по контрактам, сервисам, Flutter и verification.
- Все готовые независимые задачи активного milestone запускаются максимально параллельно в изолированных treehouse worktree: один crew — один worktree и непересекающиеся write scopes. Профили/контракты, backend-сервисы, Flutter и verify идут параллельно, когда их входы уже определены; зависимые slices ждут contract seam, а не создают второй milestone.
- Fleet state ведётся локально в `tmp/fleet/`: у каждой `T-*` есть outcome, scope, канон, worktree, owner profile, verification и integration status. Результаты интегрируются через git/PR, worktree возвращается после merge или отмены.
- Первый приоритет — закрыть пользовательский вертикальный путь и его failure states. Массовая чистка Low/Common не получает fleet раньше milestone DoD.
- Один PR содержит одну проверяемую смысловую задачу. Большой milestone закрывается серией небольших PR, каждый с тестами своего слоя; следующий `A` milestone не входит в code WIP до интеграции всей серии и зелёного vertical DoD текущего.
- Новый Critical/High vertical behavior получает feature/compose E2E в том же PR. Для неизбежной staged contract chain каждый промежуточный PR несёт contract test и выключенную capability; финальный enabling PR добавляет live E2E до включения флага.
- Смешанный blocker оформляется как handoff package: что уже готово, рекомендуемый default, точное действие владельца, безопасная проверка и какие независимые треки продолжаются.
- Если спецификация достаточна, агент не ждёт Penpot или дополнительного подтверждения. Если поведения нет в каноне, агент не изобретает его и записывает узкий вопрос в `H1`.
- Статус строки в матрице обновляется в том же PR, где существенно изменился пользовательский поток. TODO чистится после merge, а не хранит журнал закрытых batch/PR.

## Неподвижные архитектурные границы

- Auth остаётся Java/Spring в `src/backend/auth/`; перенос в Go требует отдельного решения.
- Realtime в `src/backend/realtime/` владеет WebSocket event flow и протоколом `s` / `resume`.
- После reconnect global inbox state сверяется через Chat REST `ListChats` и durable Messaging metadata; история сообщений догружается через Messaging REST/API с cursor отдельно для выбранного `chat_id`. Глобального WS replay/event-log catch-up нет.
- Сервис владеет своей БД; межсервисные записи идут через контракт или событие, а не прямой JDBC/SQL в чужую схему. Auth обращается к User-owned profile данным только через User gRPC и не получает credentials к `user_db`.
- Node.js для frontend/CI — 24.
- Федерация и её производные реализуются только в owner-approved Game Integrations sprint; live release activation остаётся deferred. `G0–G4` profiles исключают Federation deployment, DB/migrations, readiness и alerts.

## Верификация

Минимальная проверка определяется затронутым слоем и [TESTING.md](TESTING.md). Для milestone используются:

| Слой | Базовая проверка |
|---|---|
| Proto/contracts | `buf lint`, `buf format -d --exit-code` и breaking check по правилам CI |
| Go/backend | tests изменённых модулей; перед release candidate — `make build-all` и полный service matrix |
| Auth Java | Maven tests и Flyway validation в `src/backend/auth/`; после `mvn -B test` CI проверяет Surefire reports: каждый Auth suite с `@Testcontainers(disabledWithoutDocker = true)` существует, выполнил `tests > 0`, не был skipped и не содержит failures/errors |
| Flutter | `make flutter-ci`; Web/Windows и нужные widget/integration paths |
| Интеграция | feature E2E из [`.github/ci/e2e-features.yml`](../.github/ci/e2e-features.yml) |
| Compose smoke | `make compose-e2e-smoke` |
| Полный compose | `make compose-e2e-live` |
| Release | exact-SHA deploy/render, migrations, canary observation, authenticated smoke, backup→restore, observability и rollback-trigger evidence |

Live provider/device проверки добавляются только в соответствующий gate. Их отсутствие не превращает успешно проверенный fake/sandbox milestone в «заблокированную работу», но запрещает объявлять live gate пройденным.

## Accepted R23 deletion/retirement contract

The canonical docs decision for R23 is accepted: U1-A, U2-A, U3-B, U4-B and
U5-A, with HMAC rotation `K=P90D` and maximum restorable-backup age `B=P30D`.
It fixes the convergent Space lifecycle, permanent Role retirement, File
reference authority, service-owned cleanup receipts and explicit post-purge
data classifications. This closes the owner/docs-decision gate only. R23
implementation, migrations, protos, RED tests, activation evidence and source
review remain open, so no feature status in this plan changes.

## Accepted A7 subscription lifecycle contract

The provider-independent Premium/Space Pro lifecycle decision is accepted:
complete revisioned `subscription.entitlement_changed` snapshots, stable
transactional outbox IDs/bytes, leased PubAck delivery, service-owned
inbox/projection and snapshot reconciliation, D1/D3/D7 scheduled reminder
dedupe, per-reference File retention-account recalculation, User eligible-pair
downgrade cycles with subscription-only unfreeze, and equivalent Space Pro
payment-failure grace. Account delete/restore/purge is part of the closed state
machine, including provider-cancel outbox, deadline-independent participant
purges, asynchronous receipts and HMAC tombstone.
Push + in-app is canonical for grace reminders; email remains auth-only.

This closes only the docs-decision seam for A7 and permits independent RED/proto,
Subscription, Auth/User, File, Space/Voice, Notification/Analytics and Flutter
slices. Voice owns a revisioned Space Pro admission projection and accepts
personal quality only from Auth's short-lived trusted entitlement claim.
Runtime, migrations, generated contracts, fake-provider tests, consumer
activation and paid-beta evidence remain open, so Subscription stays `stub` and
A7 does not become an active product milestone. The executable acceptance matrix
is
[subscription-lifecycle-convergence-exec-plan.md](testing/subscription-lifecycle-convergence-exec-plan.md).
