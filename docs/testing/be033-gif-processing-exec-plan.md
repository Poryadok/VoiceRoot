# ExecPlan: BE-033 API GIF processing

## Purpose

An authenticated Gateway/File caller can upload a plaintext GIF with persisted
`UPLOAD_INTENT_GIF=2`, confirm it repeatedly, observe processing, and download a
silent MP4 plus thumbnail. This is API-only processing, not GIF messaging,
provider import, the emoji picker, or completion of milestone A6.

## Context

Sources: `docs/PLAN.md` A6, `docs/features/file-storage.md`,
`docs/microservices/file-service.md`, `docs/DATA_MODEL.md`,
`docs/DATA_STORES.md`, `docs/ARCHITECTURE_REQUIREMENTS.md`, `docs/TESTING.md`,
and `docs/CONTRIBUTING.md`. Architecture decision: Anton-57, accepted by the
dispatcher after concrete source review. Existing File image contains ffmpeg.
Current image processing decodes one frame and has no durable GIF job.

## Scope

In: API GIF intent; validated GIF bytes; bounded real codec; source identity
preservation; explicit delivered full/thumbnail representations; fenced durable
processing/recovery; atomic ready/preview commit; durable cleanup; serving and
versioned preview consumers. Out: provider fetch/search, GIF sends/UI, E2E GIF
processing, changes to source-based quota or binary dedup activation.

## Milestones

- [x] Freeze API/storage/preview contract and test-first compatibility cases.
- [x] Persist one recoverable GIF job with idempotent confirmation and fencing.
- [x] Process real GIF frames into MP4/thumbnail with bounded resources.
- [x] Serve selected representation with matching MIME/name/size/hash.
- [ ] Prove restart/concurrency/cleanup/ACL and runtime codec behavior.
- [ ] Independent review, exact-head CI, normal push/PR delivery.

## Detailed Steps

1. Add GIF-capable preview protocol v2; retain v1 acceptance unchanged and update
   File/Messaging shared validator and consumer tests before producer activation.
2. Extend File protobuf additively with GIF intent and full/thumb descriptors.
   Source MIME/size/hash/name and object revision retain their existing meaning.
3. Add File migration for delivered descriptors, durable jobs/attempt fencing,
   cleanup ledger and processor version. Confirm stores source hash and one job
   atomically; retries with the same hash return current metadata, another hash
   rejects. The worker owns attempt-specific object keys, and only a live fenced
   attempt can publish ready metadata plus preview in one transaction.
4. Add bounded codec with real ffmpeg; verify GIF framing, pixel/frame budgets,
   duration, cancellation, silent H.264 MP4 <=5 MiB and WebP thumbnail. Reclaim
   incomplete jobs and reconcile losing/stale attempts through durable cleanup.
5. Resolve storage key, MIME, filename, byte limit and hash together for full and
   thumbnail, both presigned and revocable paths. Missing GIF output fails closed.
6. Add regression/integration tests; run affected File/pkg/Messaging/Gateway
   checks, protobuf lint/generation/breaking, race/static checks and runtime codec
   fixtures. Review with source SHA, migration and codec image identity.

## Validation

- Focused preview tests prove v1 unchanged and v2 GIF support.
- File unit/integration tests prove admission, same-hash concurrency/replay,
  different-hash denial, fenced recovery, atomic snapshots, selected output
  larger than source, cleanup and deletion races.
- Runtime ffmpeg/ffprobe fixtures prove distinct frames, variable frame delays,
  transparency/disposal, odd dimensions, duration, silent video, size cap,
  malformed input and cancellation. Fake subprocess tests alone do not qualify.
- Gateway/File API integration proves upload -> processing -> status -> selected
  full/thumbnail bytes and headers under existing authorization.
- Required CI must pass for the exact published head before integration.

## Progress

- [x] Read canonical task/docs; confirm concrete source hazards with CodeGraph.
- [x] Obtain architecture ruling and nonoverlapping shared preview ownership.
- [x] Acquire clean WT68, lease `3e7823e1cd1123c60f2d903940e1ad75`, branch
  `codex/anton54-gif-processing`, base `3955bb90d170072b566c922fef92c80dcaccc206`.
- [x] Meaningful RED -> GREEN: v2 GIF preview, explicit admission, durable confirm
  replay, missing-output fallback denial and generic Messaging relabel denial.
- [x] PostgreSQL evidence: stale attempt fencing, takeover, source identity,
  deleted-row non-resurrection, partial R2 write recovery, durable cleanup retry,
  atomic ready/descriptor/outbox rollback, original PUT replay reconciliation,
  selected full/thumb HTTP bytes/headers/read bounds and access revocation.
- [x] Gateway authenticated GIF intent and additive descriptor contract tests.
- [x] File short suite, File vet, shared preview tests, Messaging scoped short
  suites, protobuf lint/format/breaking and Go/Dart regeneration pass.
- [x] File runtime image from `2c671887cbc46a93f1310a29b61ef439c0643897`,
  `sha256:94be374a8515f644bf4b5341f32b76ed9bdb732cdf934e210730ceae06a215e3`,
  runs all 8 codec tests with zero skips using ffmpeg 6.1.1, nonroot UID 65532,
  network disabled, read-only root and 384 MiB noexec tmpfs. Exact codec test
  binary SHA-256: `b8a91b57cf383039d5fb36776ee64775b9a833a5a55c429edb9b0dff39a09ecc`.
- [x] Race-enabled GIF integration suite and concurrent same-hash confirmation.
- [ ] Independent review and published exact-head CI.

## Decisions

- API upload/confirm/download is the named activation consumer. Preserve A1
  octet-stream attachment fallback and do not advertise GIF-send capability.
- Preserve source identity/quota semantics. Delivered metadata is additive.
- GIF snapshots use v2, with accepting consumers deployed before GIF producer.
- Completion and cleanup are durable/fenced; no ready row before output metadata
  and preview publication evidence commit together.
- The codec author is the independent package writer (inherited Sol6.1 medium),
  WT69 lease `d7eb6193d9f084a43ecf14bcee613188`; one writer owns `gifproc`.
  Its actual consumer is the ordinary File job worker in this deliverable.
- Reviewer found ffmpeg `-fs` may truncate below the cap and cause indefinite
  retries. Real encoding regression drove an explicit size watcher, cancellation
  cause and final size check; caller cancellation remains transient.
- Architecture followup permits rejecting `UNSPECIFIED + image/gif` rather than
  flattening it. Existing pending rows and opaque A1 uploads are unchanged.
- Original cleanup reconciles signed PUT replays until source upload URL expiry.
  This hardening was added during implementation review; its dedicated regression
  supplements the earlier red/green cycles.
- Independent review found terminal deletion could bypass original reconciliation
  when deletion won before processing. A real PostgreSQL deletion -> PUT replay
  regression failed on the published head, then passed after moving that cleanup
  authority into the shared deleted/expired transaction. No reviewer finding is
  considered resolved until the same reviewer checks the published correction.

## Risks And Follow-Ups

Concurrent deletion, job takeover and response loss must never delete winner
objects or resurrect terminal rows. Runtime/DEV/STAGING/provider activation is
not authorized here; local isolated qualification is separate from acceptance.
Migration DOWN must refuse to discard retained jobs/descriptor/cleanup evidence.
## PR945 corrective review note

Current-source review140 (PR945 comment6418808196, File task c26) invalidates
the earlier assumption that signed-PUT expiry plus one minute proves original
write quiescence. A request accepted earlier can commit after a successful
DELETE. The PR945 repair keeps public-original reconciliation durable beyond
URL expiry, including legacy clock-completed rows; it makes no finite physical
erasure claim without a real storage fence. It also removes promoted
bytes.Buffer.ReadFrom from subprocess captures so the16KiB stdout/stderr bound
applies through os/exec. See be085-catalog-asset-import-plan.md for actual RED,
GREEN and current-code native codec evidence; prior local qualification remains
historical and is not an independent full-source review receipt.
