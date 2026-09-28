import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/state/guest_save_account_reminder.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'first entry is quiet and a shown reminder stays suppressed on re-entry',
    () async {
      SharedPreferences.setMockInitialValues({});
      final guestStorage = InMemoryGuestCredentialsStorage();
      const accountId = 'guest-acct-1';
      await guestStorage.markNicknameCompleted(accountId);
      final controller = GuestSaveAccountReminderController(
        guestStorage: guestStorage,
      );

      expect(await controller.shouldShow(accountId), isFalse);
      expect(await controller.showAndMark(accountId), isTrue);
      expect(await controller.showAndMark(accountId), isFalse);

      final reloaded = GuestSaveAccountReminderController(
        guestStorage: guestStorage,
      );
      expect(await reloaded.showAndMark(accountId), isFalse);
    },
  );
}
