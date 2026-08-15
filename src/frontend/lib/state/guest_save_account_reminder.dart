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
final guestSaveAccountReminderVisibleProvider = FutureProvider<bool>((ref) async {
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
      .shouldShow(accountId, authorization: auth.session!.authorizationHeader);
});

class GuestSaveAccountReminderController {
  GuestSaveAccountReminderController({
    required GuestCredentialsStorage guestStorage,
    VoiceAuthClient? authClient,
    SharedPreferences? prefs,
  }) : _guestStorage = guestStorage,
       _authClient = authClient,
       _prefs = prefs;

  final GuestCredentialsStorage _guestStorage;
  final VoiceAuthClient? _authClient;
  SharedPreferences? _prefs;

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

    final server = await _serverShouldShow(authorization);
    if (server != null) {
      return server;
    }

    final lastMs = prefs.getInt('$_lastShownKeyPrefix$accountId');
    if (lastMs == null) return true;
    final last = DateTime.fromMillisecondsSinceEpoch(lastMs);
    return DateTime.now().difference(last).inHours >= 24;
  }

  Future<bool?> _serverShouldShow(String? authorization) async {
    final client = _authClient;
    if (client == null || authorization == null || authorization.isEmpty) {
      return null;
    }
    try {
      return await client.getGuestReminderShouldShow(authorization: authorization);
    } catch (_) {
      return null;
    }
  }

  Future<void> markShown(String accountId, {String? authorization}) async {
    final prefs = await _preferences();
    await prefs.setInt(
      '$_lastShownKeyPrefix$accountId',
      DateTime.now().millisecondsSinceEpoch,
    );
    final client = _authClient;
    if (client == null || authorization == null || authorization.isEmpty) {
      return;
    }
    try {
      await client.markGuestReminderShown(authorization: authorization);
    } catch (_) {
      // Keep local mark even if server mark fails.
    }
  }
}
