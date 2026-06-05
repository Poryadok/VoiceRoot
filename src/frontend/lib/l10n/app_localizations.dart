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
