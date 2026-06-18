import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/user_privacy_client.dart';
import 'package:voice_frontend/ui/stories/story_visibility.dart';

void main() {
  group('storyVisibilityFromPrivacyAudience', () {
    test('maps everyone shortcut to everyone', () {
      expect(
        storyVisibilityFromPrivacyAudience(
          VoicePrivacyAudience.everyoneWithGuests,
        ),
        'everyone',
      );
    });

    test('maps friends only to friends', () {
      expect(
        storyVisibilityFromPrivacyAudience(VoicePrivacyAudience.friendsOnly),
        'friends',
      );
    });

    test('maps friends and FoF to close_friends', () {
      expect(
        storyVisibilityFromPrivacyAudience(VoicePrivacyAudience.friendsAndFoF),
        'close_friends',
      );
    });
  });
}
