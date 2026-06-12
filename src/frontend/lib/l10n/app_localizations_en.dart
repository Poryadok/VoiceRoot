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
  String get chatMentionInsert => 'Insert mention';

  @override
  String get chatMentionEveryone => '@everyone';

  @override
  String get chatMentionHere => '@here';

  @override
  String chatMentionMember(String profileId) {
    return 'Member $profileId';
  }

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
  String get chatOfflineReadOnly => 'You\'re offline. Showing saved messages.';

  @override
  String get chatOfflineSendBlocked => 'Can\'t send messages while offline.';

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
  String get callGroupVoiceStart => 'Start group voice';

  @override
  String get callGroupVoiceJoin => 'Join voice';

  @override
  String get callGroupVoiceActive => 'Group voice active';

  @override
  String get callGroupVoiceInProgress => 'Voice call in progress in this group';

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
  String get chatMessageForward => 'Forward';

  @override
  String get chatMessageAddReaction => 'Add reaction';

  @override
  String get chatMessagePin => 'Pin message';

  @override
  String get chatMessageUnpin => 'Unpin message';

  @override
  String chatPinnedBar(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count pinned messages',
      one: '1 pinned message',
    );
    return '$_temp0';
  }

  @override
  String get chatForwardTitle => 'Forward to';

  @override
  String get chatForwardSearchHint => 'Search chats';

  @override
  String chatForwardFrom(String sender) {
    return 'Forwarded from $sender';
  }

  @override
  String get chatForwardCommentaryTitle => 'Add a comment';

  @override
  String get chatForwardCommentaryHint => 'Optional message before the forward';

  @override
  String get chatForwardEmpty => 'No chats to forward to';

  @override
  String get chatForwardSuccess => 'Message forwarded';

  @override
  String chatForwardError(String message) {
    return 'Could not forward message: $message';
  }

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
  String get spaceCreateTooltip => 'Create space';

  @override
  String get spaceCreateTitle => 'New space';

  @override
  String get spaceCreateNameLabel => 'Space name';

  @override
  String get spaceCreateNameHint => 'Friday squad';

  @override
  String get spaceCreateDescriptionLabel => 'Description';

  @override
  String get spaceCreateDescriptionHint => 'What is this space about?';

  @override
  String get spaceCreateIconLabel => 'Icon URL';

  @override
  String get spaceCreateIconHint => 'https://cdn.example/icon.webp';

  @override
  String get spaceCreateSubmit => 'Create space';

  @override
  String spaceCreateError(String message) {
    return 'Could not create space: $message';
  }

  @override
  String get spaceTreeTitle => 'Channels';

  @override
  String get spaceTreeEmpty => 'No channels yet';

  @override
  String get spaceTreeLoadError => 'Could not load space tree';

  @override
  String get spaceTreeTextChat => 'Text chat';

  @override
  String get spaceTreeVoiceRoom => 'Voice room';

  @override
  String get spaceTreeUncategorized => 'Channels';

  @override
  String get spaceSelectPrompt => 'Select a space';

  @override
  String get spaceListTitle => 'My spaces';

  @override
  String get spaceOpenAction => 'Open space';

  @override
  String get spaceInvitesTooltip => 'Invite people';

  @override
  String get spaceInvitesTitle => 'Invite links';

  @override
  String get spaceInvitesSubtitle =>
      'Create a link to invite people to this space.';

  @override
  String get spaceInvitesEmpty => 'No active invite links';

  @override
  String get spaceInvitesLoadError => 'Could not load invites';

  @override
  String get spaceInvitesRetry => 'Retry';

  @override
  String get spaceInviteCreate => 'Create link';

  @override
  String get spaceInviteAdvancedToggle => 'Advanced';

  @override
  String get spaceInviteMaxUsesLabel => 'Max uses';

  @override
  String get spaceInviteMaxUsesHint => 'Unlimited if empty';

  @override
  String get spaceInviteMaxUsesInvalid => 'Max uses must be a positive number';

  @override
  String spaceInviteCreateError(String message) {
    return 'Could not create invite: $message';
  }

  @override
  String spaceInviteRevokeError(String message) {
    return 'Could not revoke invite: $message';
  }

  @override
  String get spaceInviteCopy => 'Copy link';

  @override
  String get spaceInviteCopied => 'Invite link copied';

  @override
  String get spaceInviteRevoke => 'Revoke';

  @override
  String spaceInviteUses(int used, String maxSuffix) {
    return '$used uses$maxSuffix';
  }

  @override
  String get spaceInviteJoinTooltip => 'Join space by invite';

  @override
  String get spaceInviteJoinTitle => 'Join a space';

  @override
  String get spaceInviteJoinSubtitle =>
      'Paste an invite code or link from a friend.';

  @override
  String get spaceInviteJoinCodeLabel => 'Invite code';

  @override
  String get spaceInviteJoinCodeHint => 'abc123xyz';

  @override
  String get spaceInviteJoinSubmit => 'Join space';

  @override
  String spaceInviteJoinError(String message) {
    return 'Could not join space: $message';
  }

  @override
  String get spaceMembersTooltip => 'Space members';

  @override
  String get spaceMembersTitle => 'Members';

  @override
  String get spaceMembersSubtitle =>
      'Owners and admins can assign roles and remove members.';

  @override
  String get spaceMembersLoadError => 'Could not load members';

  @override
  String spaceMemberYou(String name) {
    return '$name (you)';
  }

  @override
  String get spaceKick => 'Remove';

  @override
  String get spaceKickConfirmTitle => 'Remove member?';

  @override
  String spaceKickConfirmMessage(String name) {
    return 'Remove $name from this space?';
  }

  @override
  String spaceKickError(String message) {
    return 'Could not remove member: $message';
  }

  @override
  String get spaceBan => 'Ban';

  @override
  String get spaceBanConfirmTitle => 'Ban member?';

  @override
  String spaceBanConfirmMessage(String name) {
    return 'Ban $name from this space? They will not be able to rejoin.';
  }

  @override
  String spaceBanError(String message) {
    return 'Could not ban member: $message';
  }

  @override
  String get spaceTimeout => 'Timeout';

  @override
  String get spaceTimeoutConfirmTitle => 'Timeout member?';

  @override
  String spaceTimeoutConfirmMessage(String name) {
    return 'Prevent $name from sending messages for 10 minutes?';
  }

  @override
  String spaceTimeoutError(String message) {
    return 'Could not timeout member: $message';
  }

  @override
  String get spaceSlowMode => 'Slow mode';

  @override
  String get spaceSlowModeSubtitle =>
      'Minimum delay between messages in this channel';

  @override
  String get spaceSlowModeOff => 'Off';

  @override
  String spaceSlowModeSeconds(int seconds) {
    return '${seconds}s';
  }

  @override
  String get spaceAssignRole => 'Assign role';

  @override
  String get spaceAssignRoleTitle => 'Assign role';

  @override
  String get spaceAssignRoleEmpty => 'No assignable roles';

  @override
  String spaceAssignRoleError(String message) {
    return 'Could not assign role: $message';
  }

  @override
  String get chatGroupMembersTooltip => 'Group members';

  @override
  String get chatGroupMembersTitle => 'Members';

  @override
  String get chatGroupMembersSubtitle =>
      'Owner can remove members. Members can leave the group.';

  @override
  String get chatGroupMembersLoadError => 'Could not load members';

  @override
  String get chatGroupRoleOwner => 'Owner';

  @override
  String chatGroupMemberYou(String name) {
    return '$name (you)';
  }

  @override
  String get chatGroupKick => 'Remove';

  @override
  String get chatGroupKickConfirmTitle => 'Remove member?';

  @override
  String chatGroupKickConfirmMessage(String name) {
    return 'Remove $name from the group?';
  }

  @override
  String get chatGroupLeave => 'Leave group';

  @override
  String get chatGroupLeaveConfirmTitle => 'Leave group?';

  @override
  String get chatGroupLeaveConfirmMessage =>
      'You will no longer receive messages from this group.';

  @override
  String get chatGroupOwnerLeaveHint =>
      'As the owner, transfer ownership before leaving (coming soon).';

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
  String get versionUpdateNow => 'Update';

  @override
  String get versionRestartToUpdate => 'Restart and update';

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

  @override
  String get gameCatalogTitle => 'Game catalog';

  @override
  String get gameCatalogEntry => 'Browse games';

  @override
  String get gameCatalogSearchHint => 'Search games';

  @override
  String get gameCatalogLoadError => 'Could not load game catalog';

  @override
  String get gameCatalogEmpty => 'No games found';

  @override
  String get gameCatalogInGameRoles => 'In-game roles';

  @override
  String get gameCatalogRankLadder => 'Rank ladder';

  @override
  String gameCatalogRegions(String regions) {
    return 'Regions: $regions';
  }

  @override
  String gameCatalogModeSlots(int slots, int min, int max) {
    return '$slots players · party $min–$max';
  }

  @override
  String get playerProfileTitle => 'Matchmaking profile';

  @override
  String get playerProfileEntry => 'Matchmaking profile';

  @override
  String get playerProfileLoadError => 'Could not load matchmaking profile';

  @override
  String get playerProfileEmpty => 'No games configured yet';

  @override
  String get playerProfileAddGame => 'Add game';

  @override
  String get playerProfileSave => 'Save game profile';

  @override
  String get playerProfileSection => 'Matchmaking';

  @override
  String get playerProfileForGame => 'My profile for this game';

  @override
  String get queueSearchStart => 'Start queue';

  @override
  String get queueSearchTitle => 'Find teammates';

  @override
  String get queueSearchSearching => 'Searching for teammates…';

  @override
  String get queueSearchCancel => 'Cancel search';

  @override
  String get queueSearchRegion => 'Region';

  @override
  String get queueSearchRole => 'Your role';

  @override
  String get queueSearchRank => 'Your rank';

  @override
  String get queueSearchSoughtRankMin => 'Min rank sought';

  @override
  String get queueSearchSoughtRankMax => 'Max rank sought';

  @override
  String get queueSearchStartError => 'Could not start search';

  @override
  String get queueSearchCancelError => 'Could not cancel search';

  @override
  String get queueSearchNudgeTitle => 'Still searching';

  @override
  String get queueSearchNudgeBody =>
      'Taking a while. Try adjusting your search parameters.';

  @override
  String get queueSearchTimeoutTitle => 'No match found';

  @override
  String get queueSearchTimeoutBody =>
      'We couldn\'t find teammates this time. Please try again.';

  @override
  String get matchFoundTitle => 'Match found';

  @override
  String matchFoundSubtitle(String gameName, String mode) {
    return '$gameName · $mode';
  }

  @override
  String get matchFoundAccept => 'Accept';

  @override
  String get matchFoundDecline => 'Decline';

  @override
  String get matchFoundRespondError => 'Could not respond to match';

  @override
  String get matchSquadLeave => 'Leave squad';

  @override
  String get matchSquadLeaveError => 'Could not leave match squad';

  @override
  String get matchRatingTitle => 'Rate your teammates';

  @override
  String get matchRatingSubtitle => 'Stars are optional for each player.';

  @override
  String get matchRatingSkipTeammate => 'Skip';

  @override
  String get matchRatingSkipAll => 'Skip all';

  @override
  String get matchRatingSkipped => 'Skipped';

  @override
  String get matchRatingSubmit => 'Submit ratings';

  @override
  String get matchRatingBanTitle => 'Ban from matchmaking?';

  @override
  String matchRatingBanMessage(String name) {
    return 'Stop matching with $name in matchmaking?';
  }

  @override
  String get matchRatingBanCancel => 'Cancel';

  @override
  String get matchRatingBanConfirm => 'Ban';

  @override
  String get matchRatingBanAction => 'Ban from MM';

  @override
  String get matchRatingSubmitError => 'Could not submit rating';

  @override
  String get matchRatingBanError => 'Could not ban from matchmaking';

  @override
  String profileMmRating(String rating) {
    return 'MM rating: $rating ★';
  }

  @override
  String get matchHistoryTitle => 'Match history';

  @override
  String get matchHistoryEntry => 'Match history';

  @override
  String get matchHistoryLoadError => 'Could not load match history';

  @override
  String get matchHistoryEmpty => 'No match squads yet';

  @override
  String get matchHistoryParticipants => 'Teammates';

  @override
  String get matchHistoryStatusActive => 'Active';

  @override
  String get matchHistoryStatusCompleted => 'Completed';

  @override
  String get matchHistoryLoadMore => 'Load more';

  @override
  String get matchHistoryAddFriend => 'Add friend';
}
