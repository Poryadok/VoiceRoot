import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  const liveTests = [
    'test/gateway_dm_ws_live_integration_test.dart',
    'test/dm_two_users_e2e_live_test.dart',
    'test/friends_e2e_live_test.dart',
    'test/auth_logout_e2e_live_test.dart',
    'test/ws_resume_e2e_live_test.dart',
    'test/avatar_e2e_live_test.dart',
    'test/presence_e2e_live_test.dart',
    'test/voice_call_signaling_e2e_live_test.dart',
    'test/message_typing_e2e_live_test.dart',
    'test/message_edit_delete_e2e_live_test.dart',
    'test/message_delivery_e2e_live_test.dart',
    'test/dm_requests_e2e_live_test.dart',
    'test/file_attachment_e2e_live_test.dart',
    'test/file_image_thumb_e2e_live_test.dart',
    'test/file_clamav_infected_e2e_live_test.dart',
    'test/groups_e2e_live_test.dart',
    'test/group_roles_e2e_live_test.dart',
    'test/group_voice_e2e_live_test.dart',
    'test/forward_messages_e2e_live_test.dart',
    'test/reactions_e2e_live_test.dart',
    'test/in_app_notifications_e2e_live_test.dart',
    'test/spaces_creation_e2e_live_test.dart',
    'test/spaces_tree_e2e_live_test.dart',
    'test/spaces_invites_e2e_live_test.dart',
    'test/spaces_roles_e2e_live_test.dart',
    'test/custom_roles_e2e_live_test.dart',
    'test/shared_media_e2e_live_test.dart',
    'test/threads_e2e_live_test.dart',
    'test/screen_share_e2e_live_test.dart',
    'test/spaces_moderation_e2e_live_test.dart',
    'test/spaces_voice_e2e_live_test.dart',
    'test/spaces_channel_e2e_live_test.dart',
    'test/spaces_shell_e2e_live_test.dart',
    'test/spaces_slow_mode_e2e_live_test.dart',
    'test/markdown_e2e_live_test.dart',
    'test/mentions_e2e_live_test.dart',
    'test/spaces_channel_mentions_e2e_live_test.dart',
    'test/pins_e2e_live_test.dart',
    'test/fcm_delivery_e2e_live_test.dart',
    'test/apns_e2e_live_test.dart',
    'test/fcm_android_e2e_live_test.dart',
    'test/mobile_layout_e2e_live_test.dart',
    'test/offline_cache_e2e_live_test.dart',
    'test/windows_version_e2e_live_test.dart',
    'test/voip_e2e_live_test.dart',
    'test/matchmaking_queue_e2e_live_test.dart',
    'test/matchmaking_e2e_live_test.dart',
    'test/matchmaking_history_e2e_live_test.dart',
    'test/game_catalog_e2e_live_test.dart',
    'test/matchmaking_fcm_e2e_live_test.dart',
    'test/search_e2e_live_test.dart',
    'test/trust_e2e_live_test.dart',
    'test/privacy_actions_e2e_live_test.dart',
    'test/e2e_dm_live_test.dart',
    'test/e2e_optout_live_test.dart',
    'test/e2e_key_backup_live_test.dart',
    'test/e2e_edit_live_test.dart',
    'test/e2e_file_live_test.dart',
    'test/e2e_shared_media_live_test.dart',
    'test/bots_slash_e2e_live_test.dart',
  ];

  for (final path in liveTests) {
    test('live e2e entry point exists: $path', () {
      expect(
        File(path).existsSync(),
        isTrue,
        reason: 'opt-in live E2E at $path',
      );
    });
  }
}
