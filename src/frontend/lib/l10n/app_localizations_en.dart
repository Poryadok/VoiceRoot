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
  String get socialTabSearch => 'Search';

  @override
  String get socialTabFriends => 'Friends';

  @override
  String get socialTabRequests => 'Requests';

  @override
  String get socialSearchHint => 'Search by name or @username';

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
  String get socialPresenceUnknown => 'Unknown';

  @override
  String socialActionError(String message) {
    return 'Error: $message';
  }
}
