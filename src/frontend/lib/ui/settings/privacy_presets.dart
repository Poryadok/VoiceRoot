import '../../backend/user_privacy_client.dart';

/// Default field values per privacy preset (aligned with user service integration tests).
class PrivacyPresetDefaults {
  const PrivacyPresetDefaults._();

  static const presets = ['personal', 'gaming', 'work'];

  static VoicePrivacySettings forPreset(String preset, {required String profileId}) {
    return switch (preset) {
      'personal' => VoicePrivacySettings(
        profileId: profileId,
        preset: 'personal',
        showOnline: 'friends',
        showGameStatus: 'friends',
        showMmRating: 'friends_of_friends',
        showPhone: 'nobody',
        showStories: 'friends_of_friends',
        allowDm: 'friends_of_friends',
        allowFriendRequests: 'everyone',
        allowGuestDm: false,
      ),
      'work' => VoicePrivacySettings(
        profileId: profileId,
        preset: 'work',
        showOnline: 'friends_of_friends',
        showGameStatus: 'nobody',
        showMmRating: 'nobody',
        showPhone: 'nobody',
        showStories: 'nobody',
        allowDm: 'friends_of_friends',
        allowFriendRequests: 'friends_of_friends',
        allowGuestDm: false,
      ),
      _ => VoicePrivacySettings(
        profileId: profileId,
        preset: 'gaming',
        showOnline: 'everyone',
        showGameStatus: 'everyone',
        showMmRating: 'everyone',
        showPhone: 'nobody',
        showStories: 'everyone',
        allowDm: 'everyone',
        allowFriendRequests: 'everyone',
        allowGuestDm: true,
      ),
    };
  }
}

/// Audience values accepted by the User privacy API.
const List<String> kPrivacyAudienceValues = [
  'everyone',
  'friends',
  'friends_of_friends',
  'nobody',
];

const List<String> kPrivacyPhoneAudienceValues = [
  'friends',
  'friends_of_friends',
  'nobody',
];

const List<String> kPrivacyFriendRequestAudienceValues = [
  'everyone',
  'friends_of_friends',
  'nobody',
];
