import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/user_privacy_client.dart';
import 'package:voice_frontend/ui/settings/privacy_presets.dart';

void main() {
  test('VoicePrivacyAudience JSON round-trip', () {
    const audience = VoicePrivacyAudience(
      friends: true,
      friendsOfFriends: true,
      spaceMembers: true,
      spaceIds: ['space-1'],
      includeGuests: false,
    );
    final decoded = voicePrivacyAudienceFromJson({
      'friends': true,
      'friends_of_friends': true,
      'space_members': true,
      'space_ids': ['space-1'],
      'include_guests': false,
    });
    expect(decoded, audience);
  });

  test('work preset show_online is space members only', () {
    final settings = PrivacyPresetDefaults.forPreset('work', profileId: 'p-1');
    expect(settings.showOnline.spaceMembers, isTrue);
    expect(settings.showOnline.friends, isFalse);
    expect(settings.allowDm.spaceMembers, isTrue);
    expect(settings.allowDm.friends, isTrue);
  });
}
