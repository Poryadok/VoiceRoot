import '../../backend/user_privacy_client.dart';

/// Default field values per privacy preset (aligned with docs/features/privacy.md).
class PrivacyPresetDefaults {
  const PrivacyPresetDefaults._();

  static const presets = ['personal', 'gaming', 'work'];

  static VoicePrivacySettings forPreset(String preset, {required String profileId}) {
    return switch (preset) {
      'personal' => VoicePrivacySettings(
        profileId: profileId,
        preset: 'personal',
        showOnline: VoicePrivacyAudience.friendsOnly,
        showGameStatus: VoicePrivacyAudience.friendsOnly,
        showMmRating: VoicePrivacyAudience.friendsAndFoF,
        showPhone: VoicePrivacyAudience.nobody,
        showStories: VoicePrivacyAudience.friendsAndFoF,
        allowPhoneSearch: VoicePrivacyAudience.friendsOnly,
        allowDm: VoicePrivacyAudience.friendsAndFoF,
        allowCalls: VoicePrivacyAudience.friendsOnly,
        allowChatSpaceInvites: VoicePrivacyAudience.friendsOnly,
        allowFiles: VoicePrivacyAudience.friendsAndFoF,
        allowVoiceMessages: VoicePrivacyAudience.friendsOnly,
        allowFriendRequests: VoicePrivacyAudience.everyoneWithGuests,
        allowGuestDm: false,
      ),
      'work' => VoicePrivacySettings(
        profileId: profileId,
        preset: 'work',
        showOnline: VoicePrivacyAudience.spaceMembersOnly,
        showGameStatus: VoicePrivacyAudience.nobody,
        showMmRating: VoicePrivacyAudience.nobody,
        showPhone: VoicePrivacyAudience.nobody,
        showStories: VoicePrivacyAudience.nobody,
        allowPhoneSearch: VoicePrivacyAudience.nobody,
        allowDm: VoicePrivacyAudience.spaceMembersAndFriends,
        allowCalls: VoicePrivacyAudience.spaceMembersAndFriends,
        allowChatSpaceInvites: VoicePrivacyAudience.nobody,
        allowFiles: VoicePrivacyAudience.spaceMembersAndFriends,
        allowVoiceMessages: VoicePrivacyAudience.spaceMembersAndFriends,
        allowFriendRequests: VoicePrivacyAudience.spaceMembersOnly,
        allowGuestDm: false,
      ),
      _ => VoicePrivacySettings(
        profileId: profileId,
        preset: 'gaming',
        showOnline: VoicePrivacyAudience.everyoneWithGuests,
        showGameStatus: VoicePrivacyAudience.everyoneWithGuests,
        showMmRating: VoicePrivacyAudience.everyoneWithGuests,
        showPhone: VoicePrivacyAudience.nobody,
        showStories: VoicePrivacyAudience.everyoneWithGuests,
        allowPhoneSearch: VoicePrivacyAudience.friendsOnly,
        allowDm: VoicePrivacyAudience.everyoneWithGuests,
        allowCalls: VoicePrivacyAudience.friendsAndFoF,
        allowChatSpaceInvites: VoicePrivacyAudience.friendsAndFoF,
        allowFiles: VoicePrivacyAudience.friendsAndFoF,
        allowVoiceMessages: VoicePrivacyAudience.friendsAndFoF,
        allowFriendRequests: VoicePrivacyAudience.everyoneWithGuests,
        allowGuestDm: true,
      ),
    };
  }
}
