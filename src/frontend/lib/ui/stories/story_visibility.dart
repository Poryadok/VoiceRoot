import '../../backend/user_privacy_client.dart';

/// Maps user privacy `show_stories` audience to story visibility wire value.
/// Mirrors backend `audienceToStoryVisibility` (story/internal/grpcsvc/audience.go).
String storyVisibilityFromPrivacyAudience(VoicePrivacyAudience audience) {
  if (audience.isEveryoneShortcut) return 'everyone';
  if (audience.isNobody) return 'friends';
  if (audience.friends &&
      audience.friendsOfFriends &&
      !audience.spaceMembers &&
      !audience.includeGuests &&
      audience.spaceIds.isEmpty) {
    return 'close_friends';
  }
  if (audience.friends &&
      !audience.friendsOfFriends &&
      !audience.spaceMembers &&
      !audience.includeGuests &&
      audience.spaceIds.isEmpty) {
    return 'friends';
  }
  return 'friends';
}
