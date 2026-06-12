import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  const liveTests = [
    'test/gateway_dm_ws_live_integration_test.dart',
    'test/phase1_two_users_e2e_live_test.dart',
    'test/phase1_friends_e2e_live_test.dart',
    'test/phase1_auth_logout_e2e_live_test.dart',
    'test/phase1_ws_resume_e2e_live_test.dart',
    'test/phase2_voice_signaling_e2e_live_test.dart',
    'test/phase3_typing_e2e_live_test.dart',
    'test/phase3_edit_delete_e2e_live_test.dart',
    'test/phase3_delivery_e2e_live_test.dart',
    'test/phase3_dm_requests_e2e_live_test.dart',
    'test/phase3_file_attachment_e2e_live_test.dart',
    'test/phase4_groups_e2e_live_test.dart',
    'test/phase4_group_roles_e2e_live_test.dart',
    'test/phase4_group_voice_e2e_live_test.dart',
    'test/phase4_forward_e2e_live_test.dart',
    'test/phase4_reactions_e2e_live_test.dart',
    'test/phase4_in_app_notifications_e2e_live_test.dart',
    'test/phase5_space_creation_e2e_live_test.dart',
    'test/phase5_space_tree_e2e_live_test.dart',
    'test/phase5_space_invites_e2e_live_test.dart',
    'test/phase5_space_roles_e2e_live_test.dart',
    'test/phase5_space_moderation_e2e_live_test.dart',
    'test/phase5_space_voice_e2e_live_test.dart',
    'test/phase5_space_channel_e2e_live_test.dart',
    'test/phase5_space_shell_e2e_live_test.dart',
    'test/phase6_markdown_e2e_live_test.dart',
    'test/phase6_mentions_e2e_live_test.dart',
    'test/phase6_pins_e2e_live_test.dart',
    'test/phase6_fcm_e2e_live_test.dart',
    'test/phase8_apns_e2e_live_test.dart',
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
