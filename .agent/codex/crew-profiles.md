# Voice Crew Profiles For Codex

Cursor stored these as `.cursor/agents/voice-*.md`. Codex does not consume those
files as native subagents, so use these profiles when delegating work through
Codex task/thread or multi-agent tooling.

## Roster

| Profile | Scope | Key boundaries |
| --- | --- | --- |
| `voice-go-gateway` | `src/backend/gateway/` | REST edge, JWT validation at edge, transcode, rate limiting. Not Realtime WS core or Auth issuance. |
| `voice-go-realtime` | `src/backend/realtime/` | WebSocket, event fan-out, `s`/`resume`, NATS, typing. Not Messaging history catch-up. |
| `voice-go-chat-messaging` | `src/backend/chat/`, `src/backend/messaging/` | DMs, chat list, messages, threads, reactions, cursors per `chat_id`. Not Realtime fan-out. |
| `voice-go-backend` | Other Go services under `src/backend/<service>/` | Use one primary service or tightly coupled pair; no cross-service DB access. |
| `voice-java-auth` | `src/backend/auth/` | Java/Spring Auth, JWT issuance, registration, login, 2FA, guest accounts. Auth stays Java. |
| `voice-flutter` | `src/frontend/` | Flutter client, web/mobile/desktop, UI implementation, tokens. Not Penpot mock authoring. |
| `voice-protos` | `protos/`, buf configs, generated stubs | API contracts, buf lint/format/breaking, Go/Dart generation. No invented RPCs. |
| `voice-verify` | Verification only | Run proportional checks from `docs/TESTING.md`; report evidence. |
| `voice-designer` | Penpot, design tokens, UX review | Product behavior from docs; shipped Penpot snapshots are read-only. |

## Delegation Prompt Shape

```markdown
## Fleet task T-<id>

**Outcome:** <what must be true>
**Scope:** <paths and service>
**Docs:** <specific docs to read>
**Worktree:** <main clone or treehouse path>
**Constraints:** docs-only behavior; no rebase/amend/force-push; no unrelated edits
**Verification:** <commands from docs/TESTING.md>
**Return:** Changed / Checked / Risk / PR / Blockers
```

Use treehouse only when parallel code work needs isolated worktrees. Keep one
agent per worktree path and integrate through git/PR, not file copying.
