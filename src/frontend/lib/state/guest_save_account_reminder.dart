import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../backend/auth_client.dart';
import '../backend/guest_credentials_storage.dart';
import 'auth_providers.dart';

final guestSaveAccountReminderProvider =
    Provider<GuestSaveAccountReminderController>((ref) {
      return GuestSaveAccountReminderController(
        guestStorage: ref.watch(guestCredentialsStorageProvider),
        authClient: ref.watch(voiceAuthClientProvider),
      );
    });

/// Whether the save-account banner should show for a returning guest (max 1×/day).
final guestSaveAccountReminderVisibleProvider = FutureProvider<bool>((
  ref,
) async {
  final auth = ref.watch(authControllerProvider);
  if (!auth.isGuest || auth.needsGuestNickname || auth.session == null) {
    return false;
  }
  final accountId = auth.session!.accountId;
  if (!await ref
      .read(guestCredentialsStorageProvider)
      .isNicknameCompleted(accountId)) {
    return false;
  }
  return ref
      .read(guestSaveAccountReminderProvider)
      .showAndMark(accountId, authorization: auth.session!.authorizationHeader);
});

class GuestSaveAccountReminderController {
  GuestSaveAccountReminderController({
    required GuestCredentialsStorage guestStorage,
    VoiceAuthClient? authClient,
    SharedPreferences? prefs,
    Future<void> Function(String accountId, int shownAtMillis)?
    persistLastShown,
  }) : _guestStorage = guestStorage,
       _authClient = authClient,
       _prefs = prefs,
       _persistLastShown = persistLastShown;

  final GuestCredentialsStorage _guestStorage;
  final VoiceAuthClient? _authClient;
  SharedPreferences? _prefs;
  final Future<void> Function(String accountId, int shownAtMillis)?
  _persistLastShown;

  static const _lastShownKeyPrefix = 'voice.auth.guest_reminder_shown.';
  static const _firstEntryDonePrefix = 'voice.auth.guest_reminder_first_entry.';

  Future<SharedPreferences> _preferences() async {
    return _prefs ??= await SharedPreferences.getInstance();
  }

  Future<bool> shouldShow(String accountId, {String? authorization}) async {
    if (!await _guestStorage.isNicknameCompleted(accountId)) {
      return false;
    }
    final prefs = await _preferences();
    final firstEntryKey = '$_firstEntryDonePrefix$accountId';
    if (!(prefs.getBool(firstEntryKey) ?? false)) {
      await prefs.setBool(firstEntryKey, true);
      return false;
    }

    final lastMs = prefs.getInt('$_lastShownKeyPrefix$accountId');
    if (lastMs != null) {
      final last = DateTime.fromMillisecondsSinceEpoch(lastMs);
      if (DateTime.now().difference(last) < const Duration(hours: 24)) {
        return false;
      }
    }
    return await _serverShouldShow(authorization) ?? true;
  }

  /// Claims the display before the banner is exposed so re-entry cannot repeat it.
  Future<bool> showAndMark(String accountId, {String? authorization}) async {
    if (!await shouldShow(accountId, authorization: authorization)) {
      return false;
    }
    return markShown(accountId, authorization: authorization);
  }

  Future<bool?> _serverShouldShow(String? authorization) async {
    final client = _authClient;
    if (client == null || authorization == null || authorization.isEmpty) {
      return null;
    }
    try {
      return await client.getGuestReminderShouldShow(
        authorization: authorization,
      );
    } catch (_) {
      return null;
    }
  }

  Future<bool> markShown(String accountId, {String? authorization}) async {
    final client = _authClient;
    if (client == null || authorization == null || authorization.isEmpty) {
      return false;
    }
    try {
      if (!await client.markGuestReminderShown(authorization: authorization)) {
        return false;
      }
    } catch (_) {
      return false;
    }
    try {
      final shownAtMillis = DateTime.now().millisecondsSinceEpoch;
      final persistLastShown = _persistLastShown;
      if (persistLastShown != null) {
        await persistLastShown(accountId, shownAtMillis);
      } else {
        final prefs = await _preferences();
        await prefs.setInt('$_lastShownKeyPrefix$accountId', shownAtMillis);
      }
    } catch (_) {
      // The server owns the claim; local persistence is only a fast path.
    }
    return true;
  }
}
