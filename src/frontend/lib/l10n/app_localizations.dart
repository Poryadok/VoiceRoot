import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_en.dart';
import 'app_localizations_ru.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'l10n/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations? of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations);
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('en'),
    Locale('ru'),
  ];

  /// No description provided for @appTitle.
  ///
  /// In en, this message translates to:
  /// **'Voice'**
  String get appTitle;

  /// No description provided for @gatewayStatusOk.
  ///
  /// In en, this message translates to:
  /// **'Gateway: ok'**
  String get gatewayStatusOk;

  /// No description provided for @gatewayStatusChecking.
  ///
  /// In en, this message translates to:
  /// **'Gateway: checking…'**
  String get gatewayStatusChecking;

  /// No description provided for @gatewayMissingBaseUrl.
  ///
  /// In en, this message translates to:
  /// **'Gateway: missing base URL'**
  String get gatewayMissingBaseUrl;

  /// No description provided for @gatewayStatusError.
  ///
  /// In en, this message translates to:
  /// **'Gateway: error ({error})'**
  String gatewayStatusError(String error);

  /// No description provided for @gatewayStatusFailure.
  ///
  /// In en, this message translates to:
  /// **'Gateway: {detail}'**
  String gatewayStatusFailure(String detail);

  /// No description provided for @authTitle.
  ///
  /// In en, this message translates to:
  /// **'Sign in to Voice'**
  String get authTitle;

  /// No description provided for @authEmailLabel.
  ///
  /// In en, this message translates to:
  /// **'Email'**
  String get authEmailLabel;

  /// No description provided for @authPasswordLabel.
  ///
  /// In en, this message translates to:
  /// **'Password'**
  String get authPasswordLabel;

  /// No description provided for @authPasswordHelper.
  ///
  /// In en, this message translates to:
  /// **'At least 8 characters'**
  String get authPasswordHelper;

  /// No description provided for @authErrorEmptyFields.
  ///
  /// In en, this message translates to:
  /// **'Enter your email and password.'**
  String get authErrorEmptyFields;

  /// No description provided for @authErrorPasswordTooShort.
  ///
  /// In en, this message translates to:
  /// **'Password must be at least 8 characters.'**
  String get authErrorPasswordTooShort;

  /// No description provided for @authErrorValidationFailed.
  ///
  /// In en, this message translates to:
  /// **'Use a valid email and a password of at least 8 characters.'**
  String get authErrorValidationFailed;

  /// No description provided for @authErrorRateLimited.
  ///
  /// In en, this message translates to:
  /// **'Too many attempts. Please wait and try again.'**
  String get authErrorRateLimited;

  /// No description provided for @authErrorInvalidCredentials.
  ///
  /// In en, this message translates to:
  /// **'Incorrect email or password.'**
  String get authErrorInvalidCredentials;

  /// No description provided for @authLogin.
  ///
  /// In en, this message translates to:
  /// **'Log in'**
  String get authLogin;

  /// No description provided for @authRegister.
  ///
  /// In en, this message translates to:
  /// **'Register'**
  String get authRegister;

  /// No description provided for @authLogout.
  ///
  /// In en, this message translates to:
  /// **'Log out'**
  String get authLogout;

  /// No description provided for @authError.
  ///
  /// In en, this message translates to:
  /// **'Auth error: {message}'**
  String authError(String message);

  /// No description provided for @authSessionProfile.
  ///
  /// In en, this message translates to:
  /// **'Profile: {profileId}'**
  String authSessionProfile(String profileId);

  /// No description provided for @authSessionHandle.
  ///
  /// In en, this message translates to:
  /// **'{handle}'**
  String authSessionHandle(String handle);

  /// No description provided for @socialDiscoverHint.
  ///
  /// In en, this message translates to:
  /// **'Find people — use the icon on the left'**
  String get socialDiscoverHint;

  /// No description provided for @backendUnavailable.
  ///
  /// In en, this message translates to:
  /// **'Social and chat features are unavailable. Start the full API stack (docker compose --profile app).'**
  String get backendUnavailable;

  /// No description provided for @socialTabSearch.
  ///
  /// In en, this message translates to:
  /// **'Search'**
  String get socialTabSearch;

  /// No description provided for @socialTabFriends.
  ///
  /// In en, this message translates to:
  /// **'Friends'**
  String get socialTabFriends;

  /// No description provided for @socialTabRequests.
  ///
  /// In en, this message translates to:
  /// **'Requests'**
  String get socialTabRequests;

  /// No description provided for @socialSearchHint.
  ///
  /// In en, this message translates to:
  /// **'Search by name or @username'**
  String get socialSearchHint;

  /// No description provided for @socialSearchStart.
  ///
  /// In en, this message translates to:
  /// **'Search for people'**
  String get socialSearchStart;

  /// No description provided for @socialSearchStartHint.
  ///
  /// In en, this message translates to:
  /// **'Enter a name or @username to start a conversation.'**
  String get socialSearchStartHint;

  /// No description provided for @socialSearchEmpty.
  ///
  /// In en, this message translates to:
  /// **'No profiles found'**
  String get socialSearchEmpty;

  /// No description provided for @socialSearchEmptyHint.
  ///
  /// In en, this message translates to:
  /// **'Check the spelling or try another handle.'**
  String get socialSearchEmptyHint;

  /// No description provided for @socialSearchLoading.
  ///
  /// In en, this message translates to:
  /// **'Searching…'**
  String get socialSearchLoading;

  /// No description provided for @globalSearchHint.
  ///
  /// In en, this message translates to:
  /// **'Search contacts, spaces, messages'**
  String get globalSearchHint;

  /// No description provided for @globalSearchContacts.
  ///
  /// In en, this message translates to:
  /// **'Contacts'**
  String get globalSearchContacts;

  /// No description provided for @globalSearchSpaces.
  ///
  /// In en, this message translates to:
  /// **'Spaces'**
  String get globalSearchSpaces;

  /// No description provided for @globalSearchMessages.
  ///
  /// In en, this message translates to:
  /// **'Messages'**
  String get globalSearchMessages;

  /// No description provided for @globalSearchStartHint.
  ///
  /// In en, this message translates to:
  /// **'Type to search across your chats and spaces.'**
  String get globalSearchStartHint;

  /// No description provided for @globalSearchEmptyContacts.
  ///
  /// In en, this message translates to:
  /// **'No matching contacts'**
  String get globalSearchEmptyContacts;

  /// No description provided for @globalSearchEmptySpaces.
  ///
  /// In en, this message translates to:
  /// **'No matching spaces'**
  String get globalSearchEmptySpaces;

  /// No description provided for @globalSearchEmptyMessages.
  ///
  /// In en, this message translates to:
  /// **'No matching messages'**
  String get globalSearchEmptyMessages;

  /// No description provided for @inChatSearchHint.
  ///
  /// In en, this message translates to:
  /// **'Search in this chat'**
  String get inChatSearchHint;

  /// No description provided for @inChatSearchPrevious.
  ///
  /// In en, this message translates to:
  /// **'Previous match'**
  String get inChatSearchPrevious;

  /// No description provided for @inChatSearchNext.
  ///
  /// In en, this message translates to:
  /// **'Next match'**
  String get inChatSearchNext;

  /// No description provided for @inChatSearchResultScore.
  ///
  /// In en, this message translates to:
  /// **'Score {score}'**
  String inChatSearchResultScore(String score);

  /// No description provided for @inChatSearchOpen.
  ///
  /// In en, this message translates to:
  /// **'Search messages'**
  String get inChatSearchOpen;

  /// No description provided for @socialAddFriend.
  ///
  /// In en, this message translates to:
  /// **'Add friend'**
  String get socialAddFriend;

  /// No description provided for @socialAcceptRequest.
  ///
  /// In en, this message translates to:
  /// **'Accept'**
  String get socialAcceptRequest;

  /// No description provided for @socialDeclineRequest.
  ///
  /// In en, this message translates to:
  /// **'Decline'**
  String get socialDeclineRequest;

  /// No description provided for @socialRequestPending.
  ///
  /// In en, this message translates to:
  /// **'Request pending'**
  String get socialRequestPending;

  /// No description provided for @socialFriendsEmpty.
  ///
  /// In en, this message translates to:
  /// **'No friends yet'**
  String get socialFriendsEmpty;

  /// No description provided for @socialRequestsEmpty.
  ///
  /// In en, this message translates to:
  /// **'No friend requests'**
  String get socialRequestsEmpty;

  /// No description provided for @socialIncomingRequests.
  ///
  /// In en, this message translates to:
  /// **'Incoming'**
  String get socialIncomingRequests;

  /// No description provided for @socialOutgoingRequests.
  ///
  /// In en, this message translates to:
  /// **'Outgoing'**
  String get socialOutgoingRequests;

  /// No description provided for @socialFriendsLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load friends'**
  String get socialFriendsLoadError;

  /// No description provided for @socialFriendsBackendUnavailable.
  ///
  /// In en, this message translates to:
  /// **'Friends are unavailable. Start the full API stack (docker compose --profile app).'**
  String get socialFriendsBackendUnavailable;

  /// No description provided for @socialRequestsLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load requests'**
  String get socialRequestsLoadError;

  /// No description provided for @socialProfileLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load profile'**
  String get socialProfileLoadError;

  /// No description provided for @socialPresenceOnline.
  ///
  /// In en, this message translates to:
  /// **'Online'**
  String get socialPresenceOnline;

  /// No description provided for @socialPresenceIdle.
  ///
  /// In en, this message translates to:
  /// **'Idle'**
  String get socialPresenceIdle;

  /// No description provided for @socialPresenceDnd.
  ///
  /// In en, this message translates to:
  /// **'Do not disturb'**
  String get socialPresenceDnd;

  /// No description provided for @socialPresenceOffline.
  ///
  /// In en, this message translates to:
  /// **'Offline'**
  String get socialPresenceOffline;

  /// No description provided for @socialPresenceLastSeen.
  ///
  /// In en, this message translates to:
  /// **'Last seen {dateTime}'**
  String socialPresenceLastSeen(String dateTime);

  /// No description provided for @socialPresenceUnknown.
  ///
  /// In en, this message translates to:
  /// **'Unknown'**
  String get socialPresenceUnknown;

  /// No description provided for @socialActionError.
  ///
  /// In en, this message translates to:
  /// **'Error: {message}'**
  String socialActionError(String message);

  /// No description provided for @socialRailTooltip.
  ///
  /// In en, this message translates to:
  /// **'Friends and search'**
  String get socialRailTooltip;

  /// No description provided for @chatListTitle.
  ///
  /// In en, this message translates to:
  /// **'Direct messages'**
  String get chatListTitle;

  /// No description provided for @chatListEmpty.
  ///
  /// In en, this message translates to:
  /// **'No conversations yet'**
  String get chatListEmpty;

  /// No description provided for @chatListEmptyHint.
  ///
  /// In en, this message translates to:
  /// **'Find people in Search to start a direct message.'**
  String get chatListEmptyHint;

  /// No description provided for @chatListLoadMore.
  ///
  /// In en, this message translates to:
  /// **'Load more chats'**
  String get chatListLoadMore;

  /// No description provided for @chatListUnreadCount.
  ///
  /// In en, this message translates to:
  /// **'{count} unread'**
  String chatListUnreadCount(int count);

  /// No description provided for @chatListLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load chats'**
  String get chatListLoadError;

  /// No description provided for @chatListBackendUnavailable.
  ///
  /// In en, this message translates to:
  /// **'Chats are unavailable. Start the full API stack (docker compose --profile app).'**
  String get chatListBackendUnavailable;

  /// No description provided for @chatListDmFallback.
  ///
  /// In en, this message translates to:
  /// **'Chat {id}'**
  String chatListDmFallback(String id);

  /// No description provided for @chatRoomSelectPrompt.
  ///
  /// In en, this message translates to:
  /// **'Select a conversation'**
  String get chatRoomSelectPrompt;

  /// No description provided for @chatRoomBack.
  ///
  /// In en, this message translates to:
  /// **'Back to chats'**
  String get chatRoomBack;

  /// No description provided for @chatRoomTitle.
  ///
  /// In en, this message translates to:
  /// **'Chat {id}'**
  String chatRoomTitle(String id);

  /// No description provided for @chatRoomEmpty.
  ///
  /// In en, this message translates to:
  /// **'No messages yet'**
  String get chatRoomEmpty;

  /// No description provided for @chatRoomEmptyHint.
  ///
  /// In en, this message translates to:
  /// **'Send the first message when you are ready.'**
  String get chatRoomEmptyHint;

  /// No description provided for @chatRoomLoadOlder.
  ///
  /// In en, this message translates to:
  /// **'Load older messages'**
  String get chatRoomLoadOlder;

  /// No description provided for @chatRoomInputHint.
  ///
  /// In en, this message translates to:
  /// **'Message'**
  String get chatRoomInputHint;

  /// No description provided for @chatSendMessage.
  ///
  /// In en, this message translates to:
  /// **'Send message'**
  String get chatSendMessage;

  /// No description provided for @chatMentionInsert.
  ///
  /// In en, this message translates to:
  /// **'Insert mention'**
  String get chatMentionInsert;

  /// No description provided for @chatMentionEveryone.
  ///
  /// In en, this message translates to:
  /// **'@everyone'**
  String get chatMentionEveryone;

  /// No description provided for @chatMentionHere.
  ///
  /// In en, this message translates to:
  /// **'@here'**
  String get chatMentionHere;

  /// No description provided for @chatMentionMember.
  ///
  /// In en, this message translates to:
  /// **'Member {profileId}'**
  String chatMentionMember(String profileId);

  /// No description provided for @chatRoomError.
  ///
  /// In en, this message translates to:
  /// **'Error: {message}'**
  String chatRoomError(String message);

  /// No description provided for @chatRealtimeConnected.
  ///
  /// In en, this message translates to:
  /// **'Live'**
  String get chatRealtimeConnected;

  /// No description provided for @chatRealtimeConnecting.
  ///
  /// In en, this message translates to:
  /// **'Connecting…'**
  String get chatRealtimeConnecting;

  /// No description provided for @chatRealtimeReconnecting.
  ///
  /// In en, this message translates to:
  /// **'Reconnecting…'**
  String get chatRealtimeReconnecting;

  /// No description provided for @chatRealtimeOffline.
  ///
  /// In en, this message translates to:
  /// **'Offline'**
  String get chatRealtimeOffline;

  /// No description provided for @chatOfflineReadOnly.
  ///
  /// In en, this message translates to:
  /// **'You\'re offline. Showing saved messages.'**
  String get chatOfflineReadOnly;

  /// No description provided for @chatOfflineSendBlocked.
  ///
  /// In en, this message translates to:
  /// **'Can\'t send messages while offline.'**
  String get chatOfflineSendBlocked;

  /// No description provided for @chatOpenDm.
  ///
  /// In en, this message translates to:
  /// **'Message'**
  String get chatOpenDm;

  /// No description provided for @callStartAudio.
  ///
  /// In en, this message translates to:
  /// **'Start audio call'**
  String get callStartAudio;

  /// No description provided for @callStartVideo.
  ///
  /// In en, this message translates to:
  /// **'Start video call'**
  String get callStartVideo;

  /// No description provided for @callIncomingTitle.
  ///
  /// In en, this message translates to:
  /// **'{name} is calling'**
  String callIncomingTitle(String name);

  /// No description provided for @callIncomingAudio.
  ///
  /// In en, this message translates to:
  /// **'Audio call'**
  String get callIncomingAudio;

  /// No description provided for @callIncomingVideo.
  ///
  /// In en, this message translates to:
  /// **'Video call'**
  String get callIncomingVideo;

  /// No description provided for @callAccept.
  ///
  /// In en, this message translates to:
  /// **'Accept'**
  String get callAccept;

  /// No description provided for @callDecline.
  ///
  /// In en, this message translates to:
  /// **'Decline'**
  String get callDecline;

  /// No description provided for @callConnecting.
  ///
  /// In en, this message translates to:
  /// **'Connecting call…'**
  String get callConnecting;

  /// No description provided for @callActive.
  ///
  /// In en, this message translates to:
  /// **'Call active'**
  String get callActive;

  /// No description provided for @callTapToEnableAudio.
  ///
  /// In en, this message translates to:
  /// **'Tap to enable incoming audio'**
  String get callTapToEnableAudio;

  /// No description provided for @callMute.
  ///
  /// In en, this message translates to:
  /// **'Mute'**
  String get callMute;

  /// No description provided for @callUnmute.
  ///
  /// In en, this message translates to:
  /// **'Unmute'**
  String get callUnmute;

  /// No description provided for @callSpeakerOff.
  ///
  /// In en, this message translates to:
  /// **'Mute speakers'**
  String get callSpeakerOff;

  /// No description provided for @callSpeakerOn.
  ///
  /// In en, this message translates to:
  /// **'Unmute speakers'**
  String get callSpeakerOn;

  /// No description provided for @callVideoOn.
  ///
  /// In en, this message translates to:
  /// **'Turn camera on'**
  String get callVideoOn;

  /// No description provided for @callVideoOff.
  ///
  /// In en, this message translates to:
  /// **'Turn camera off'**
  String get callVideoOff;

  /// No description provided for @callHangup.
  ///
  /// In en, this message translates to:
  /// **'Hang up'**
  String get callHangup;

  /// No description provided for @callOutgoingTitle.
  ///
  /// In en, this message translates to:
  /// **'Calling {name}…'**
  String callOutgoingTitle(String name);

  /// No description provided for @callFailed.
  ///
  /// In en, this message translates to:
  /// **'Could not start call: {message}'**
  String callFailed(String message);

  /// No description provided for @callLivekitConnectFailed.
  ///
  /// In en, this message translates to:
  /// **'Could not connect to LiveKit'**
  String get callLivekitConnectFailed;

  /// No description provided for @callActiveCallExists.
  ///
  /// In en, this message translates to:
  /// **'You already have an active call'**
  String get callActiveCallExists;

  /// No description provided for @callGroupVoiceStart.
  ///
  /// In en, this message translates to:
  /// **'Start group voice'**
  String get callGroupVoiceStart;

  /// No description provided for @callGroupVoiceJoin.
  ///
  /// In en, this message translates to:
  /// **'Join voice'**
  String get callGroupVoiceJoin;

  /// No description provided for @callGroupVoiceActive.
  ///
  /// In en, this message translates to:
  /// **'Group voice active'**
  String get callGroupVoiceActive;

  /// No description provided for @callGroupVoiceInProgress.
  ///
  /// In en, this message translates to:
  /// **'Voice call in progress in this group'**
  String get callGroupVoiceInProgress;

  /// No description provided for @profileMessage.
  ///
  /// In en, this message translates to:
  /// **'Message'**
  String get profileMessage;

  /// No description provided for @profileEditTitle.
  ///
  /// In en, this message translates to:
  /// **'Edit profile'**
  String get profileEditTitle;

  /// No description provided for @profileEditTooltip.
  ///
  /// In en, this message translates to:
  /// **'Edit profile'**
  String get profileEditTooltip;

  /// No description provided for @profileDisplayNameLabel.
  ///
  /// In en, this message translates to:
  /// **'Display name'**
  String get profileDisplayNameLabel;

  /// No description provided for @profileBioLabel.
  ///
  /// In en, this message translates to:
  /// **'About'**
  String get profileBioLabel;

  /// No description provided for @profileBioHelper.
  ///
  /// In en, this message translates to:
  /// **'Up to 500 characters'**
  String get profileBioHelper;

  /// No description provided for @profileAvatarChange.
  ///
  /// In en, this message translates to:
  /// **'Change avatar'**
  String get profileAvatarChange;

  /// No description provided for @profileAvatarSelected.
  ///
  /// In en, this message translates to:
  /// **'Selected: {fileName}'**
  String profileAvatarSelected(String fileName);

  /// No description provided for @profileSave.
  ///
  /// In en, this message translates to:
  /// **'Save'**
  String get profileSave;

  /// No description provided for @profileErrorDisplayNameRequired.
  ///
  /// In en, this message translates to:
  /// **'Enter a display name.'**
  String get profileErrorDisplayNameRequired;

  /// No description provided for @profileErrorDisplayNameTooLong.
  ///
  /// In en, this message translates to:
  /// **'Display name must be 32 characters or fewer.'**
  String get profileErrorDisplayNameTooLong;

  /// No description provided for @profileErrorBioTooLong.
  ///
  /// In en, this message translates to:
  /// **'About must be 500 characters or fewer.'**
  String get profileErrorBioTooLong;

  /// No description provided for @profileErrorAvatarType.
  ///
  /// In en, this message translates to:
  /// **'Use a static JPEG, PNG, or WebP image.'**
  String get profileErrorAvatarType;

  /// No description provided for @profileErrorAvatarTooLarge.
  ///
  /// In en, this message translates to:
  /// **'Avatar must be a non-empty image up to 5 MB.'**
  String get profileErrorAvatarTooLarge;

  /// No description provided for @profileEditSaveError.
  ///
  /// In en, this message translates to:
  /// **'Could not save profile: {message}'**
  String profileEditSaveError(String message);

  /// No description provided for @commonCancel.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get commonCancel;

  /// No description provided for @commonEdit.
  ///
  /// In en, this message translates to:
  /// **'Edit'**
  String get commonEdit;

  /// No description provided for @commonDelete.
  ///
  /// In en, this message translates to:
  /// **'Delete'**
  String get commonDelete;

  /// No description provided for @commonSave.
  ///
  /// In en, this message translates to:
  /// **'Save'**
  String get commonSave;

  /// No description provided for @commonRetry.
  ///
  /// In en, this message translates to:
  /// **'Try again'**
  String get commonRetry;

  /// No description provided for @commonLoading.
  ///
  /// In en, this message translates to:
  /// **'…'**
  String get commonLoading;

  /// No description provided for @chatInboxDm.
  ///
  /// In en, this message translates to:
  /// **'DMs'**
  String get chatInboxDm;

  /// No description provided for @chatInboxRequests.
  ///
  /// In en, this message translates to:
  /// **'Requests'**
  String get chatInboxRequests;

  /// No description provided for @chatTyping.
  ///
  /// In en, this message translates to:
  /// **'Typing…'**
  String get chatTyping;

  /// No description provided for @chatAttachFile.
  ///
  /// In en, this message translates to:
  /// **'Attach file'**
  String get chatAttachFile;

  /// No description provided for @chatMessageEdit.
  ///
  /// In en, this message translates to:
  /// **'Edit'**
  String get chatMessageEdit;

  /// No description provided for @chatMessageForward.
  ///
  /// In en, this message translates to:
  /// **'Forward'**
  String get chatMessageForward;

  /// No description provided for @chatMessageReply.
  ///
  /// In en, this message translates to:
  /// **'Reply'**
  String get chatMessageReply;

  /// No description provided for @chatReplyingTo.
  ///
  /// In en, this message translates to:
  /// **'Replying to {preview}'**
  String chatReplyingTo(String preview);

  /// No description provided for @chatThreadTitle.
  ///
  /// In en, this message translates to:
  /// **'Thread'**
  String get chatThreadTitle;

  /// No description provided for @chatThreadEmpty.
  ///
  /// In en, this message translates to:
  /// **'No replies yet'**
  String get chatThreadEmpty;

  /// No description provided for @chatChannelMainFeedBlocked.
  ///
  /// In en, this message translates to:
  /// **'Post in a thread or as the channel'**
  String get chatChannelMainFeedBlocked;

  /// No description provided for @chatMessageAddReaction.
  ///
  /// In en, this message translates to:
  /// **'Add reaction'**
  String get chatMessageAddReaction;

  /// No description provided for @chatMessagePin.
  ///
  /// In en, this message translates to:
  /// **'Pin message'**
  String get chatMessagePin;

  /// No description provided for @chatMessageUnpin.
  ///
  /// In en, this message translates to:
  /// **'Unpin message'**
  String get chatMessageUnpin;

  /// No description provided for @chatPinnedBar.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 pinned message} other{{count} pinned messages}}'**
  String chatPinnedBar(int count);

  /// No description provided for @chatForwardTitle.
  ///
  /// In en, this message translates to:
  /// **'Forward to'**
  String get chatForwardTitle;

  /// No description provided for @chatForwardSearchHint.
  ///
  /// In en, this message translates to:
  /// **'Search chats'**
  String get chatForwardSearchHint;

  /// No description provided for @chatForwardFrom.
  ///
  /// In en, this message translates to:
  /// **'Forwarded from {sender}'**
  String chatForwardFrom(String sender);

  /// No description provided for @chatForwardCommentaryTitle.
  ///
  /// In en, this message translates to:
  /// **'Add a comment'**
  String get chatForwardCommentaryTitle;

  /// No description provided for @chatForwardCommentaryHint.
  ///
  /// In en, this message translates to:
  /// **'Optional message before the forward'**
  String get chatForwardCommentaryHint;

  /// No description provided for @chatForwardEmpty.
  ///
  /// In en, this message translates to:
  /// **'No chats to forward to'**
  String get chatForwardEmpty;

  /// No description provided for @chatForwardSuccess.
  ///
  /// In en, this message translates to:
  /// **'Message forwarded'**
  String get chatForwardSuccess;

  /// No description provided for @chatForwardError.
  ///
  /// In en, this message translates to:
  /// **'Could not forward message: {message}'**
  String chatForwardError(String message);

  /// No description provided for @chatMessageDeleteForMe.
  ///
  /// In en, this message translates to:
  /// **'Delete for me'**
  String get chatMessageDeleteForMe;

  /// No description provided for @chatMessageDeleteForEveryone.
  ///
  /// In en, this message translates to:
  /// **'Delete for everyone'**
  String get chatMessageDeleteForEveryone;

  /// No description provided for @chatEditMessageTitle.
  ///
  /// In en, this message translates to:
  /// **'Edit message'**
  String get chatEditMessageTitle;

  /// No description provided for @chatMessageEdited.
  ///
  /// In en, this message translates to:
  /// **'(edited)'**
  String get chatMessageEdited;

  /// No description provided for @chatDeliverySent.
  ///
  /// In en, this message translates to:
  /// **'Sent'**
  String get chatDeliverySent;

  /// No description provided for @chatDeliveryDelivered.
  ///
  /// In en, this message translates to:
  /// **'Delivered'**
  String get chatDeliveryDelivered;

  /// No description provided for @chatDeliveryRead.
  ///
  /// In en, this message translates to:
  /// **'Read'**
  String get chatDeliveryRead;

  /// No description provided for @chatImageAttachment.
  ///
  /// In en, this message translates to:
  /// **'Image attachment'**
  String get chatImageAttachment;

  /// No description provided for @chatNewMessages.
  ///
  /// In en, this message translates to:
  /// **'New messages'**
  String get chatNewMessages;

  /// No description provided for @chatUnreadSeparator.
  ///
  /// In en, this message translates to:
  /// **'Unread messages'**
  String get chatUnreadSeparator;

  /// No description provided for @chatListStrangerBadge.
  ///
  /// In en, this message translates to:
  /// **'Stranger'**
  String get chatListStrangerBadge;

  /// No description provided for @chatCreateGroupTooltip.
  ///
  /// In en, this message translates to:
  /// **'Create group'**
  String get chatCreateGroupTooltip;

  /// No description provided for @chatCreateGroupTitle.
  ///
  /// In en, this message translates to:
  /// **'New group'**
  String get chatCreateGroupTitle;

  /// No description provided for @chatCreateGroupNameLabel.
  ///
  /// In en, this message translates to:
  /// **'Group name'**
  String get chatCreateGroupNameLabel;

  /// No description provided for @chatCreateGroupNameHint.
  ///
  /// In en, this message translates to:
  /// **'Friday squad'**
  String get chatCreateGroupNameHint;

  /// No description provided for @chatCreateGroupMembers.
  ///
  /// In en, this message translates to:
  /// **'Add members'**
  String get chatCreateGroupMembers;

  /// No description provided for @chatCreateGroupMembersHint.
  ///
  /// In en, this message translates to:
  /// **'Select at least 2 friends (3 people total including you).'**
  String get chatCreateGroupMembersHint;

  /// No description provided for @chatCreateGroupSubmit.
  ///
  /// In en, this message translates to:
  /// **'Create group'**
  String get chatCreateGroupSubmit;

  /// No description provided for @chatCreateGroupMinMembers.
  ///
  /// In en, this message translates to:
  /// **'Select at least 2 friends to create a group.'**
  String get chatCreateGroupMinMembers;

  /// No description provided for @chatCreateGroupFriendsEmptyHint.
  ///
  /// In en, this message translates to:
  /// **'Add friends first, then invite them to a group.'**
  String get chatCreateGroupFriendsEmptyHint;

  /// No description provided for @chatCreateGroupError.
  ///
  /// In en, this message translates to:
  /// **'Could not create group: {message}'**
  String chatCreateGroupError(String message);

  /// No description provided for @spaceCreateTooltip.
  ///
  /// In en, this message translates to:
  /// **'Create space'**
  String get spaceCreateTooltip;

  /// No description provided for @spaceCreateTitle.
  ///
  /// In en, this message translates to:
  /// **'New space'**
  String get spaceCreateTitle;

  /// No description provided for @spaceCreateNameLabel.
  ///
  /// In en, this message translates to:
  /// **'Space name'**
  String get spaceCreateNameLabel;

  /// No description provided for @spaceCreateNameHint.
  ///
  /// In en, this message translates to:
  /// **'Friday squad'**
  String get spaceCreateNameHint;

  /// No description provided for @spaceCreateDescriptionLabel.
  ///
  /// In en, this message translates to:
  /// **'Description'**
  String get spaceCreateDescriptionLabel;

  /// No description provided for @spaceCreateDescriptionHint.
  ///
  /// In en, this message translates to:
  /// **'What is this space about?'**
  String get spaceCreateDescriptionHint;

  /// No description provided for @spaceCreateIconLabel.
  ///
  /// In en, this message translates to:
  /// **'Icon URL'**
  String get spaceCreateIconLabel;

  /// No description provided for @spaceCreateIconHint.
  ///
  /// In en, this message translates to:
  /// **'https://cdn.example/icon.webp'**
  String get spaceCreateIconHint;

  /// No description provided for @spaceCreateSubmit.
  ///
  /// In en, this message translates to:
  /// **'Create space'**
  String get spaceCreateSubmit;

  /// No description provided for @spaceCreateError.
  ///
  /// In en, this message translates to:
  /// **'Could not create space: {message}'**
  String spaceCreateError(String message);

  /// No description provided for @spaceTreeTitle.
  ///
  /// In en, this message translates to:
  /// **'Channels'**
  String get spaceTreeTitle;

  /// No description provided for @spaceTreeEmpty.
  ///
  /// In en, this message translates to:
  /// **'No channels yet'**
  String get spaceTreeEmpty;

  /// No description provided for @spaceTreeLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load space tree'**
  String get spaceTreeLoadError;

  /// No description provided for @spaceTreeTextChat.
  ///
  /// In en, this message translates to:
  /// **'Text chat'**
  String get spaceTreeTextChat;

  /// No description provided for @spaceTreeVoiceRoom.
  ///
  /// In en, this message translates to:
  /// **'Voice room'**
  String get spaceTreeVoiceRoom;

  /// No description provided for @spaceTreeUncategorized.
  ///
  /// In en, this message translates to:
  /// **'Channels'**
  String get spaceTreeUncategorized;

  /// No description provided for @spaceSelectPrompt.
  ///
  /// In en, this message translates to:
  /// **'Select a space'**
  String get spaceSelectPrompt;

  /// No description provided for @spaceListTitle.
  ///
  /// In en, this message translates to:
  /// **'My spaces'**
  String get spaceListTitle;

  /// No description provided for @spaceOpenAction.
  ///
  /// In en, this message translates to:
  /// **'Open space'**
  String get spaceOpenAction;

  /// No description provided for @spaceInvitesTooltip.
  ///
  /// In en, this message translates to:
  /// **'Invite people'**
  String get spaceInvitesTooltip;

  /// No description provided for @spaceInvitesTitle.
  ///
  /// In en, this message translates to:
  /// **'Invite links'**
  String get spaceInvitesTitle;

  /// No description provided for @spaceInvitesSubtitle.
  ///
  /// In en, this message translates to:
  /// **'Create a link to invite people to this space.'**
  String get spaceInvitesSubtitle;

  /// No description provided for @spaceInvitesEmpty.
  ///
  /// In en, this message translates to:
  /// **'No active invite links'**
  String get spaceInvitesEmpty;

  /// No description provided for @spaceInvitesLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load invites'**
  String get spaceInvitesLoadError;

  /// No description provided for @spaceInvitesRetry.
  ///
  /// In en, this message translates to:
  /// **'Retry'**
  String get spaceInvitesRetry;

  /// No description provided for @spaceInviteCreate.
  ///
  /// In en, this message translates to:
  /// **'Create link'**
  String get spaceInviteCreate;

  /// No description provided for @spaceInviteAdvancedToggle.
  ///
  /// In en, this message translates to:
  /// **'Advanced'**
  String get spaceInviteAdvancedToggle;

  /// No description provided for @spaceInviteMaxUsesLabel.
  ///
  /// In en, this message translates to:
  /// **'Max uses'**
  String get spaceInviteMaxUsesLabel;

  /// No description provided for @spaceInviteMaxUsesHint.
  ///
  /// In en, this message translates to:
  /// **'Unlimited if empty'**
  String get spaceInviteMaxUsesHint;

  /// No description provided for @spaceInviteMaxUsesInvalid.
  ///
  /// In en, this message translates to:
  /// **'Max uses must be a positive number'**
  String get spaceInviteMaxUsesInvalid;

  /// No description provided for @spaceInviteCreateError.
  ///
  /// In en, this message translates to:
  /// **'Could not create invite: {message}'**
  String spaceInviteCreateError(String message);

  /// No description provided for @spaceInviteRevokeError.
  ///
  /// In en, this message translates to:
  /// **'Could not revoke invite: {message}'**
  String spaceInviteRevokeError(String message);

  /// No description provided for @spaceInviteCopy.
  ///
  /// In en, this message translates to:
  /// **'Copy link'**
  String get spaceInviteCopy;

  /// No description provided for @spaceInviteCopied.
  ///
  /// In en, this message translates to:
  /// **'Invite link copied'**
  String get spaceInviteCopied;

  /// No description provided for @spaceInviteRevoke.
  ///
  /// In en, this message translates to:
  /// **'Revoke'**
  String get spaceInviteRevoke;

  /// No description provided for @spaceInviteUses.
  ///
  /// In en, this message translates to:
  /// **'{used} uses{maxSuffix}'**
  String spaceInviteUses(int used, String maxSuffix);

  /// No description provided for @spaceInviteJoinTooltip.
  ///
  /// In en, this message translates to:
  /// **'Join space by invite'**
  String get spaceInviteJoinTooltip;

  /// No description provided for @spaceInviteJoinTitle.
  ///
  /// In en, this message translates to:
  /// **'Join a space'**
  String get spaceInviteJoinTitle;

  /// No description provided for @spaceInviteJoinSubtitle.
  ///
  /// In en, this message translates to:
  /// **'Paste an invite code or link from a friend.'**
  String get spaceInviteJoinSubtitle;

  /// No description provided for @spaceInviteJoinCodeLabel.
  ///
  /// In en, this message translates to:
  /// **'Invite code'**
  String get spaceInviteJoinCodeLabel;

  /// No description provided for @spaceInviteJoinCodeHint.
  ///
  /// In en, this message translates to:
  /// **'abc123xyz'**
  String get spaceInviteJoinCodeHint;

  /// No description provided for @spaceInviteJoinSubmit.
  ///
  /// In en, this message translates to:
  /// **'Join space'**
  String get spaceInviteJoinSubmit;

  /// No description provided for @spaceInviteJoinError.
  ///
  /// In en, this message translates to:
  /// **'Could not join space: {message}'**
  String spaceInviteJoinError(String message);

  /// No description provided for @spaceMembersTooltip.
  ///
  /// In en, this message translates to:
  /// **'Space members'**
  String get spaceMembersTooltip;

  /// No description provided for @spaceMembersTitle.
  ///
  /// In en, this message translates to:
  /// **'Members'**
  String get spaceMembersTitle;

  /// No description provided for @spaceMembersSubtitle.
  ///
  /// In en, this message translates to:
  /// **'Owners and admins can assign roles and remove members.'**
  String get spaceMembersSubtitle;

  /// No description provided for @spaceMembersLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load members'**
  String get spaceMembersLoadError;

  /// No description provided for @spaceMemberYou.
  ///
  /// In en, this message translates to:
  /// **'{name} (you)'**
  String spaceMemberYou(String name);

  /// No description provided for @spaceKick.
  ///
  /// In en, this message translates to:
  /// **'Remove'**
  String get spaceKick;

  /// No description provided for @spaceKickConfirmTitle.
  ///
  /// In en, this message translates to:
  /// **'Remove member?'**
  String get spaceKickConfirmTitle;

  /// No description provided for @spaceKickConfirmMessage.
  ///
  /// In en, this message translates to:
  /// **'Remove {name} from this space?'**
  String spaceKickConfirmMessage(String name);

  /// No description provided for @spaceKickError.
  ///
  /// In en, this message translates to:
  /// **'Could not remove member: {message}'**
  String spaceKickError(String message);

  /// No description provided for @spaceBan.
  ///
  /// In en, this message translates to:
  /// **'Ban'**
  String get spaceBan;

  /// No description provided for @spaceBanConfirmTitle.
  ///
  /// In en, this message translates to:
  /// **'Ban member?'**
  String get spaceBanConfirmTitle;

  /// No description provided for @spaceBanConfirmMessage.
  ///
  /// In en, this message translates to:
  /// **'Ban {name} from this space? They will not be able to rejoin.'**
  String spaceBanConfirmMessage(String name);

  /// No description provided for @spaceBanError.
  ///
  /// In en, this message translates to:
  /// **'Could not ban member: {message}'**
  String spaceBanError(String message);

  /// No description provided for @spaceTimeout.
  ///
  /// In en, this message translates to:
  /// **'Timeout'**
  String get spaceTimeout;

  /// No description provided for @spaceTimeoutConfirmTitle.
  ///
  /// In en, this message translates to:
  /// **'Timeout member?'**
  String get spaceTimeoutConfirmTitle;

  /// No description provided for @spaceTimeoutConfirmMessage.
  ///
  /// In en, this message translates to:
  /// **'Prevent {name} from sending messages for 10 minutes?'**
  String spaceTimeoutConfirmMessage(String name);

  /// No description provided for @spaceTimeoutError.
  ///
  /// In en, this message translates to:
  /// **'Could not timeout member: {message}'**
  String spaceTimeoutError(String message);

  /// No description provided for @spaceSlowMode.
  ///
  /// In en, this message translates to:
  /// **'Slow mode'**
  String get spaceSlowMode;

  /// No description provided for @spaceSlowModeSubtitle.
  ///
  /// In en, this message translates to:
  /// **'Minimum delay between messages in this channel'**
  String get spaceSlowModeSubtitle;

  /// No description provided for @spaceSlowModeOff.
  ///
  /// In en, this message translates to:
  /// **'Off'**
  String get spaceSlowModeOff;

  /// No description provided for @spaceSlowModeSeconds.
  ///
  /// In en, this message translates to:
  /// **'{seconds}s'**
  String spaceSlowModeSeconds(int seconds);

  /// No description provided for @spaceAssignRole.
  ///
  /// In en, this message translates to:
  /// **'Assign role'**
  String get spaceAssignRole;

  /// No description provided for @spaceAssignRoleTitle.
  ///
  /// In en, this message translates to:
  /// **'Assign role'**
  String get spaceAssignRoleTitle;

  /// No description provided for @spaceAssignRoleEmpty.
  ///
  /// In en, this message translates to:
  /// **'No assignable roles'**
  String get spaceAssignRoleEmpty;

  /// No description provided for @spaceAssignRoleError.
  ///
  /// In en, this message translates to:
  /// **'Could not assign role: {message}'**
  String spaceAssignRoleError(String message);

  /// No description provided for @spaceRolesTooltip.
  ///
  /// In en, this message translates to:
  /// **'Manage roles'**
  String get spaceRolesTooltip;

  /// No description provided for @spaceRolesTitle.
  ///
  /// In en, this message translates to:
  /// **'Roles'**
  String get spaceRolesTitle;

  /// No description provided for @spaceRolesLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load roles'**
  String get spaceRolesLoadError;

  /// No description provided for @spaceRoleCreateTitle.
  ///
  /// In en, this message translates to:
  /// **'Create role'**
  String get spaceRoleCreateTitle;

  /// No description provided for @spaceRoleEditTitle.
  ///
  /// In en, this message translates to:
  /// **'Edit role'**
  String get spaceRoleEditTitle;

  /// No description provided for @spaceRoleNameLabel.
  ///
  /// In en, this message translates to:
  /// **'Role name'**
  String get spaceRoleNameLabel;

  /// No description provided for @spaceRoleManaged.
  ///
  /// In en, this message translates to:
  /// **'System role'**
  String get spaceRoleManaged;

  /// No description provided for @spaceRoleCustom.
  ///
  /// In en, this message translates to:
  /// **'Custom role'**
  String get spaceRoleCustom;

  /// No description provided for @spaceChatOverrideTitle.
  ///
  /// In en, this message translates to:
  /// **'Chat access overrides'**
  String get spaceChatOverrideTitle;

  /// No description provided for @spaceChatOverrideHint.
  ///
  /// In en, this message translates to:
  /// **'Deny view or send for a role in this chat only.'**
  String get spaceChatOverrideHint;

  /// No description provided for @spaceChatOverrideDenyView.
  ///
  /// In en, this message translates to:
  /// **'Deny view chat'**
  String get spaceChatOverrideDenyView;

  /// No description provided for @spaceChatOverrideDenySend.
  ///
  /// In en, this message translates to:
  /// **'Deny send messages'**
  String get spaceChatOverrideDenySend;

  /// No description provided for @spaceVoiceOverrideTitle.
  ///
  /// In en, this message translates to:
  /// **'Voice room access overrides'**
  String get spaceVoiceOverrideTitle;

  /// No description provided for @spaceVoiceOverrideHint.
  ///
  /// In en, this message translates to:
  /// **'Deny join for a role in this voice room only.'**
  String get spaceVoiceOverrideHint;

  /// No description provided for @spaceVoiceOverrideDenyJoin.
  ///
  /// In en, this message translates to:
  /// **'Deny join voice'**
  String get spaceVoiceOverrideDenyJoin;

  /// No description provided for @spaceSetDefaultJoinRole.
  ///
  /// In en, this message translates to:
  /// **'Set as default join role'**
  String get spaceSetDefaultJoinRole;

  /// No description provided for @spaceDefaultJoinRole.
  ///
  /// In en, this message translates to:
  /// **'Default join role: {name}'**
  String spaceDefaultJoinRole(String name);

  /// No description provided for @spaceRevokeRole.
  ///
  /// In en, this message translates to:
  /// **'Revoke'**
  String get spaceRevokeRole;

  /// No description provided for @spaceRevokeRoleError.
  ///
  /// In en, this message translates to:
  /// **'Could not revoke role: {message}'**
  String spaceRevokeRoleError(String message);

  /// No description provided for @chatGroupMembersTooltip.
  ///
  /// In en, this message translates to:
  /// **'Group members'**
  String get chatGroupMembersTooltip;

  /// No description provided for @chatGroupMembersTitle.
  ///
  /// In en, this message translates to:
  /// **'Members'**
  String get chatGroupMembersTitle;

  /// No description provided for @chatGroupMembersSubtitle.
  ///
  /// In en, this message translates to:
  /// **'Owner can remove members. Members can leave the group.'**
  String get chatGroupMembersSubtitle;

  /// No description provided for @chatGroupMembersLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load members'**
  String get chatGroupMembersLoadError;

  /// No description provided for @chatInfoTitle.
  ///
  /// In en, this message translates to:
  /// **'Chat info'**
  String get chatInfoTitle;

  /// No description provided for @chatInfoOpen.
  ///
  /// In en, this message translates to:
  /// **'Chat info'**
  String get chatInfoOpen;

  /// No description provided for @chatSharedMediaTabMedia.
  ///
  /// In en, this message translates to:
  /// **'Media'**
  String get chatSharedMediaTabMedia;

  /// No description provided for @chatSharedMediaTabFiles.
  ///
  /// In en, this message translates to:
  /// **'Files'**
  String get chatSharedMediaTabFiles;

  /// No description provided for @chatSharedMediaTabLinks.
  ///
  /// In en, this message translates to:
  /// **'Links'**
  String get chatSharedMediaTabLinks;

  /// No description provided for @chatSharedMediaTabVoice.
  ///
  /// In en, this message translates to:
  /// **'Voice'**
  String get chatSharedMediaTabVoice;

  /// No description provided for @chatSharedMediaEmpty.
  ///
  /// In en, this message translates to:
  /// **'Nothing here yet'**
  String get chatSharedMediaEmpty;

  /// No description provided for @chatSharedMediaLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load shared media'**
  String get chatSharedMediaLoadError;

  /// No description provided for @chatGroupRoleOwner.
  ///
  /// In en, this message translates to:
  /// **'Owner'**
  String get chatGroupRoleOwner;

  /// No description provided for @chatGroupMemberYou.
  ///
  /// In en, this message translates to:
  /// **'{name} (you)'**
  String chatGroupMemberYou(String name);

  /// No description provided for @chatGroupKick.
  ///
  /// In en, this message translates to:
  /// **'Remove'**
  String get chatGroupKick;

  /// No description provided for @chatGroupKickConfirmTitle.
  ///
  /// In en, this message translates to:
  /// **'Remove member?'**
  String get chatGroupKickConfirmTitle;

  /// No description provided for @chatGroupKickConfirmMessage.
  ///
  /// In en, this message translates to:
  /// **'Remove {name} from the group?'**
  String chatGroupKickConfirmMessage(String name);

  /// No description provided for @chatGroupLeave.
  ///
  /// In en, this message translates to:
  /// **'Leave group'**
  String get chatGroupLeave;

  /// No description provided for @chatGroupLeaveConfirmTitle.
  ///
  /// In en, this message translates to:
  /// **'Leave group?'**
  String get chatGroupLeaveConfirmTitle;

  /// No description provided for @chatGroupLeaveConfirmMessage.
  ///
  /// In en, this message translates to:
  /// **'You will no longer receive messages from this group.'**
  String get chatGroupLeaveConfirmMessage;

  /// No description provided for @chatGroupOwnerLeaveHint.
  ///
  /// In en, this message translates to:
  /// **'Transfer ownership to another member before leaving.'**
  String get chatGroupOwnerLeaveHint;

  /// No description provided for @chatGroupTransferOwnershipTo.
  ///
  /// In en, this message translates to:
  /// **'Transfer ownership to {name}'**
  String chatGroupTransferOwnershipTo(String name);

  /// No description provided for @chatGroupTransferOwnershipTitle.
  ///
  /// In en, this message translates to:
  /// **'Transfer group ownership'**
  String get chatGroupTransferOwnershipTitle;

  /// No description provided for @chatGroupTransferOwnershipMessage.
  ///
  /// In en, this message translates to:
  /// **'Make {name} the new group owner?'**
  String chatGroupTransferOwnershipMessage(String name);

  /// No description provided for @chatGroupTransferOwnershipConfirm.
  ///
  /// In en, this message translates to:
  /// **'Transfer'**
  String get chatGroupTransferOwnershipConfirm;

  /// No description provided for @settingsTitle.
  ///
  /// In en, this message translates to:
  /// **'Settings'**
  String get settingsTitle;

  /// No description provided for @settingsTooltip.
  ///
  /// In en, this message translates to:
  /// **'Settings'**
  String get settingsTooltip;

  /// No description provided for @settingsTheme.
  ///
  /// In en, this message translates to:
  /// **'Theme'**
  String get settingsTheme;

  /// No description provided for @settingsThemeSystem.
  ///
  /// In en, this message translates to:
  /// **'System'**
  String get settingsThemeSystem;

  /// No description provided for @settingsThemeLight.
  ///
  /// In en, this message translates to:
  /// **'Light'**
  String get settingsThemeLight;

  /// No description provided for @settingsThemeDark.
  ///
  /// In en, this message translates to:
  /// **'Dark'**
  String get settingsThemeDark;

  /// No description provided for @settingsThemeHighContrast.
  ///
  /// In en, this message translates to:
  /// **'High contrast'**
  String get settingsThemeHighContrast;

  /// No description provided for @settingsLanguage.
  ///
  /// In en, this message translates to:
  /// **'Language'**
  String get settingsLanguage;

  /// No description provided for @settingsLanguageSystem.
  ///
  /// In en, this message translates to:
  /// **'System default'**
  String get settingsLanguageSystem;

  /// No description provided for @settingsLanguageEn.
  ///
  /// In en, this message translates to:
  /// **'English'**
  String get settingsLanguageEn;

  /// No description provided for @settingsLanguageRu.
  ///
  /// In en, this message translates to:
  /// **'Russian'**
  String get settingsLanguageRu;

  /// No description provided for @settingsAccent.
  ///
  /// In en, this message translates to:
  /// **'Profile accent'**
  String get settingsAccent;

  /// No description provided for @settingsSubscription.
  ///
  /// In en, this message translates to:
  /// **'Subscription'**
  String get settingsSubscription;

  /// No description provided for @subscriptionSettingsTitle.
  ///
  /// In en, this message translates to:
  /// **'Subscription'**
  String get subscriptionSettingsTitle;

  /// No description provided for @subscriptionCurrentPlan.
  ///
  /// In en, this message translates to:
  /// **'Current plan'**
  String get subscriptionCurrentPlan;

  /// No description provided for @subscriptionStatusFree.
  ///
  /// In en, this message translates to:
  /// **'Free'**
  String get subscriptionStatusFree;

  /// No description provided for @subscriptionStatusPremium.
  ///
  /// In en, this message translates to:
  /// **'Premium'**
  String get subscriptionStatusPremium;

  /// No description provided for @subscriptionBillingPeriod.
  ///
  /// In en, this message translates to:
  /// **'Billing: {period}'**
  String subscriptionBillingPeriod(String period);

  /// No description provided for @subscriptionUpgradeTitle.
  ///
  /// In en, this message translates to:
  /// **'Upgrade to Premium'**
  String get subscriptionUpgradeTitle;

  /// No description provided for @subscriptionUpgradeMonthly.
  ///
  /// In en, this message translates to:
  /// **'Premium — monthly'**
  String get subscriptionUpgradeMonthly;

  /// No description provided for @subscriptionUpgradeYearly.
  ///
  /// In en, this message translates to:
  /// **'Premium — yearly (−20%)'**
  String get subscriptionUpgradeYearly;

  /// No description provided for @subscriptionManageBilling.
  ///
  /// In en, this message translates to:
  /// **'Manage subscription'**
  String get subscriptionManageBilling;

  /// No description provided for @subscriptionCancel.
  ///
  /// In en, this message translates to:
  /// **'Cancel subscription'**
  String get subscriptionCancel;

  /// No description provided for @subscriptionProfilesLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load profiles'**
  String get subscriptionProfilesLoadError;

  /// No description provided for @downgradeProfilePickerTitle.
  ///
  /// In en, this message translates to:
  /// **'Choose 2 profiles to keep'**
  String get downgradeProfilePickerTitle;

  /// No description provided for @downgradeProfilePickerHint.
  ///
  /// In en, this message translates to:
  /// **'Other profiles will be frozen until you renew Premium.'**
  String get downgradeProfilePickerHint;

  /// No description provided for @downgradeProfilePickerConfirm.
  ///
  /// In en, this message translates to:
  /// **'Keep selected profiles'**
  String get downgradeProfilePickerConfirm;

  /// No description provided for @downgradeProfilePrimary.
  ///
  /// In en, this message translates to:
  /// **'Primary profile'**
  String get downgradeProfilePrimary;

  /// No description provided for @premiumBadgeLabel.
  ///
  /// In en, this message translates to:
  /// **'Premium'**
  String get premiumBadgeLabel;

  /// No description provided for @settingsSecurity.
  ///
  /// In en, this message translates to:
  /// **'Security & trust'**
  String get settingsSecurity;

  /// No description provided for @securitySettingsTitle.
  ///
  /// In en, this message translates to:
  /// **'Security'**
  String get securitySettingsTitle;

  /// No description provided for @verificationSettingsTitle.
  ///
  /// In en, this message translates to:
  /// **'Verification'**
  String get verificationSettingsTitle;

  /// No description provided for @verificationSettingsHint.
  ///
  /// In en, this message translates to:
  /// **'Link platforms to earn a verified badge, or verify your organization domain.'**
  String get verificationSettingsHint;

  /// No description provided for @verificationLinkedAccountsTitle.
  ///
  /// In en, this message translates to:
  /// **'Linked accounts'**
  String get verificationLinkedAccountsTitle;

  /// No description provided for @verificationLinkedAccountsEmpty.
  ///
  /// In en, this message translates to:
  /// **'No linked accounts yet.'**
  String get verificationLinkedAccountsEmpty;

  /// No description provided for @verificationLinkTwitch.
  ///
  /// In en, this message translates to:
  /// **'Link Twitch'**
  String get verificationLinkTwitch;

  /// No description provided for @verifiedBadgePersonal.
  ///
  /// In en, this message translates to:
  /// **'Verified'**
  String get verifiedBadgePersonal;

  /// No description provided for @verifiedBadgeOrganization.
  ///
  /// In en, this message translates to:
  /// **'Verified organization'**
  String get verifiedBadgeOrganization;

  /// No description provided for @security2faEnableTitle.
  ///
  /// In en, this message translates to:
  /// **'Two-factor authentication'**
  String get security2faEnableTitle;

  /// No description provided for @security2faEnableHint.
  ///
  /// In en, this message translates to:
  /// **'Confirm your password to start 2FA setup.'**
  String get security2faEnableHint;

  /// No description provided for @security2faContinue.
  ///
  /// In en, this message translates to:
  /// **'Continue'**
  String get security2faContinue;

  /// No description provided for @security2faScanQr.
  ///
  /// In en, this message translates to:
  /// **'Scan this QR code in your authenticator app'**
  String get security2faScanQr;

  /// No description provided for @security2faBackupCodesTitle.
  ///
  /// In en, this message translates to:
  /// **'Backup codes (save these now)'**
  String get security2faBackupCodesTitle;

  /// No description provided for @security2faVerifyTitle.
  ///
  /// In en, this message translates to:
  /// **'Verify authenticator'**
  String get security2faVerifyTitle;

  /// No description provided for @security2faVerifyHint.
  ///
  /// In en, this message translates to:
  /// **'Enter the 6-digit code or a backup code.'**
  String get security2faVerifyHint;

  /// No description provided for @security2faVerify.
  ///
  /// In en, this message translates to:
  /// **'Enable 2FA'**
  String get security2faVerify;

  /// No description provided for @security2faBackToQr.
  ///
  /// In en, this message translates to:
  /// **'Back to QR'**
  String get security2faBackToQr;

  /// No description provided for @security2faEnabled.
  ///
  /// In en, this message translates to:
  /// **'Two-factor authentication is enabled.'**
  String get security2faEnabled;

  /// No description provided for @privacySettingsTitle.
  ///
  /// In en, this message translates to:
  /// **'Privacy'**
  String get privacySettingsTitle;

  /// No description provided for @privacyLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load privacy settings'**
  String get privacyLoadError;

  /// No description provided for @privacySaved.
  ///
  /// In en, this message translates to:
  /// **'Privacy settings saved'**
  String get privacySaved;

  /// No description provided for @privacyPresetTitle.
  ///
  /// In en, this message translates to:
  /// **'Preset'**
  String get privacyPresetTitle;

  /// No description provided for @privacyPresetPersonal.
  ///
  /// In en, this message translates to:
  /// **'Personal'**
  String get privacyPresetPersonal;

  /// No description provided for @privacyPresetGaming.
  ///
  /// In en, this message translates to:
  /// **'Gaming'**
  String get privacyPresetGaming;

  /// No description provided for @privacyPresetWork.
  ///
  /// In en, this message translates to:
  /// **'Work'**
  String get privacyPresetWork;

  /// No description provided for @privacyAllowDm.
  ///
  /// In en, this message translates to:
  /// **'Who can message you'**
  String get privacyAllowDm;

  /// No description provided for @privacyAllowGuestDm.
  ///
  /// In en, this message translates to:
  /// **'Allow guest accounts in DMs'**
  String get privacyAllowGuestDm;

  /// No description provided for @privacyVisibilityTitle.
  ///
  /// In en, this message translates to:
  /// **'Visibility'**
  String get privacyVisibilityTitle;

  /// No description provided for @privacyShowOnline.
  ///
  /// In en, this message translates to:
  /// **'Online status'**
  String get privacyShowOnline;

  /// No description provided for @privacyShowGameStatus.
  ///
  /// In en, this message translates to:
  /// **'In-game status'**
  String get privacyShowGameStatus;

  /// No description provided for @privacyShowMmRating.
  ///
  /// In en, this message translates to:
  /// **'Matchmaking rating'**
  String get privacyShowMmRating;

  /// No description provided for @privacyShowPhone.
  ///
  /// In en, this message translates to:
  /// **'Phone number'**
  String get privacyShowPhone;

  /// No description provided for @privacyShowStories.
  ///
  /// In en, this message translates to:
  /// **'Stories'**
  String get privacyShowStories;

  /// No description provided for @privacyAllowFriendRequests.
  ///
  /// In en, this message translates to:
  /// **'Friend requests'**
  String get privacyAllowFriendRequests;

  /// No description provided for @privacyAudienceEveryone.
  ///
  /// In en, this message translates to:
  /// **'Everyone'**
  String get privacyAudienceEveryone;

  /// No description provided for @privacyAudienceFriends.
  ///
  /// In en, this message translates to:
  /// **'Friends'**
  String get privacyAudienceFriends;

  /// No description provided for @privacyAudienceFriendsOfFriends.
  ///
  /// In en, this message translates to:
  /// **'Friends of friends'**
  String get privacyAudienceFriendsOfFriends;

  /// No description provided for @privacyAudienceNobody.
  ///
  /// In en, this message translates to:
  /// **'Nobody'**
  String get privacyAudienceNobody;

  /// No description provided for @reportAction.
  ///
  /// In en, this message translates to:
  /// **'Report'**
  String get reportAction;

  /// No description provided for @reportTitle.
  ///
  /// In en, this message translates to:
  /// **'Report'**
  String get reportTitle;

  /// No description provided for @reportSubtitle.
  ///
  /// In en, this message translates to:
  /// **'Choose a category. We will review your report.'**
  String get reportSubtitle;

  /// No description provided for @reportCategorySpam.
  ///
  /// In en, this message translates to:
  /// **'Spam'**
  String get reportCategorySpam;

  /// No description provided for @reportCategoryHarassment.
  ///
  /// In en, this message translates to:
  /// **'Harassment'**
  String get reportCategoryHarassment;

  /// No description provided for @reportCategoryOffensive.
  ///
  /// In en, this message translates to:
  /// **'Offensive content'**
  String get reportCategoryOffensive;

  /// No description provided for @reportCategoryFake.
  ///
  /// In en, this message translates to:
  /// **'Fake / impersonation'**
  String get reportCategoryFake;

  /// No description provided for @reportCategoryMmToxic.
  ///
  /// In en, this message translates to:
  /// **'Cheating / MM toxic'**
  String get reportCategoryMmToxic;

  /// No description provided for @reportCategoryOther.
  ///
  /// In en, this message translates to:
  /// **'Other'**
  String get reportCategoryOther;

  /// No description provided for @reportCommentLabel.
  ///
  /// In en, this message translates to:
  /// **'Comment'**
  String get reportCommentLabel;

  /// No description provided for @reportCommentRequired.
  ///
  /// In en, this message translates to:
  /// **'Required for «Other» (up to 500 characters)'**
  String get reportCommentRequired;

  /// No description provided for @reportSubmit.
  ///
  /// In en, this message translates to:
  /// **'Submit report'**
  String get reportSubmit;

  /// No description provided for @reportAcceptedTitle.
  ///
  /// In en, this message translates to:
  /// **'Report accepted'**
  String get reportAcceptedTitle;

  /// No description provided for @reportAcceptedMessage.
  ///
  /// In en, this message translates to:
  /// **'We will review it shortly. You will not receive status updates.'**
  String get reportAcceptedMessage;

  /// No description provided for @authTotpStepTitle.
  ///
  /// In en, this message translates to:
  /// **'Two-factor code'**
  String get authTotpStepTitle;

  /// No description provided for @authTotpLabel.
  ///
  /// In en, this message translates to:
  /// **'Authenticator or backup code'**
  String get authTotpLabel;

  /// No description provided for @authTotpHelper.
  ///
  /// In en, this message translates to:
  /// **'Enter the code from your authenticator app or a backup code.'**
  String get authTotpHelper;

  /// No description provided for @authErrorTotpRequired.
  ///
  /// In en, this message translates to:
  /// **'Enter your two-factor code to continue.'**
  String get authErrorTotpRequired;

  /// No description provided for @authErrorInvalidTotp.
  ///
  /// In en, this message translates to:
  /// **'Invalid authenticator or backup code.'**
  String get authErrorInvalidTotp;

  /// No description provided for @authTagline.
  ///
  /// In en, this message translates to:
  /// **'Voice chat and messages for gamers'**
  String get authTagline;

  /// No description provided for @versionUpdateRequired.
  ///
  /// In en, this message translates to:
  /// **'Update required'**
  String get versionUpdateRequired;

  /// No description provided for @versionUpdateAvailable.
  ///
  /// In en, this message translates to:
  /// **'Update {version} available'**
  String versionUpdateAvailable(String version);

  /// No description provided for @versionUpdateAvailableGeneric.
  ///
  /// In en, this message translates to:
  /// **'Update available'**
  String get versionUpdateAvailableGeneric;

  /// No description provided for @versionUpdateLater.
  ///
  /// In en, this message translates to:
  /// **'Later'**
  String get versionUpdateLater;

  /// No description provided for @versionUpdateNow.
  ///
  /// In en, this message translates to:
  /// **'Update'**
  String get versionUpdateNow;

  /// No description provided for @versionRestartToUpdate.
  ///
  /// In en, this message translates to:
  /// **'Restart and update'**
  String get versionRestartToUpdate;

  /// No description provided for @profileBlock.
  ///
  /// In en, this message translates to:
  /// **'Block user'**
  String get profileBlock;

  /// No description provided for @profileBlockConfirmTitle.
  ///
  /// In en, this message translates to:
  /// **'Block this user?'**
  String get profileBlockConfirmTitle;

  /// No description provided for @profileBlockConfirmMessage.
  ///
  /// In en, this message translates to:
  /// **'They will not be able to message you.'**
  String get profileBlockConfirmMessage;

  /// No description provided for @callVideoPlaceholder.
  ///
  /// In en, this message translates to:
  /// **'Video preview'**
  String get callVideoPlaceholder;

  /// No description provided for @screenShareStart.
  ///
  /// In en, this message translates to:
  /// **'Share screen'**
  String get screenShareStart;

  /// No description provided for @screenShareStop.
  ///
  /// In en, this message translates to:
  /// **'Stop sharing'**
  String get screenShareStop;

  /// No description provided for @screenSharePause.
  ///
  /// In en, this message translates to:
  /// **'Pause share'**
  String get screenSharePause;

  /// No description provided for @screenShareResume.
  ///
  /// In en, this message translates to:
  /// **'Resume share'**
  String get screenShareResume;

  /// No description provided for @screenShareQualityTitle.
  ///
  /// In en, this message translates to:
  /// **'Share quality'**
  String get screenShareQualityTitle;

  /// No description provided for @screenShareQuality720p15.
  ///
  /// In en, this message translates to:
  /// **'720p · 15 FPS'**
  String get screenShareQuality720p15;

  /// No description provided for @screenShareQuality720p30.
  ///
  /// In en, this message translates to:
  /// **'720p · 30 FPS'**
  String get screenShareQuality720p30;

  /// No description provided for @screenShareLimitReached.
  ///
  /// In en, this message translates to:
  /// **'This voice chat already has 3 screen shares'**
  String get screenShareLimitReached;

  /// No description provided for @screenShareWaitingForVideo.
  ///
  /// In en, this message translates to:
  /// **'Waiting for screen video…'**
  String get screenShareWaitingForVideo;

  /// No description provided for @themeLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load theme'**
  String get themeLoadError;

  /// No description provided for @bootstrapRestoring.
  ///
  /// In en, this message translates to:
  /// **'Restoring session…'**
  String get bootstrapRestoring;

  /// No description provided for @gameCatalogTitle.
  ///
  /// In en, this message translates to:
  /// **'Game catalog'**
  String get gameCatalogTitle;

  /// No description provided for @gameCatalogEntry.
  ///
  /// In en, this message translates to:
  /// **'Browse games'**
  String get gameCatalogEntry;

  /// No description provided for @gameCatalogSearchHint.
  ///
  /// In en, this message translates to:
  /// **'Search games'**
  String get gameCatalogSearchHint;

  /// No description provided for @gameCatalogLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load game catalog'**
  String get gameCatalogLoadError;

  /// No description provided for @gameCatalogEmpty.
  ///
  /// In en, this message translates to:
  /// **'No games found'**
  String get gameCatalogEmpty;

  /// No description provided for @gameCatalogInGameRoles.
  ///
  /// In en, this message translates to:
  /// **'In-game roles'**
  String get gameCatalogInGameRoles;

  /// No description provided for @gameCatalogRankLadder.
  ///
  /// In en, this message translates to:
  /// **'Rank ladder'**
  String get gameCatalogRankLadder;

  /// No description provided for @gameCatalogRegions.
  ///
  /// In en, this message translates to:
  /// **'Regions: {regions}'**
  String gameCatalogRegions(String regions);

  /// No description provided for @gameCatalogModeSlots.
  ///
  /// In en, this message translates to:
  /// **'{slots} players · party {min}–{max}'**
  String gameCatalogModeSlots(int slots, int min, int max);

  /// No description provided for @playerProfileTitle.
  ///
  /// In en, this message translates to:
  /// **'Matchmaking profile'**
  String get playerProfileTitle;

  /// No description provided for @playerProfileEntry.
  ///
  /// In en, this message translates to:
  /// **'Matchmaking profile'**
  String get playerProfileEntry;

  /// No description provided for @playerProfileLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load matchmaking profile'**
  String get playerProfileLoadError;

  /// No description provided for @playerProfileEmpty.
  ///
  /// In en, this message translates to:
  /// **'No games configured yet'**
  String get playerProfileEmpty;

  /// No description provided for @playerProfileAddGame.
  ///
  /// In en, this message translates to:
  /// **'Add game'**
  String get playerProfileAddGame;

  /// No description provided for @playerProfileSave.
  ///
  /// In en, this message translates to:
  /// **'Save game profile'**
  String get playerProfileSave;

  /// No description provided for @playerProfileSection.
  ///
  /// In en, this message translates to:
  /// **'Matchmaking'**
  String get playerProfileSection;

  /// No description provided for @playerProfileForGame.
  ///
  /// In en, this message translates to:
  /// **'My profile for this game'**
  String get playerProfileForGame;

  /// No description provided for @queueSearchStart.
  ///
  /// In en, this message translates to:
  /// **'Start queue'**
  String get queueSearchStart;

  /// No description provided for @queueSearchTitle.
  ///
  /// In en, this message translates to:
  /// **'Find teammates'**
  String get queueSearchTitle;

  /// No description provided for @queueSearchSearching.
  ///
  /// In en, this message translates to:
  /// **'Searching for teammates…'**
  String get queueSearchSearching;

  /// No description provided for @queueSearchCancel.
  ///
  /// In en, this message translates to:
  /// **'Cancel search'**
  String get queueSearchCancel;

  /// No description provided for @queueSearchRegion.
  ///
  /// In en, this message translates to:
  /// **'Region'**
  String get queueSearchRegion;

  /// No description provided for @queueSearchRole.
  ///
  /// In en, this message translates to:
  /// **'Your role'**
  String get queueSearchRole;

  /// No description provided for @queueSearchRank.
  ///
  /// In en, this message translates to:
  /// **'Your rank'**
  String get queueSearchRank;

  /// No description provided for @queueSearchSoughtRankMin.
  ///
  /// In en, this message translates to:
  /// **'Min rank sought'**
  String get queueSearchSoughtRankMin;

  /// No description provided for @queueSearchSoughtRankMax.
  ///
  /// In en, this message translates to:
  /// **'Max rank sought'**
  String get queueSearchSoughtRankMax;

  /// No description provided for @queueSearchStartError.
  ///
  /// In en, this message translates to:
  /// **'Could not start search'**
  String get queueSearchStartError;

  /// No description provided for @queueSearchCancelError.
  ///
  /// In en, this message translates to:
  /// **'Could not cancel search'**
  String get queueSearchCancelError;

  /// No description provided for @queueSearchNudgeTitle.
  ///
  /// In en, this message translates to:
  /// **'Still searching'**
  String get queueSearchNudgeTitle;

  /// No description provided for @queueSearchNudgeBody.
  ///
  /// In en, this message translates to:
  /// **'Taking a while. Try adjusting your search parameters.'**
  String get queueSearchNudgeBody;

  /// No description provided for @queueSearchTimeoutTitle.
  ///
  /// In en, this message translates to:
  /// **'No match found'**
  String get queueSearchTimeoutTitle;

  /// No description provided for @queueSearchTimeoutBody.
  ///
  /// In en, this message translates to:
  /// **'We couldn\'t find teammates this time. Please try again.'**
  String get queueSearchTimeoutBody;

  /// No description provided for @matchFoundTitle.
  ///
  /// In en, this message translates to:
  /// **'Match found'**
  String get matchFoundTitle;

  /// No description provided for @matchFoundSubtitle.
  ///
  /// In en, this message translates to:
  /// **'{gameName} · {mode}'**
  String matchFoundSubtitle(String gameName, String mode);

  /// No description provided for @matchFoundAccept.
  ///
  /// In en, this message translates to:
  /// **'Accept'**
  String get matchFoundAccept;

  /// No description provided for @matchFoundDecline.
  ///
  /// In en, this message translates to:
  /// **'Decline'**
  String get matchFoundDecline;

  /// No description provided for @matchFoundRespondError.
  ///
  /// In en, this message translates to:
  /// **'Could not respond to match'**
  String get matchFoundRespondError;

  /// No description provided for @matchSquadLeave.
  ///
  /// In en, this message translates to:
  /// **'Leave squad'**
  String get matchSquadLeave;

  /// No description provided for @matchSquadLeaveError.
  ///
  /// In en, this message translates to:
  /// **'Could not leave match squad'**
  String get matchSquadLeaveError;

  /// No description provided for @matchRatingTitle.
  ///
  /// In en, this message translates to:
  /// **'Rate your teammates'**
  String get matchRatingTitle;

  /// No description provided for @matchRatingSubtitle.
  ///
  /// In en, this message translates to:
  /// **'Stars are optional for each player.'**
  String get matchRatingSubtitle;

  /// No description provided for @matchRatingSkipTeammate.
  ///
  /// In en, this message translates to:
  /// **'Skip'**
  String get matchRatingSkipTeammate;

  /// No description provided for @matchRatingSkipAll.
  ///
  /// In en, this message translates to:
  /// **'Skip all'**
  String get matchRatingSkipAll;

  /// No description provided for @matchRatingSkipped.
  ///
  /// In en, this message translates to:
  /// **'Skipped'**
  String get matchRatingSkipped;

  /// No description provided for @matchRatingSubmit.
  ///
  /// In en, this message translates to:
  /// **'Submit ratings'**
  String get matchRatingSubmit;

  /// No description provided for @matchRatingBanTitle.
  ///
  /// In en, this message translates to:
  /// **'Ban from matchmaking?'**
  String get matchRatingBanTitle;

  /// No description provided for @matchRatingBanMessage.
  ///
  /// In en, this message translates to:
  /// **'Stop matching with {name} in matchmaking?'**
  String matchRatingBanMessage(String name);

  /// No description provided for @matchRatingBanCancel.
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get matchRatingBanCancel;

  /// No description provided for @matchRatingBanConfirm.
  ///
  /// In en, this message translates to:
  /// **'Ban'**
  String get matchRatingBanConfirm;

  /// No description provided for @matchRatingBanAction.
  ///
  /// In en, this message translates to:
  /// **'Ban from MM'**
  String get matchRatingBanAction;

  /// No description provided for @matchRatingSubmitError.
  ///
  /// In en, this message translates to:
  /// **'Could not submit rating'**
  String get matchRatingSubmitError;

  /// No description provided for @matchRatingBanError.
  ///
  /// In en, this message translates to:
  /// **'Could not ban from matchmaking'**
  String get matchRatingBanError;

  /// No description provided for @profileMmRating.
  ///
  /// In en, this message translates to:
  /// **'MM rating: {rating} ★'**
  String profileMmRating(String rating);

  /// No description provided for @matchHistoryTitle.
  ///
  /// In en, this message translates to:
  /// **'Match history'**
  String get matchHistoryTitle;

  /// No description provided for @matchHistoryEntry.
  ///
  /// In en, this message translates to:
  /// **'Match history'**
  String get matchHistoryEntry;

  /// No description provided for @matchHistoryLoadError.
  ///
  /// In en, this message translates to:
  /// **'Could not load match history'**
  String get matchHistoryLoadError;

  /// No description provided for @matchHistoryEmpty.
  ///
  /// In en, this message translates to:
  /// **'No match squads yet'**
  String get matchHistoryEmpty;

  /// No description provided for @matchHistoryParticipants.
  ///
  /// In en, this message translates to:
  /// **'Teammates'**
  String get matchHistoryParticipants;

  /// No description provided for @matchHistoryStatusActive.
  ///
  /// In en, this message translates to:
  /// **'Active'**
  String get matchHistoryStatusActive;

  /// No description provided for @matchHistoryStatusCompleted.
  ///
  /// In en, this message translates to:
  /// **'Completed'**
  String get matchHistoryStatusCompleted;

  /// No description provided for @matchHistoryLoadMore.
  ///
  /// In en, this message translates to:
  /// **'Load more'**
  String get matchHistoryLoadMore;

  /// No description provided for @matchHistoryAddFriend.
  ///
  /// In en, this message translates to:
  /// **'Add friend'**
  String get matchHistoryAddFriend;
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['en', 'ru'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'en':
      return AppLocalizationsEn();
    case 'ru':
      return AppLocalizationsRu();
  }

  throw FlutterError(
    'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}
