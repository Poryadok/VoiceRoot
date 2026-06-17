import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../backend/guest_credentials_storage.dart';
import 'auth_providers.dart';

final guestSaveAccountReminderProvider =
    Provider<GuestSaveAccountReminderController>((ref) {
      return GuestSaveAccountReminderController(
        guestStorage: ref.watch(guestCredentialsStorageProvider),
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
  return ref.read(guestSaveAccountReminderProvider).shouldShow(accountId);
});

class GuestSaveAccountReminderController {
  GuestSaveAccountReminderController({
    required GuestCredentialsStorage guestStorage,
    SharedPreferences? prefs,
  }) : _guestStorage = guestStorage,
       _prefs = prefs;

  final GuestCredentialsStorage _guestStorage;
  SharedPreferences? _prefs;

  static const _lastShownKeyPrefix = 'voice.auth.guest_reminder_shown.';

  Future<SharedPreferences> _preferences() async {
    return _prefs ??= await SharedPreferences.getInstance();
  }

  Future<bool> shouldShow(String accountId) async {
    if (!await _guestStorage.isNicknameCompleted(accountId)) {
      return true;
    }
    final prefs = await _preferences();
    final lastMs = prefs.getInt('$_lastShownKeyPrefix$accountId');
    if (lastMs == null) return true;
    final last = DateTime.fromMillisecondsSinceEpoch(lastMs);
    return DateTime.now().difference(last).inHours >= 24;
  }

  Future<void> markShown(String accountId) async {
    final prefs = await _preferences();
    await prefs.setInt(
      '$_lastShownKeyPrefix$accountId',
      DateTime.now().millisecondsSinceEpoch,
    );
  }
}
