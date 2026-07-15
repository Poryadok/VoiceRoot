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
  String get authErrorRegistrationConflict =>
      'This email can\'t be used. Try another one or sign in if you already have an account.';

  @override
  String get authLogin => 'Log in';

  @override
  String get authRegister => 'Register';

  @override
  String get authContinueGuest => 'Continue as guest';

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
  String get globalSearchHint => 'Search contacts, spaces, messages';

  @override
  String get globalSearchContacts => 'Contacts';

  @override
  String get globalSearchSpaces => 'Spaces';

  @override
  String get globalSearchMessages => 'Messages';

  @override
  String get globalSearchStartHint =>
      'Type to search across your chats and spaces.';

  @override
  String get globalSearchEmptyContacts => 'No matching contacts';

  @override
  String get globalSearchEmptySpaces => 'No matching spaces';

  @override
  String get globalSearchEmptyMessages => 'No matching messages';

  @override
  String get inChatSearchHint => 'Search in this chat';

  @override
  String get inChatSearchPrevious => 'Previous match';

  @override
  String get inChatSearchNext => 'Next match';

  @override
  String inChatSearchResultScore(String score) {
    return 'Score $score';
  }

  @override
  String get inChatSearchOpen => 'Search messages';

  @override
  String get socialAddFriend => 'Add friend';

  @override
  String get socialRemoveFriend => 'Remove from friends';

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
  String get socialProfileUnavailable => 'User unavailable';

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
  String get botTimeoutError =>
      'The bot did not respond in time. Try again later.';

  @override
  String get botDeferredProcessing => 'Processing your request…';

  @override
  String get slashCommandsTitle => 'Commands';

  @override
  String get slashCommandsLoadError => 'Could not load bot commands.';

  @override
  String get slashCommandsEmpty => 'No bot commands in this chat.';

  @override
  String get slashCommandsEmptyHint =>
      'Install bots in Space settings, or enable them for this chat in Chat info.';

  @override
  String get slashCommandsDmEmptyHint =>
      'Bot slash commands are available only in space text chats.';

  @override
  String get slashCommandsNoMatch => 'No matching commands';

  @override
  String get slashCommandsNoMatchHint =>
      'Keep typing after / or try another command or bot name.';

  @override
  String get slashCommandsHelp =>
      'Type / in the message box to open this menu. Greyed-out commands mean the bot is offline.';

  @override
  String get botUnavailableTooltip => 'Bot unavailable';

  @override
  String get botOnlineStatus => 'Bot online';

  @override
  String get botOfflineStatus => 'Bot offline';

  @override
  String get botInstallTitle => 'Install bot';

  @override
  String get botInstallDescriptionHeading => 'About';

  @override
  String get botInstallScopesHeading => 'Permissions';

  @override
  String get botInstallCommandsHeading => 'Commands';

  @override
  String get botInstallCommandsEmpty =>
      'Slash commands will appear here once registered.';

  @override
  String get botInstallWhitelistHeading => 'Install to space';

  @override
  String get botInstallSelectSpace => 'Choose a space';

  @override
  String get botInstallNoSpaces =>
      'Join or create a space to install this bot.';

  @override
  String get botInstallConfirm => 'Install bot';

  @override
  String get botScopeTextChatSendMessages =>
      'Send messages in allowed text chats';

  @override
  String get botScopeDmSend => 'Send direct messages (reply only)';

  @override
  String get botScopeSpaceViewMemberList => 'View space member list';

  @override
  String get botScopeMemberAssignRoles => 'Assign roles below the bot';

  @override
  String get botScopeTextChatCreateInSpace => 'Create text chats in the space';

  @override
  String get botScopeTextChatReadHistory => 'Read message history (privileged)';

  @override
  String get botScopeSpaceManageRoles =>
      'Create and manage roles below the bot (privileged)';

  @override
  String slashOptionPickUser(String name) {
    return 'User: $name';
  }

  @override
  String slashOptionPickChannel(String name) {
    return 'Channel: $name';
  }

  @override
  String slashOptionPickRole(String name) {
    return 'Role: $name';
  }

  @override
  String slashOptionPickAttachment(String name) {
    return 'Attachment: $name';
  }

  @override
  String slashOptionAttachmentSelected(String fileName) {
    return 'Selected: $fileName';
  }

  @override
  String get slashOptionPickerUnavailable =>
      'Picker unavailable in this chat context.';

  @override
  String get slashCommandRun => 'Run command';

  @override
  String get chatBotsSectionTitle => 'Bots';

  @override
  String get chatBotsLoadError => 'Could not load bots for this chat.';

  @override
  String get chatBotsEmpty => 'No bots installed in this space.';

  @override
  String get spaceBotsTitle => 'Space bots';

  @override
  String get spaceBotsInstall => 'Install bot';

  @override
  String get spaceBotsUninstall => 'Remove from space';

  @override
  String get spaceBotsInstallConfirm => 'Install';

  @override
  String get spaceBotsScopeWarning =>
      'This bot requests privileged access to read chat history.';

  @override
  String get spaceBotsPrivilegedAck =>
      'I understand this bot can read chat history';

  @override
  String get spaceBotsSelectChats => 'Allowed text chats';

  @override
  String get spaceBotsInstallSuccess => 'Bot installed.';

  @override
  String get spaceBotsUninstallSuccess => 'Bot removed from space.';

  @override
  String get ephemeralMessageLabel => 'Only visible to you';

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
  String get createProfileTitle => 'Add profile';

  @override
  String get createProfileSubmit => 'Create profile';

  @override
  String get createProfilePresetHint => 'Privacy preset';

  @override
  String get createProfileLimitReached =>
      'Profile limit reached. Premium allows up to 5 profiles.';

  @override
  String get createProfileOpenSubscription => 'View Premium';

  @override
  String get createProfileAddAction => 'Add profile';

  @override
  String profileSwitchVoiceBound(String profileName) {
    return 'Voice: $profileName';
  }

  @override
  String get voiceLeaveCurrentDialogTitle => 'Leave current voice?';

  @override
  String voiceLeaveCurrentDialogMessage(String profileName) {
    return 'You are in a voice chat ($profileName). Leave it and join here?';
  }

  @override
  String get voiceLeaveCurrentDialogConfirm => 'Leave and join';

  @override
  String get commonCancel => 'Cancel';

  @override
  String get shareLinkAction => 'Copy link';

  @override
  String get shareLinkCopied => 'Link copied to clipboard';

  @override
  String get commonEdit => 'Edit';

  @override
  String get commonDelete => 'Delete';

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
  String get chatMessageReply => 'Reply';

  @override
  String chatReplyingTo(String preview) {
    return 'Replying to $preview';
  }

  @override
  String get chatThreadTitle => 'Thread';

  @override
  String get chatThreadEmpty => 'No replies yet';

  @override
  String get chatThreadLoadError => 'Could not load thread';

  @override
  String get chatChannelMainFeedBlocked => 'Post in a thread or as the channel';

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
  String get spacePermissionDeniedManageRoles =>
      'You need permission to manage roles';

  @override
  String get spacePermissionDeniedSetSlowMode =>
      'You need permission to change slow mode';

  @override
  String get spacePermissionDeniedVoiceJoin =>
      'You need permission to join this voice room';

  @override
  String get spacePermissionDeniedSendMessages =>
      'You need permission to post in this channel';

  @override
  String spacePermissionDeniedGeneric(String permission) {
    return 'Missing permission: $permission';
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
  String get spaceRolesTooltip => 'Manage roles';

  @override
  String get spaceRolesTitle => 'Roles';

  @override
  String get spaceRolesLoadError => 'Could not load roles';

  @override
  String get spaceRoleCreateTitle => 'Create role';

  @override
  String get spaceRoleEditTitle => 'Edit role';

  @override
  String get spaceRoleNameLabel => 'Role name';

  @override
  String get spaceRoleManaged => 'System role';

  @override
  String get spaceRoleCustom => 'Custom role';

  @override
  String get spaceChatOverrideTitle => 'Chat access overrides';

  @override
  String get spaceChatOverrideHint =>
      'Deny view or send for a role in this chat only.';

  @override
  String get spaceChatOverrideDenyView => 'Deny view chat';

  @override
  String get spaceChatOverrideDenySend => 'Deny send messages';

  @override
  String get spaceVoiceOverrideTitle => 'Voice room access overrides';

  @override
  String get spaceVoiceOverrideHint =>
      'Deny join for a role in this voice room only.';

  @override
  String get spaceVoiceOverrideDenyJoin => 'Deny join voice';

  @override
  String get spaceSetDefaultJoinRole => 'Set as default join role';

  @override
  String spaceDefaultJoinRole(String name) {
    return 'Default join role: $name';
  }

  @override
  String get spaceRevokeRole => 'Revoke';

  @override
  String spaceRevokeRoleError(String message) {
    return 'Could not revoke role: $message';
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
  String get chatInfoTitle => 'Chat info';

  @override
  String get chatInfoOpen => 'Chat info';

  @override
  String get chatSharedMediaTabMedia => 'Media';

  @override
  String get chatSharedMediaTabFiles => 'Files';

  @override
  String get chatSharedMediaTabLinks => 'Links';

  @override
  String get chatSharedMediaTabVoice => 'Voice';

  @override
  String get chatSharedMediaEmpty => 'Nothing here yet';

  @override
  String get chatSharedMediaLoadError => 'Could not load shared media';

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
      'Transfer ownership to another member before leaving.';

  @override
  String chatGroupTransferOwnershipTo(String name) {
    return 'Transfer ownership to $name';
  }

  @override
  String get chatGroupTransferOwnershipTitle => 'Transfer group ownership';

  @override
  String chatGroupTransferOwnershipMessage(String name) {
    return 'Make $name the new group owner?';
  }

  @override
  String get chatGroupTransferOwnershipConfirm => 'Transfer';

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
  String get settingsReducedMotion => 'Reduced motion';

  @override
  String get settingsHelp => 'Help';

  @override
  String get settingsHelpTitle => 'Help';

  @override
  String get settingsHelpChatsTitle => 'Chats';

  @override
  String get settingsHelpChatsBody =>
      'Direct messages, groups, and channels appear in the chat list. Use folders to organize.';

  @override
  String get settingsHelpSpacesTitle => 'Spaces';

  @override
  String get settingsHelpSpacesBody =>
      'Join or create spaces for communities with text channels and voice rooms.';

  @override
  String get settingsHelpMatchmakingTitle => 'Matchmaking';

  @override
  String get settingsHelpMatchmakingBody =>
      'Find teammates by game and criteria from the matchmaking tab.';

  @override
  String get settingsHelpVoiceTitle => 'Voice';

  @override
  String get settingsHelpVoiceBody =>
      'Join voice rooms in spaces or start DM calls from a chat.';

  @override
  String get settingsHelpFooter =>
      'Need more? Contact support from your account settings.';

  @override
  String get settingsSubscription => 'Subscription';

  @override
  String get subscriptionSettingsTitle => 'Subscription';

  @override
  String get subscriptionCurrentPlan => 'Current plan';

  @override
  String get subscriptionStatusFree => 'Free';

  @override
  String get subscriptionStatusPremium => 'Premium';

  @override
  String subscriptionBillingPeriod(String period) {
    return 'Billing: $period';
  }

  @override
  String get subscriptionUpgradeTitle => 'Upgrade to Premium';

  @override
  String get subscriptionUpgradeMonthly => 'Premium — monthly';

  @override
  String get subscriptionUpgradeYearly => 'Premium — yearly (−20%)';

  @override
  String get subscriptionManageBilling => 'Manage subscription';

  @override
  String get subscriptionCancel => 'Cancel subscription';

  @override
  String get subscriptionStatusGracePeriod => 'Premium — payment issue';

  @override
  String subscriptionStatusPremiumUntil(String date) {
    return 'Premium until $date';
  }

  @override
  String get subscriptionGracePeriodHint =>
      'Update your payment method to keep Premium benefits.';

  @override
  String get subscriptionPremiumUntilHint =>
      'Premium features stay active until this date.';

  @override
  String get subscriptionLoadError => 'Could not load subscription';

  @override
  String get subscriptionRetry => 'Retry';

  @override
  String get subscriptionFreeTierNote =>
      'Messages and chats stay free on the Free plan.';

  @override
  String get subscriptionUpgradeSubtitle =>
      'Unlock cosmetics, larger uploads, and more profiles — without losing access to messages.';

  @override
  String get subscriptionBenefitBadge => 'Premium ★ badge in chats';

  @override
  String get subscriptionBenefitUploads => '200 MB file uploads';

  @override
  String get subscriptionBenefitProfiles => 'Up to 5 profiles';

  @override
  String get subscriptionBillingPeriodMonthly => 'Monthly';

  @override
  String get subscriptionBillingPeriodYearly => 'Yearly';

  @override
  String get subscriptionCheckoutLaunchFailed => 'Could not open checkout';

  @override
  String get subscriptionInvalidCheckoutUrl => 'Checkout link is invalid';

  @override
  String get subscriptionProfilesLoadError => 'Could not load profiles';

  @override
  String get downgradeProfilePickerTitle => 'Choose 2 profiles to keep';

  @override
  String get downgradeProfilePickerHint =>
      'Other profiles will be frozen until you renew Premium.';

  @override
  String get downgradeProfilePickerConfirm => 'Keep selected profiles';

  @override
  String get downgradeProfilePrimary => 'Primary profile';

  @override
  String get premiumBadgeLabel => 'Premium';

  @override
  String get settingsSecurity => 'Security & trust';

  @override
  String get securitySettingsTitle => 'Security';

  @override
  String get verificationSettingsTitle => 'Verification';

  @override
  String get verificationSettingsHint =>
      'Link platforms to earn a verified badge, or verify your organization domain.';

  @override
  String get verificationLinkedAccountsTitle => 'Linked accounts';

  @override
  String get verificationLinkedAccountsEmpty => 'No linked accounts yet.';

  @override
  String get verificationLinkTwitch => 'Link Twitch';

  @override
  String get verifiedBadgePersonal => 'Verified';

  @override
  String get verifiedBadgeOrganization => 'Verified organization';

  @override
  String get security2faEnableTitle => 'Two-factor authentication';

  @override
  String get security2faEnableHint =>
      'Confirm your password to start 2FA setup.';

  @override
  String get security2faContinue => 'Continue';

  @override
  String get security2faScanQr => 'Scan this QR code in your authenticator app';

  @override
  String get security2faBackupCodesTitle => 'Backup codes (save these now)';

  @override
  String get security2faVerifyTitle => 'Verify authenticator';

  @override
  String get security2faVerifyHint =>
      'Enter the 6-digit code or a backup code.';

  @override
  String get security2faVerify => 'Enable 2FA';

  @override
  String get security2faBackToQr => 'Back to QR';

  @override
  String get security2faEnabled => 'Two-factor authentication is enabled.';

  @override
  String get privacySettingsTitle => 'Privacy';

  @override
  String get notificationSettingsTitle => 'Notifications';

  @override
  String get notificationChatSettingsTitle => 'Chat notifications';

  @override
  String get notificationChatOverridesTitle => 'Notification overrides';

  @override
  String get notificationChatOverridesHint => 'Customize alerts for this chat';

  @override
  String get notificationLoadError => 'Could not load notification settings';

  @override
  String get notificationSettingsSaved => 'Notification settings saved';

  @override
  String get notificationSettingsSavedQuietHoursFailed =>
      'Settings saved, but quiet hours could not sync';

  @override
  String get notificationGlobalEnabled => 'Notifications enabled';

  @override
  String get notificationChatEnabled => 'Notifications for this chat';

  @override
  String get notificationEventTypesTitle => 'Event types';

  @override
  String get notificationTypeNewMessage => 'Direct messages';

  @override
  String get notificationTypeMention => 'Mentions';

  @override
  String get notificationTypeReply => 'Replies';

  @override
  String get notificationTypeReaction => 'Reactions';

  @override
  String get notificationTypeFriendRequest => 'Friend requests';

  @override
  String get notificationTypeMatchFound => 'Match found';

  @override
  String get notificationTypeSystem => 'System';

  @override
  String get notificationQuietHoursTitle => 'Quiet hours';

  @override
  String get notificationQuietHoursEnabled => 'Enable quiet hours';

  @override
  String get notificationQuietHoursStart => 'Start';

  @override
  String get notificationQuietHoursEnd => 'End';

  @override
  String get notificationQuietHoursOverrideMentions =>
      'Allow mentions during quiet hours';

  @override
  String get notificationQuietHoursOverrideMentionsHint =>
      'When enabled, @mentions still notify you';

  @override
  String get notificationPushSectionTitle => 'Device notifications';

  @override
  String get notificationPushEnableTitle => 'Enable push notifications';

  @override
  String get notificationPushExplainerTitle => 'Stay in the loop';

  @override
  String get notificationPushExplainerBody =>
      'Voice can send push notifications for messages, mentions, friend requests, and matchmaking — even when the app is in the background.';

  @override
  String get notificationPushExplainerContinue => 'Continue';

  @override
  String get notificationPushStatusGranted => 'Enabled on this device';

  @override
  String get notificationPushStatusDenied =>
      'Blocked — enable in system settings';

  @override
  String get notificationPushStatusNotDetermined => 'Not enabled yet';

  @override
  String get notificationPushStatusUnsupported =>
      'Not available on this device';

  @override
  String get notificationPushEnabled => 'Push notifications enabled';

  @override
  String get notificationPushDenied => 'Push permission was denied';

  @override
  String get notificationPushUnsupported =>
      'Push is not available in this build';

  @override
  String get privacyLoadError => 'Could not load privacy settings';

  @override
  String get privacySaved => 'Privacy settings saved';

  @override
  String get privacyPresetTitle => 'Preset';

  @override
  String get privacyPresetPersonal => 'Personal';

  @override
  String get privacyPresetGaming => 'Gaming';

  @override
  String get privacyPresetWork => 'Work';

  @override
  String get privacyAllowDm => 'Who can message you';

  @override
  String get privacyAllowGuestDm => 'Allow guest accounts in DMs';

  @override
  String get privacyVisibilityTitle => 'Visibility';

  @override
  String get privacyShowOnline => 'Online status';

  @override
  String get privacyShowGameStatus => 'In-game status';

  @override
  String get privacyShowMmRating => 'Matchmaking rating';

  @override
  String get privacyShowPhone => 'Phone number';

  @override
  String get privacyShowStories => 'Stories';

  @override
  String get privacyAllowFriendRequests => 'Friend requests';

  @override
  String get privacyAudienceEveryone => 'Everyone';

  @override
  String get privacyAudienceFriends => 'Friends';

  @override
  String get privacyAudienceFriendsOfFriends => 'Friends of friends';

  @override
  String get privacyAudienceNobody => 'Nobody';

  @override
  String get privacyAudienceSpaceMembers => 'Space members';

  @override
  String get privacyAudienceIncludeGuests => 'Guest accounts';

  @override
  String get privacyAudienceSpacesTitle => 'Spaces';

  @override
  String get privacyAudienceSpacesEmpty => 'You are not a member of any spaces';

  @override
  String get privacyActionsTitle => 'Actions';

  @override
  String get privacyAllowPhoneSearch => 'Phone number search';

  @override
  String get privacyAllowCalls => 'Calls';

  @override
  String get privacyAllowChatSpaceInvites => 'Chat and space invites';

  @override
  String get privacyAllowFiles => 'File sharing';

  @override
  String get privacyAllowVoiceMessages => 'Voice messages';

  @override
  String get privacyDeniedCall => 'This person does not accept calls from you';

  @override
  String get privacyDeniedInvite =>
      'This person does not accept chat or space invites';

  @override
  String get privacyDeniedFile =>
      'This person does not accept file attachments';

  @override
  String get privacyDeniedVoice => 'This person does not accept voice messages';

  @override
  String get privacyDeniedDm => 'This person does not accept messages from you';

  @override
  String get reportAction => 'Report';

  @override
  String get reportTitle => 'Report';

  @override
  String get reportSubtitle => 'Choose a category. We will review your report.';

  @override
  String get reportCategorySpam => 'Spam';

  @override
  String get reportCategoryHarassment => 'Harassment';

  @override
  String get reportCategoryOffensive => 'Offensive content';

  @override
  String get reportCategoryFake => 'Fake / impersonation';

  @override
  String get reportCategoryMmToxic => 'Cheating / MM toxic';

  @override
  String get reportCategoryOther => 'Other';

  @override
  String get reportCommentLabel => 'Comment';

  @override
  String get reportCommentRequired =>
      'Required for «Other» (up to 500 characters)';

  @override
  String get reportSubmit => 'Submit report';

  @override
  String get reportAcceptedTitle => 'Report accepted';

  @override
  String get reportAcceptedMessage =>
      'We will review it shortly. You will not receive status updates.';

  @override
  String get authTotpStepTitle => 'Two-factor code';

  @override
  String get authTotpLabel => 'Authenticator or backup code';

  @override
  String get authTotpHelper =>
      'Enter the code from your authenticator app or a backup code.';

  @override
  String get authErrorTotpRequired => 'Enter your two-factor code to continue.';

  @override
  String get authErrorInvalidTotp => 'Invalid authenticator or backup code.';

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
  String get screenShareStart => 'Share screen';

  @override
  String get screenShareStop => 'Stop sharing';

  @override
  String get screenSharePause => 'Pause share';

  @override
  String get screenShareResume => 'Resume share';

  @override
  String get screenShareQualityTitle => 'Share quality';

  @override
  String get screenShareQuality720p15 => '720p · 15 FPS';

  @override
  String get screenShareQuality720p30 => '720p · 30 FPS';

  @override
  String get screenShareLimitReached =>
      'This voice chat already has 3 screen shares';

  @override
  String get screenShareWaitingForVideo => 'Waiting for screen video…';

  @override
  String get platformWebSystemAudioUnavailable =>
      'System audio sharing is not available in the browser';

  @override
  String get platformWebGlobalPttUnavailable =>
      'Global push-to-talk hotkeys are not available outside this browser tab';

  @override
  String get themeLoadError => 'Could not load theme';

  @override
  String get bootstrapRestoring => 'Restoring session…';

  @override
  String get guestNicknameTitle => 'Choose a nickname';

  @override
  String get guestNicknameSubtitle =>
      'Guests need a display name before chatting.';

  @override
  String get guestNicknameLabel => 'Nickname';

  @override
  String get guestNicknameHint => 'How others will see you';

  @override
  String get guestNicknameContinue => 'Continue';

  @override
  String get guestConvertTitle => 'Create your account';

  @override
  String get guestConvertSubtitle =>
      'Add email and password to keep your chats and profile.';

  @override
  String get guestConvertSubmit => 'Create account';

  @override
  String get guestSaveAccountReminder =>
      'Register your account so you do not lose access.';

  @override
  String get guestSaveAccountReminderCta => 'Register';

  @override
  String get privacyShowOnlineIncludeGuests =>
      'Guest accounts can see my online status';

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
  String get queueSearchRecoveryReturnToQueue => 'Return to queue';

  @override
  String get queueSearchDeclinedTitle => 'Match declined';

  @override
  String get queueSearchDeclinedBody =>
      'The match was declined. Search continues with your current parameters.';

  @override
  String get queueSearchRecoveryContinueSearch => 'Continue searching';

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

  @override
  String get e2eEnableTitle => 'Enable end-to-end encryption';

  @override
  String get e2eChatInfoSwitchLabel => 'End-to-end encryption';

  @override
  String get e2eChatInfoKeyBackup => 'Key backup';

  @override
  String get e2eEncryptFailed =>
      'Could not encrypt this message. Open the app on both devices and try again.';

  @override
  String get e2ePeerMissingPreKeys =>
      'Your contact has not set up encryption keys yet.';

  @override
  String get e2eEnableBody =>
      'End-to-end encryption is enabled for this chat.\n\nMessages are encrypted and unavailable to the server.\n— Global search will not find message bodies from this chat.\n— Local search works only on history loaded on this device.\n— Attachments are encrypted and automatically deleted after 90 days.';

  @override
  String get e2eEnableConfirm => 'Enable E2E';

  @override
  String get e2eEnableCancel => 'Cancel';

  @override
  String get e2eDisableTitle => 'Disable end-to-end encryption?';

  @override
  String get e2eDisableBody =>
      'If you disable E2E, new messages are stored in plain text on the server and server search will work again for this chat.';

  @override
  String get e2eDisableConfirm => 'Disable E2E';

  @override
  String get e2eDisableCancel => 'Cancel';

  @override
  String get e2eKeyBackupTitle => 'E2E key backup';

  @override
  String get e2eKeyBackupHint =>
      'Set a backup password so you can restore E2E keys on a new device.';

  @override
  String get e2eKeyBackupPasswordLabel => 'Backup password';

  @override
  String get e2eKeyBackupPasswordHintLabel => 'Password hint (optional)';

  @override
  String get e2eKeyBackupSave => 'Save backup';

  @override
  String get e2eKeyBackupRestore => 'Restore from backup';

  @override
  String get e2eUndecryptableGeneric =>
      'This message is encrypted and cannot be decrypted on this device. Set up key backup to avoid losing history.';

  @override
  String e2eUndecryptableBefore(String date) {
    return 'Messages before $date are encrypted and unavailable on this device. Set up key backup to avoid losing history.';
  }

  @override
  String get e2eInChatSearchLocalOnly =>
      'Encrypted chat: searching loaded history on this device only';

  @override
  String get e2eChatSettingsEnable => 'Enable E2E encryption';

  @override
  String get e2eChatSettingsDisable => 'Disable E2E encryption';

  @override
  String get e2eChatSettingsKeyBackup => 'Key backup';

  @override
  String get e2eEncryptionCodeTitle => 'Encryption code';

  @override
  String get e2eEncryptionCodeBody =>
      'Compare with your contact in voice or in person. Codes must match. If they do not match, your chat may not be protected from eavesdropping.';

  @override
  String e2eIdentityKeyChangedTitle(String nick) {
    return '$nick\'s encryption key changed';
  }

  @override
  String get e2eIdentityKeyChangedBody =>
      'This usually happens after reinstalling the app or switching devices without a key backup. Continue only if you expected this. If unsure, compare the encryption code in chat settings.';

  @override
  String get e2eIdentityKeyChangedContinue => 'Continue';

  @override
  String get e2eIdentityKeyChangedDistrust => 'Don\'t trust';

  @override
  String get e2eFileRetentionNotice =>
      'Encrypted attachments in this chat are automatically deleted after 90 days.';

  @override
  String get e2eAttachmentTapToDownload => 'Tap to download encrypted file';

  @override
  String get e2eAttachmentDownloadFailed => 'Could not save attachment';

  @override
  String get e2eAttachmentDecryptFailed => 'Could not decrypt attachment';

  @override
  String get storyRingActiveLabel => 'Active story';

  @override
  String get storyCreateTitle => 'New story';

  @override
  String get storyCreateTypeText => 'Text';

  @override
  String get storyCreateTypePhoto => 'Photo';

  @override
  String get storyCreateTypeVideo => 'Video';

  @override
  String get storyCreateTextLabel => 'Story text';

  @override
  String get storyCreateCaptionLabel => 'Caption';

  @override
  String get storyCreatePickMedia => 'Choose media';

  @override
  String get storyCreateSubmit => 'Publish story';

  @override
  String get storyCreateTextRequired => 'Enter story text';

  @override
  String get storyCreateMediaRequired => 'Choose a photo or video first';

  @override
  String get storyViewerEmpty => 'No stories to show';

  @override
  String get storyViewerLoadError => 'Could not load story';

  @override
  String get storyViewerNoMedia => 'Media unavailable';

  @override
  String get storyViewerVideoPlaceholder =>
      'Video playback is not available in this build';

  @override
  String get storyReactTooltip => 'React';

  @override
  String get storyReactSent => 'Reaction sent';

  @override
  String get storyHighlightsTitle => 'Highlights';

  @override
  String get storyLfpTitle => 'Looking for party';

  @override
  String get storyLfpGame => 'Game';

  @override
  String get storyCreateVisibilityLabel => 'Who can see this story';

  @override
  String get storyCreateMentionLabel => 'Mention friends';

  @override
  String get storyCreateMentionHint => '@username';

  @override
  String get storyCreateGameTagLabel => 'Game tag';

  @override
  String get storyCreateGameTagPick => 'Choose game';

  @override
  String get storyCreateGameTagClear => 'Clear';

  @override
  String get storyCreateTextStyleLabel => 'Background';

  @override
  String get storyVisibilityEveryone => 'Everyone';

  @override
  String get storyVisibilityFriends => 'Friends';

  @override
  String get storyVisibilityCloseFriends => 'Close friends';

  @override
  String get storyViewerReply => 'Reply';

  @override
  String get storyViewerReplyHint => 'Private reply';

  @override
  String get storyViewerReplySent => 'Reply sent';

  @override
  String storyViewerViewCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count views',
      one: '1 view',
    );
    return '$_temp0';
  }

  @override
  String storyHighlightVisibility(String value) {
    return 'Visibility: $value';
  }

  @override
  String get storyLfpJoin => 'Join';

  @override
  String get storyLfpWrite => 'Write';

  @override
  String get socialStoryCreate => 'New story';

  @override
  String get storyArchiveTitle => 'Story archive';

  @override
  String get storyArchiveEmpty => 'No archived stories';

  @override
  String get storyArchiveLoadError => 'Could not load story archive';

  @override
  String get storyArchiveAddToHighlight => 'Add to highlight';

  @override
  String get storyHighlightsManageTitle => 'Manage highlights';

  @override
  String get storyHighlightsEmpty => 'No highlights yet';

  @override
  String get storyHighlightCreate => 'New highlight';

  @override
  String get storyHighlightEditTitle => 'Edit highlight';

  @override
  String get storyHighlightDelete => 'Delete';

  @override
  String get storyHighlightDeleteConfirm => 'Delete this highlight?';

  @override
  String get storyHighlightNameHint => 'Highlight name';

  @override
  String get storyHighlightSave => 'Save';

  @override
  String get storyHighlightSaved => 'Highlight saved';

  @override
  String get storyHighlightSelectHighlight => 'Choose a highlight';

  @override
  String get storyHighlightAddStories => 'Add stories';

  @override
  String get storyHighlightStoriesSection => 'Stories';

  @override
  String storyHighlightStoryCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count stories',
      one: '1 story',
    );
    return '$_temp0';
  }

  @override
  String get storyViewersTitle => 'Viewers';

  @override
  String get storyViewersEmpty => 'No viewers yet';

  @override
  String get storyViewersLoadError => 'Could not load viewers';

  @override
  String get storyCreateVideoTooLong => 'Video must be 60 seconds or shorter';

  @override
  String get storyGameTagTapHint => 'Open game page';

  @override
  String get onboardingSkip => 'Skip';

  @override
  String get onboardingGotIt => 'Got it';

  @override
  String get onboardingLater => 'Later';

  @override
  String get onboardingSaveAccountTitle => 'Save your account';

  @override
  String get onboardingSaveAccountBody =>
      'Set a nickname and add email — save your account so you do not lose access.';

  @override
  String get onboardingChatsNavTitle => 'Chats and navigation';

  @override
  String get onboardingChatsNavBody =>
      'All your chats live here — DMs, groups, channels, and spaces, each in its own folder.';

  @override
  String get onboardingSpacesTitle => 'Spaces';

  @override
  String get onboardingSpacesBody =>
      'Spaces are communities with channels and voice rooms. Find one for your game or create your own.';

  @override
  String get onboardingSpacesFind => 'Find a space';

  @override
  String get onboardingMatchmakingTitle => 'Matchmaking';

  @override
  String get onboardingMatchmakingBody =>
      'Looking for a squad? We match you with people who fit your criteria.';

  @override
  String get onboardingMatchmakingTry => 'Try it';

  @override
  String get onboardingWrapUpTitle => 'You are all set';

  @override
  String get onboardingWrapUpBody =>
      'You know the basics! If something is unclear — Help is always available in Settings.';

  @override
  String get onboardingWrapUpStart => 'Start';
}
