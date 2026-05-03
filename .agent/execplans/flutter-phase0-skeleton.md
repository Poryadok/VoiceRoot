# ExecPlan: Flutter Phase-0 skeleton

## Purpose

Deliver a runnable Flutter app in `src/frontend/` with: three-column desktop shell (active rail | chat list | main per `docs/features/navigation.md`), configurable API base URL, thin HTTP client for Gateway `GET /health` and `GET /api/v1/version`, Riverpod state; `flutter analyze` / `flutter test` green; CI job on PR/push.

## Context

- PLAN Phase 0 checklist: three-column layout, backend, DI, state.
- `docs/features/navigation.md` — desktop three columns.
- `docs/microservices/api-gateway.md` — public `/health`, `/api/v1/version`.
- `docs/TESTING.md` — `flutter test`, `flutter analyze`.
- TDD workflow: `.agent/workflows/tdd-code-workflow/SKILL.md` (strict).

## Scope

In: scaffold, unit/widget tests, `lib/` implementation, `.github/workflows/ci.yml` Flutter job.
Out: i18n, login UI, integration_test against staging, `docs/` edits unless requested.

## Milestones

1. `flutter create` + pubspec deps.
2. Failing tests for shell, config, client, providers.
3. Green implementation (RGR cycles).
4. CI Flutter job.

## Detailed steps

See approved Cursor plan `flutter_skeleton_tdd` (not edited by agents). Implementation order: shell → `GatewayConfig` → `VoiceGatewayClient` → Riverpod `ProviderScope` + health provider → refactor.

## Validation

- `cd src/frontend; flutter analyze`
- `cd src/frontend; flutter test`
- CI: new job passes.

## Progress

- [x] Scaffold
- [x] Tests
- [x] Lib
- [x] CI

## Decisions

- Riverpod + `http`; base URL via `String.fromEnvironment('VOICE_API_BASE_URL', defaultValue: '')`.
- Keys: `nav_active_rail`, `nav_chat_list`, `nav_open_chat` for widget tests.

## Risks

- CI Flutter SDK version drift vs README 3.41.7 — pin in workflow if needed.
