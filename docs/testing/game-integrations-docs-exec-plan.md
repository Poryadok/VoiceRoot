# Game integrations documentation ExecPlan

## Purpose

Document a reviewable target for game SDKs, game-owned communities on federated
nodes, and game-to-player bot interactions. A developer must understand identity,
authority, lifecycle, failure behavior, dependencies, and acceptance evidence
without needing the original discussion. This is documentation work, not runtime
activation or a claim that the proposed SDK exists.

## Context

The owner requested a separate worktree and comprehensive documentation after
discussing HerdTrip (2–4 player cooperative sessions), an MMO with corporation
Spaces, and Dejavu (persistent characters receiving commands from a messenger).
The concepts were read from local user-provided files; the resulting documents
must summarize the necessary scenarios without linking to private/local paths.

Baseline: `77ec7240a` on 2026-09-26. See [PLAN](../PLAN.md),
[data model](../DATA_MODEL.md), [architecture](../ARCHITECTURE_REQUIREMENTS.md),
[bots](../features/bots.md), [federation](../features/federation.md), and
[testing](../TESTING.md). Auth remains Java; each service owns its store;
Realtime owns WS; missed message history uses per-chat Messaging REST cursors.

## Scope

In scope: product specification, proposed external contracts, Unity/Unreal SDK
behavior, bot feasibility/source evidence, federation authority and recovery,
developer onboarding, rollout and acceptance matrix, discoverable cross-links.

Out of scope: production code, generated SDKs/protos, DB migrations, running a
game, changing A1/WIP or federation release status, merging to master.
The design distinguishes user requirements, recommended target decisions,
existing implementation, and unresolved activation choices.

## Milestones

1. Establish isolated branch, read canon and inspect bot/federation evidence.
2. Write connected product/API/SDK/bot/federation/acceptance documents.
3. Review authorization, ownership and delivery guarantees; resolve findings.
4. Check links and diff, preserve changes on the dedicated branch/worktree.

## Detailed Steps

- Use the Codex-managed worktree `game-sdk-federation-docs/Voice` and branch
  `codex/game-sdk-federation-docs`; all edits target its absolute paths.
- Read-only audits run independently: `bot_game_audit` (auditor, inherited model
  and effort, focused brief, checkpoint at source capability matrix) and
  `federation_design_review` (auditor, inherited model and effort, focused brief,
  checkpoint at authority/failure contradictions). They do not write files or
  contend for runtime resources.
- Add `features/game-integrations.md` as the entry point; detailed contracts in
  `architecture/game-integration-api.md`, `sdk/game-sdk.md`,
  `features/game-bot-interactions.md`, `architecture/game-federation.md`.
- Add `testing/game-integrations-acceptance.md` with milestone dependencies and
  executable acceptance scenarios for later implementation.
- Link from feature catalog, glossary and relevant existing feature/service
  pages. Keep proposal status explicit rather than marking runtime shipped.
- Run the repository Markdown link checker on changed documents and
  `git diff --check`; inspect scope and source-reference validity. No production
  behavior changes means no new runtime tests under the TDD policy.

## Validation

Documentation checks: relative links, source paths, consistent target/version
labels, no localhost/private concept links, no executable endpoint claims for
new routes, scope still inactive, failure semantics and delivery guarantees.
Source audit is not live E2E proof. Any existing tests run are reported with
their exact scope and limitations.

## Progress

- [x] Read repository workflow and relevant product/architecture context.
- [x] Fetch/fast-forward master; create isolated checkout and dedicated branch.
- [x] Start independent source and federation audits.
- [x] Complete specification set and navigation updates.
- [x] Incorporate two independent reviews and run documentation checks.
- [x] Record evidence and dedicated worktree/branch for handoff.

## Decisions

- SDK and game API can run on the main deployment; federation is optional.
- New behavior is a proposed target, not silently accepted runtime scope.
- Existing Bot API supports a limited text/slash prototype; rich actions and
  reliable game-command processing require explicit extensions.
- External SDK documentation informs competitive requirements only. No Discord
  SDK was downloaded, executed, reverse engineered, or included in Voice.

## Risks And Follow-Ups

- Federation documents disagree on snapshot versus replay; the game proposal
  must choose one recovery model and identify legacy text needing reconciliation.
- Third-party storage cannot provide a cryptographic guarantee of erasure by a
  malicious operator; distinguish receipts from physical proof.
- SDK adoption is not messenger retention; require separate pilot evidence.

## Outcome

Six connected target documents plus this execution record were added. Nine
existing catalog/canon/backlog documents link to the design and preserve current
runtime/deferred status. The source audit establishes a text/slash prototype path
and records missing scopes, durable command delivery, cards and proactive DM.

Review corrections: three total webhook attempts, explicit send-scope gap,
operation/command mapping and immutable result CAS; central account-wide voice
admission; existing ≤60s LiveKit bearer limitation versus the new node verifier;
single master Chat metadata writer; command admission/revoke linearization and
bounded in-flight completion without a distributed atomic-commit promise.

Validation: Node 24.15.0; `rtk proxy npx --yes markdown-link-check@3.12.2 -q -c
.markdown-link-check.json <file>` succeeded for all 16 changed/new Markdown
documents. The repository config ignores external HTTP(S) links and anchors;
the check proves local path integrity, not remote availability. New local anchor
targets were inspected separately. `rtk git diff --check` passed; staged check
is performed before commit. No code/proto/migrations changed, no runtime or
engine/live tests were run, no claim of SDK/federation production readiness.

Worktree: `C:/Users/Sergey/.codex/worktrees/game-sdk-federation-docs/Voice`.
Branch: `codex/game-sdk-federation-docs`. Main checkout remains on master;
the user's untracked local concept directory was not copied or modified.

Follow-up owner clarification (2026-09-26): recommend first-entry Voice service
disclosure with the same account-connect option for everyone. Accepted a distinct
game identity (not legacy guest) and conversion to both new and existing permanent
accounts. Added a proposed durable conversion flow, trust boundaries, conflict and
recovery requirements, SDK behavior, glossary, and acceptance cases ID07–ID13.
G01 now tracks implementation policy details, not whether these paths are needed.
Scoped local Markdown link checks and diff whitespace checks passed; runtime
remains unchanged and untested by this documentation-only follow-up.
