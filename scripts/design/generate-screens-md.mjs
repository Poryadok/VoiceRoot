import fs from 'node:fs';

const inventoryPath = process.argv[2];
const outPath = process.argv[3] ?? 'docs/design/screens.md';
const raw = fs.readFileSync(inventoryPath, 'utf8');
const inv = JSON.parse(raw).result.filter((item) => !item.penpotName.includes('·'));

const flutterMap = {
  'Screen/Auth/Login': 'auth/auth_screen.dart',
  'Screen/Auth/GuestNickname': 'auth/guest_nickname_screen.dart',
  'Screen/Shell/Desktop': 'shell/three_column_shell.dart',
  'Screen/Shell/Mobile': 'shell/three_column_shell.dart',
  'Screen/Shell/MobileChatOpen': 'shell/three_column_shell.dart',
  'Screen/Chat/List': 'chat/chat_list_panel.dart',
  'Screen/Chat/Room': 'chat/chat_room_panel.dart',
  'Screen/Social/Panel': 'social/social_panel.dart',
  'Screen/Settings/Privacy': 'settings/privacy_settings_screen.dart',
  'Screen/Settings/Security': 'settings/security_settings_screen.dart',
  'Screen/Settings/Notifications': 'settings/notification_settings_screen.dart',
  'Screen/Settings/Subscription': 'settings/subscription_settings_screen.dart',
  'Screen/Matchmaking/GameCatalog': 'matchmaking/game_catalog_screen.dart',
  'Screen/Matchmaking/GameDetail': 'matchmaking/game_detail_screen.dart',
  'Screen/Matchmaking/QueueSearch': 'matchmaking/queue_search_screen.dart',
  'Screen/Matchmaking/MatchSquad': 'matchmaking/match_squad_screen.dart',
  'Screen/Matchmaking/MatchHistory': 'matchmaking/match_history_screen.dart',
  'Screen/Stories/Create': 'stories/story_create_screen.dart',
  'Screen/Stories/Viewer': 'stories/story_viewer_screen.dart',
  'Screen/Stories/Archive': 'stories/story_archive_screen.dart',
  'Screen/Stories/Highlights': 'stories/story_highlights_screen.dart',
  'Screen/Bots/Install': 'bots/bot_install_page.dart',
  'Screen/Profile/DowngradePicker': 'profile/profile_downgrade_picker_screen.dart',
  'State/Chat/Empty': 'core/voice_state_panel.dart',
  'State/Chat/Error': 'core/voice_state_panel.dart',
  'State/Network/Offline': 'core/voice_compact_banner.dart',
  'Overlay/Call/Incoming': 'call/incoming_call_overlay.dart',
  'Overlay/Call/Outgoing': 'call/outgoing_call_overlay.dart',
  'Overlay/Call/Active': 'call/call_modal_overlay.dart',
  'Overlay/Matchmaking/MatchFound': 'matchmaking/match_found_overlay.dart',
  'Overlay/Matchmaking/Rating': 'matchmaking/match_rating_overlay.dart',
  'Overlay/Onboarding/CoachMarks': 'onboarding/onboarding_overlay.dart',
  'Overlay/Version/ForceUpdate': 'version/version_policy_overlay.dart',
  'Panel/Chat/Info': 'chat/chat_info_panel.dart',
  'Panel/Chat/Thread': 'chat/thread_side_panel.dart',
  'Panel/Search/Global': 'search/global_search_panel.dart',
  'Panel/Space/Tree': 'space/space_tree_panel.dart',
  'Panel/Call/ScreenShare': 'call/screen_share_panel.dart',
  'Panel/Shell/Navigation': 'shell/navigation_panel.dart',
  'Panel/Shell/SideHost': 'shell/side_panel.dart',
  'Panel/Settings/Sheet': 'settings/settings_sheet.dart',
  'Panel/Settings/Help': 'settings/help_sheet.dart',
  'Panel/Settings/Verification': 'settings/verification_settings_sheet.dart',
  'Panel/Profile/Create': 'profile/create_profile_sheet.dart',
  'Panel/Profile/Edit': 'profile/profile_edit_sheet.dart',
  'Panel/Auth/GuestConvert': 'auth/guest_convert_sheet.dart',
  'Panel/Space/Create': 'space/create_space_sheet.dart',
  'Panel/Space/JoinInvite': 'space/join_space_invite_sheet.dart',
  'Panel/Space/Invites': 'space/space_invites_sheet.dart',
  'Panel/Space/Members': 'space/space_members_sheet.dart',
  'Panel/Space/Bots': 'space/space_bots_sheet.dart',
  'Panel/Space/Roles': 'space/space_roles_sheet.dart',
  'Panel/Space/RoleEditor': 'space/space_role_editor_sheet.dart',
  'Panel/Space/ChatOverride': 'space/space_chat_override_sheet.dart',
  'Panel/Space/ChatSlowMode': 'space/space_chat_slow_mode_sheet.dart',
  'Panel/Space/VoiceRoomOverride': 'space/space_voice_room_override_sheet.dart',
  'Panel/Chat/CreateGroup': 'chat/create_group_sheet.dart',
  'Panel/Chat/GroupMembers': 'chat/group_members_sheet.dart',
  'Panel/Chat/ForwardMessage': 'chat/forward_message_sheet.dart',
  'Panel/Chat/SlashCommandMenu': 'chat/slash_command_menu.dart',
  'Panel/Chat/SlashCommandOptions': 'chat/slash_command_options_sheet.dart',
  'Panel/Social/ProfileDetail': 'social/profile_detail_sheet.dart',
  'Panel/Stories/HighlightEdit': 'stories/highlight_edit_sheet.dart',
  'Panel/Stories/StoryViewers': 'stories/story_viewers_sheet.dart',
  'Panel/Matchmaking/PlayerProfile': 'matchmaking/player_profile_sheet.dart',
  'Panel/Report/Sheet': 'report/report_sheet.dart',
};

function normalizeId(item) {
  if (item.screenId.includes('·')) {
    return item.penpotName.replace(/ \//g, '/').replace(/ /g, '');
  }
  return item.screenId;
}

const grouped = {};
for (const item of inv) {
  const id = normalizeId(item);
  if (!grouped[id]) grouped[id] = [];
  grouped[id].push(item);
}

const rows = Object.entries(grouped)
  .sort(([a], [b]) => a.localeCompare(b))
  .map(([id, frames]) => {
    const desktop =
      frames.find((f) => f.w >= 960) ||
      frames.find((f) => f.pageName.includes('Desktop') && f.w >= 480) ||
      frames[0];
    const mobile = frames.find((f) => f.w <= 420 && f.h >= 800);
    let viewer = `[desktop](${desktop.viewer})`;
    if (mobile && mobile.frameId !== desktop.frameId) {
      viewer += ` · [mobile](${mobile.viewer})`;
    }
    const flutterPath = flutterMap[id];
    const flutter = flutterPath
      ? `[${flutterPath.split('/').pop()}](../../src/frontend/lib/ui/${flutterPath})`
      : '';
    const notes = desktop.pageName;
    return `| \`${id}\` | \`${desktop.frameId}\` | ${viewer} | ${flutter} | ${notes} |`;
  });

const pages = [
  ['00_References', '20d3f736-cc1b-8043-8008-561cb65228f0', 'Discord / external refs'],
  ['01_Foundation', '6d4c4410-c47e-8083-8008-561ce95f11e2', 'Design tokens + components'],
  ['10_Screens_Desktop', '6d4c4410-c47e-8083-8008-561cf0765607', 'Full-screen frames 1280×800'],
  ['11_Screens_Mobile', '6d4c4410-c47e-8083-8008-561cf5662204', 'Full-screen frames 390×844'],
  ['12_States', '6d4c4410-c47e-8083-8008-561cfaa8677c', 'Empty / error / offline'],
  ['13_Panels_Desktop', '6d4c4410-c47e-8083-8008-564229c3b00f', 'Panels + sheets in desktop shell context'],
  ['14_Panels_Mobile', '6d4c4410-c47e-8083-8008-564229f6af85', 'Panels + sheets as mobile bottom sheets'],
  ['15_Overlays', '6d4c4410-c47e-8083-8008-56422a11288b', 'Call / match / onboarding / update overlays'],
];

let md = `# Screen inventory (Penpot ↔ Flutter)

**File ID:** \`20d3f736-cc1b-8043-8008-561cb65228ef\`  
**Setup:** [penpot-setup.md](penpot-setup.md)

Viewer URL pattern: \`https://design.penpot.app/#/viewer/{fileId}/{pageId}/{frameId}\`

В задачи и PR — **viewer URL** фрейма или frame ID. Для новых экранов UX — [brand.md](brand.md).

| Screen ID | Penpot frame ID | Viewer (desktop / mobile) | Flutter / spec | Notes |
|-----------|-----------------|----------------------------|----------------|-------|
${rows.join('\n')}

## Penpot pages

| Page | Page ID | Purpose |
|------|---------|---------|
${pages.map(([name, id, purpose]) => `| \`${name}\` | \`${id}\` | ${purpose} |`).join('\n')}

## Legacy Figma

Historical inventory: [figma-setup.md](figma-setup.md) (archive).
`;

fs.writeFileSync(outPath, md);
console.log(`Wrote ${rows.length} rows to ${outPath}`);
