# ExecPlan: Product question triage

## Purpose

Resolve repository product questions whose answers follow directly from the existing
product strategy, established UX conventions, or safe reversible defaults. Record
those answers in canonical feature docs and leave the owner only choices with a
material product, privacy, data-loss, legal, or irreversible architecture tradeoff.

## Context

- `docs/PLAN.md` defines H1 and requires one precise owner question only for missing
  or conflicting product semantics.
- `docs/todo/product-roadmap.md` contains the known cross-feature decision backlog.
- `docs/features/*` is the canonical product behavior.
- The user explicitly delegated obvious decisions and asked for a question/answer
  table plus only genuinely non-obvious remaining questions.
- Existing unrelated worktree changes, including `.cache/`, must be preserved.

## Scope

- In: explicit undecided/TBD/deferred product-policy items in feature docs and the
  product roadmap; documentation-only decisions and consistency cleanup.
- Out: code implementation, provider credentials, release acceptance, visual taste,
  external legal decisions, and federation.
- Documentation gaps: retain as H1 only when alternatives have materially different
  user, privacy, recovery, cost, or platform consequences.

## Milestones

- [x] Build an inventory of explicit product-policy gaps.
- [x] Classify each gap as obvious/reversible or genuine owner choice.
- [x] Record obvious decisions in canonical feature docs and clean roadmap entries.
- [x] Validate links/text consistency and present the decision table plus remaining H1 questions.

## Detailed Steps

1. Search `docs/features/*`, `docs/todo/product-roadmap.md`, and `docs/PLAN.md` for
   explicit TBD, deferred decision, conflict, and missing-policy markers.
2. Read each referenced canonical feature section and related strategy constraints.
3. Choose defaults only where the existing canon makes one option clearly safer or
   more internally consistent; document the reason in this plan.
4. Apply narrow documentation patches and remove/close resolved roadmap decision items.
5. Run targeted `rg` checks and inspect `git diff --check` plus the final diff.

## Validation

- [x] `rg` shows no stale unresolved marker for decisions closed in this pass; references
  to obsolete auto-unarchive remain only where they describe the implementation gap.
- [x] `git diff --check` passes.
- [x] Final diff changes documentation only and preserves unrelated `.cache/` work.

## Progress

- [x] Owner decisions recorded for archived-chat notifications and DM subtitle priority.
- [x] Inventory and classification complete.
- [x] Obvious/reversible decisions recorded in canonical docs.
- [x] Final consistency validation and owner handoff complete.

## Decisions

- Archived chats: keep archived, update unread badge only, no push/full in-app delivery
  (explicit owner decision).
- DM subtitle priority: combined online+call, DND, custom status, online/idle,
  last-seen, then privacy-safe offline (explicit owner decision).
- Active strip: retain a chat while unread exists (explicit owner decision).
- Pinned bar: cycle newest to oldest on repeated taps; a separate full-list action remains.
- Location messages: Shared Media → Media because the primary artifact is a visual map
  preview and a separate low-volume tab would add unnecessary IA.
- DM requests, moderated user game requests, and Space matchmaking are current scope;
  the contrary roadmap text was stale against PLAN and shipped evidence.
- Guest admission defaults fail-closed and requires explicit Space/chat opt-in plus invite.
- Article metadata fetch belongs to a Messaging background worker, matching text-chat canon.
- Standard account discriminators remain random; custom discriminator selection is out of scope.
- Product label is «Спейс»; contract identifier remains `space`.
- Owner accepted all four recommended choices: verification-gated regular rights,
  composable AND Space entry policy, 30-day account recovery then erasure/pseudonymization,
  and 7-day hidden/frozen Space recovery then irreversible purge.
- `allow_guests=false` is explicitly confirmed for Spaces and group/channel chats.
- T-056 is resolved as the minimum A1 account soft-delete contract; complete erasure
  remains A4, and the fleet blocker is removed in documentation only.

## Risks And Follow-Ups

- Some TODO entries describe implementation gaps rather than product questions; do not
  convert them into owner decisions.
- Broad text searches produce false positives; only explicit policy gaps are in scope.
- SMS/GIF provider activation and major mobile IA remain deferred until their milestones;
  they do not block current alpha work and are not useful owner questions now.
