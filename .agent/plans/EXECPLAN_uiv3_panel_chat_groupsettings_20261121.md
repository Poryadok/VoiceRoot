# ExecPlan: Panel/Chat/GroupSettings v3 destination (UIV3-PANEL-CHAT-GROUPSETTINGS)

Single milestone-sized deliverable. No new branches/milestones. Worktree:
`tmp/uiv3-groupsettings-20261121` @ `7d7808a` (branch `codex/uiv3-panel-chat-groupsettings-20261121`).

## Objective
- Implement v3 destination `Panel / Chat / GroupSettings` (mockup `group-settings`, §10.1c:
  "Identity, admins, add-member policy and threads") with all 6 §10.1c controls wired
  to existing Chat Service state/API, Chat Info entry (#21), behavior + H/V capture tests.

## Ground truth (verified this session)
- Controls canon: `docs/design/screen-controls.md` §10.1c (Close/Back; edit name owner/admin;
  assign admins owner promote/demote; who can add members; threads; Save when changes).
  Entry: Chat Info #21 `Group settings | H+V | GRP owner/admin | Open Panel/Chat/GroupSettings`.
- Rights canon: `docs/features/text-chat.md` §Group Rights; admin promote member→admin allowed,
  admin demoting another admin forbidden, owner may (NewAdminRights.md); admins edit settings.
- API: `voiceChatsClientProvider.updateGroup` (name/threads/`memberAdditionsEnabled(member|admins)`),
  `listGroupMembers` page size 500 (500/500 tooltip cap source), roles constants
  `kChatRoleOwner/Admin/Member`; canonical role route `PATCH /api/v1/chats/{id}/members/{profile}/role`
  bound in `src/backend/gateway/transcode_chats.go:284-286` (proto `SetGroupMemberRoleRequest`).
- Mockup composition (`settingsBody('group')`): identity card (letter monogram avatar + name +
  "Изменить имя и аватар" + edit icon-btn), section-label Настройки + 3 toggles (Треды=on,
  Publications participants Включено/Только администраторы, Adding members), foot Cancel+Save.
  Mockup does NOT render admins rows despite desc + §10.1c → gap recorded in PR body.

## Design decisions
- Clone `ChannelSettingsPanel` destination family (loading skeleton, load-error+retry panel,
  Cancel+Save foot with Save gated by `hasChanges`, drafts, shrink on lost authority,
  reload-and-reflect after save) → `GroupSettingsPanel`.
- New `setGroupMemberRole` wrapper in `chats_client.dart` for the admins control.
- `groupSettingsAuthorityProvider(chatId)` + `groupRosterProvider(chatId)` in
  `src/frontend/lib/state/group_settings_provider.dart` (full pagination, page size 500).
- `StandaloneGroupSettingsEntry` in `chat_info_panel.dart` mirroring
  `StandaloneChannelSettingsEntry`, inserted in `ChatInfoPanel._stateLayout`.
- Identity: card avatar (letter monogram from name) + name + inline name field (mockup affordance)
  → Save → `updateGroup(name:)`; name re-read from chat list after save. No image-upload/avatar
  URL UI is invented (no picker source contract; text-chat L586 says "Edit group name").
- Toggles: Треды→threadsEnabled; Publications participants→allowUserMainFeed (mockup row 2 for
  group); Adding members→memberAdditionsEnabled. Admins rows = immediate
  `setGroupMemberRole` (promote/demote), owner/admin visibility per rights canon.

## Files (write scope)
1. `src/frontend/lib/ui/chat/group_settings_panel.dart` (new)
2. `src/frontend/lib/state/group_settings_provider.dart` (new)
3. `src/frontend/lib/backend/chats_client.dart` (+`setGroupMemberRole` only)
4. `src/frontend/lib/ui/chat/chat_info_panel.dart` (entry widget + insertion)
5. `src/frontend/lib/l10n/app_localizations_en.dart` + `_ru.dart` (new keys)
6. `src/frontend/test/group_settings_panel_test.dart`, `src/frontend/test/group_settings_capture_test.dart`

## Verification gate
- 2 new flutter tests green from `src/frontend`; touched-file analyze clean (client).
- Capture PNGs 1280x800 + 390x844 produced (no horizontal overflow assertions).
- PR body: acceptance mapping, evidence, mockup admins gap, contract gaps; PR open, not merged.
- TT: task pr, comment, checkpoint `Codex-UI-GroupSettings`; status awaiting PR CI/merge.

## Progress
- [x] Worktree synced with origin (fast-forward merge origin/master 7d7808a→a4a43e8, docs-only
  delta; frontend base unchanged; branch not rebased/force-pushed).
- [x] Canons read (screen-controls §10.1c, text-chat Group Rights, mockup composition).
- [x] Read ChatInfoPanel host + construct site; implemented provider/wrapper/panel/entry/l10n
  (group_settings_provider.dart authority gate, GroupSettingsPanel state machine cloned from
  ChannelSettingsPanel, StandaloneGroupSettingsEntry wired into chat_info_panel.dart, 7 groupSettings
  l10n keys in EN/RU, setGroupMemberRole added to VoiceChatsClient).
- [x] Tests written and executed green from src/frontend:
  `flutter test test/group_settings_panel_test.dart` → 11/11 passed (threads staging, inline rename,
  owner promote, admin cannot demote, owner demote, additions row, save failure retry, cancel, paged
  roster, hidden scenarios, load-error retry recovery).
  `flutter test test/group_settings_capture_test.dart` → 1/1 passed; PNGs written via
  VOICE_GROUP_SETTINGS_CAPTURE_DIR (wide 1280x800 SidePanelHost inline, narrow 390x844 ChatInfoPanel
  BottomSheet).
  Regression: `flutter test test/chat_info_panel_test.dart test/chat_info_panel_e2e_test.dart` → 33/33
  passed; `flutter test test/e2e_chat_settings_test.dart` → 13/13 passed.
  Scoped `dart analyze` on touched files: 0 errors/0 warnings (final run clean, then "No issues found").
- [ ] Commit, push, PR open; TT pr/comment/checkpoint.

## Review
- (pending)

## Return
- (pending)
