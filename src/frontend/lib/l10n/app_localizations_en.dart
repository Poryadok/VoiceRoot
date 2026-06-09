// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class AppLocalizationsEn extends AppLocalizations {
  AppLocalizationsEn([String locale = 'en']) : super(locale);

  @override
  String get appTitle => 'Voice';

  @override
  String get gatewayStatusOk => 'Gateway: ok';

  @override
  String get gatewayStatusChecking => 'Gateway: checking…';

  @override
  String get gatewayMissingBaseUrl => 'Gateway: missing base URL';

  @override
  String gatewayStatusError(String error) {
    return 'Gateway: error ($error)';
  }

  @override
  String gatewayStatusFailure(String detail) {
    return 'Gateway: $detail';
  }

  @override
  String get authTitle => 'Sign in to Voice';

  @override
  String get authEmailLabel => 'Email';

  @override
  String get authPasswordLabel => 'Password';

  @override
  String get authPasswordHelper => 'At least 8 characters';

  @override
  String get authErrorEmptyFields => 'Enter your email and password.';

  @override
  String get authErrorPasswordTooShort =>
      'Password must be at least 8 characters.';

  @override
  String get authErrorValidationFailed =>
      'Use a valid email and a password of at least 8 characters.';

  @override
  String get authErrorRateLimited =>
      'Too many attempts. Please wait and try again.';

  @override
  String get authErrorInvalidCredentials => 'Incorrect email or password.';

  @override
  String get authLogin => 'Log in';

  @override
  String get authRegister => 'Register';

  @override
  String get authLogout => 'Log out';

  @override
  String authError(String message) {
    return 'Auth error: $message';
  }

  @override
  String authSessionProfile(String profileId) {
    return 'Profile: $profileId';
  }

  @override
  String authSessionHandle(String handle) {
    return '$handle';
  }

  @override
  String get socialDiscoverHint => 'Find people — use the icon on the left';

  @override
  String get backendUnavailable =>
      'Social and chat features are unavailable. Start the full API stack (docker compose --profile app).';

  @override
  String get socialTabSearch => 'Search';

  @override
  String get socialTabFriends => 'Friends';

  @override
  String get socialTabRequests => 'Requests';

  @override
  String get socialSearchHint => 'Search by name or @username';

  @override
  String get socialSearchStart => 'Search for people';

  @override
  String get socialSearchStartHint =>
      'Enter a name or @username to start a conversation.';

  @override
  String get socialSearchEmpty => 'No profiles found';

  @override
  String get socialSearchEmptyHint =>
      'Check the spelling or try another handle.';

  @override
  String get socialSearchLoading => 'Searching…';

  @override
  String get socialAddFriend => 'Add friend';

  @override
  String get socialAcceptRequest => 'Accept';

  @override
  String get socialDeclineRequest => 'Decline';

  @override
  String get socialRequestPending => 'Request pending';

  @override
  String get socialFriendsEmpty => 'No friends yet';

  @override
  String get socialRequestsEmpty => 'No friend requests';

  @override
  String get socialIncomingRequests => 'Incoming';

  @override
  String get socialOutgoingRequests => 'Outgoing';

  @override
  String get socialFriendsLoadError => 'Could not load friends';

  @override
  String get socialFriendsBackendUnavailable =>
      'Friends are unavailable. Start the full API stack (docker compose --profile app).';

  @override
  String get socialRequestsLoadError => 'Could not load requests';

  @override
  String get socialProfileLoadError => 'Could not load profile';

  @override
  String get socialPresenceOnline => 'Online';

  @override
  String get socialPresenceIdle => 'Idle';

  @override
  String get socialPresenceDnd => 'Do not disturb';

  @override
  String get socialPresenceOffline => 'Offline';

  @override
  String socialPresenceLastSeen(String dateTime) {
    return 'Last seen $dateTime';
  }

  @override
  String get socialPresenceUnknown => 'Unknown';

  @override
  String socialActionError(String message) {
    return 'Error: $message';
  }

  @override
  String get socialRailTooltip => 'Friends and search';

  @override
  String get chatListTitle => 'Direct messages';

  @override
  String get chatListEmpty => 'No conversations yet';

  @override
  String get chatListEmptyHint =>
      'Find people in Search to start a direct message.';

  @override
  String get chatListLoadMore => 'Load more chats';

  @override
  String chatListUnreadCount(int count) {
    return '$count unread';
  }

  @override
  String get chatListLoadError => 'Could not load chats';

  @override
  String get chatListBackendUnavailable =>
      'Chats are unavailable. Start the full API stack (docker compose --profile app).';

  @override
  String chatListDmFallback(String id) {
    return 'Chat $id';
  }

  @override
  String get chatRoomSelectPrompt => 'Select a conversation';

  @override
  String get chatRoomBack => 'Back to chats';

  @override
  String chatRoomTitle(String id) {
    return 'Chat $id';
  }

  @override
  String get chatRoomEmpty => 'No messages yet';

  @override
  String get chatRoomEmptyHint => 'Send the first message when you are ready.';

  @override
  String get chatRoomLoadOlder => 'Load older messages';

  @override
  String get chatRoomInputHint => 'Message';

  @override
  String get chatSendMessage => 'Send message';

  @override
  String chatRoomError(String message) {
    return 'Error: $message';
  }

  @override
  String get chatRealtimeConnected => 'Live';

  @override
  String get chatRealtimeConnecting => 'Connecting…';

  @override
  String get chatRealtimeReconnecting => 'Reconnecting…';

  @override
  String get chatRealtimeOffline => 'Offline';

  @override
  String get chatOpenDm => 'Message';

  @override
  String get callStartAudio => 'Start audio call';

  @override
  String get callStartVideo => 'Start video call';

  @override
  String callIncomingTitle(String name) {
    return '$name is calling';
  }

  @override
  String get callIncomingAudio => 'Audio call';

  @override
  String get callIncomingVideo => 'Video call';

  @override
  String get callAccept => 'Accept';

  @override
  String get callDecline => 'Decline';

  @override
  String get callConnecting => 'Connecting call…';

  @override
  String get callActive => 'Call active';

  @override
  String get callTapToEnableAudio => 'Tap to enable incoming audio';

  @override
  String get callMute => 'Mute';

  @override
  String get callUnmute => 'Unmute';

  @override
  String get callSpeakerOff => 'Mute speakers';

  @override
  String get callSpeakerOn => 'Unmute speakers';

  @override
  String get callVideoOn => 'Turn camera on';

  @override
  String get callVideoOff => 'Turn camera off';

  @override
  String get callHangup => 'Hang up';

  @override
  String callOutgoingTitle(String name) {
    return 'Calling $name…';
  }

  @override
  String callFailed(String message) {
    return 'Could not start call: $message';
  }

  @override
  String get callLivekitConnectFailed => 'Could not connect to LiveKit';

  @override
  String get callActiveCallExists => 'You already have an active call';

  @override
  String get profileMessage => 'Message';

  @override
  String get profileEditTitle => 'Edit profile';

  @override
  String get profileEditTooltip => 'Edit profile';

  @override
  String get profileDisplayNameLabel => 'Display name';

  @override
  String get profileBioLabel => 'About';

  @override
  String get profileBioHelper => 'Up to 500 characters';

  @override
  String get profileAvatarChange => 'Change avatar';

  @override
  String profileAvatarSelected(String fileName) {
    return 'Selected: $fileName';
  }

  @override
  String get profileSave => 'Save';

  @override
  String get profileErrorDisplayNameRequired => 'Enter a display name.';

  @override
  String get profileErrorDisplayNameTooLong =>
      'Display name must be 32 characters or fewer.';

  @override
  String get profileErrorBioTooLong => 'About must be 500 characters or fewer.';

  @override
  String get profileErrorAvatarType => 'Use a static JPEG, PNG, or WebP image.';

  @override
  String get profileErrorAvatarTooLarge =>
      'Avatar must be a non-empty image up to 5 MB.';

  @override
  String profileEditSaveError(String message) {
    return 'Could not save profile: $message';
  }

  @override
  String get commonCancel => 'Cancel';

  @override
  String get commonSave => 'Save';

  @override
  String get commonRetry => 'Try again';

  @override
  String get commonLoading => '…';

  @override
  String get chatInboxDm => 'DMs';

  @override
  String get chatInboxRequests => 'Requests';

  @override
  String get chatTyping => 'Typing…';

  @override
  String get chatAttachFile => 'Attach file';

  @override
  String get chatMessageEdit => 'Edit';

  @override
  String get chatMessageDeleteForMe => 'Delete for me';

  @override
  String get chatMessageDeleteForEveryone => 'Delete for everyone';

  @override
  String get chatEditMessageTitle => 'Edit message';

  @override
  String get chatMessageEdited => '(edited)';

  @override
  String get chatDeliverySent => 'Sent';

  @override
  String get chatDeliveryDelivered => 'Delivered';

  @override
  String get chatDeliveryRead => 'Read';

  @override
  String get chatImageAttachment => 'Image attachment';

  @override
  String get chatNewMessages => 'New messages';

  @override
  String get chatUnreadSeparator => 'Unread messages';

  @override
  String get chatListStrangerBadge => 'Stranger';

  @override
  String get chatCreateGroupTooltip => 'Create group';

  @override
  String get chatCreateGroupTitle => 'New group';

  @override
  String get chatCreateGroupNameLabel => 'Group name';

  @override
  String get chatCreateGroupNameHint => 'Friday squad';

  @override
  String get chatCreateGroupMembers => 'Add members';

  @override
  String get chatCreateGroupMembersHint =>
      'Select at least 2 friends (3 people total including you).';

  @override
  String get chatCreateGroupSubmit => 'Create group';

  @override
  String get chatCreateGroupMinMembers =>
      'Select at least 2 friends to create a group.';

  @override
  String get chatCreateGroupFriendsEmptyHint =>
      'Add friends first, then invite them to a group.';

  @override
  String chatCreateGroupError(String message) {
    return 'Could not create group: $message';
  }

  @override
  String get settingsTitle => 'Settings';

  @override
  String get settingsTooltip => 'Settings';

  @override
  String get settingsTheme => 'Theme';

  @override
  String get settingsThemeSystem => 'System';

  @override
  String get settingsThemeLight => 'Light';

  @override
  String get settingsThemeDark => 'Dark';

  @override
  String get settingsThemeHighContrast => 'High contrast';

  @override
  String get settingsLanguage => 'Language';

  @override
  String get settingsLanguageSystem => 'System default';

  @override
  String get settingsLanguageEn => 'English';

  @override
  String get settingsLanguageRu => 'Russian';

  @override
  String get settingsAccent => 'Profile accent';

  @override
  String get authTagline => 'Voice chat and messages for gamers';

  @override
  String get versionUpdateRequired => 'Update required';

  @override
  String versionUpdateAvailable(String version) {
    return 'Update $version available';
  }

  @override
  String get versionUpdateAvailableGeneric => 'Update available';

  @override
  String get versionUpdateLater => 'Later';

  @override
  String get profileBlock => 'Block user';

  @override
  String get profileBlockConfirmTitle => 'Block this user?';

  @override
  String get profileBlockConfirmMessage =>
      'They will not be able to message you.';

  @override
  String get callVideoPlaceholder => 'Video preview';

  @override
  String get themeLoadError => 'Could not load theme';

  @override
  String get bootstrapRestoring => 'Restoring session…';
}
