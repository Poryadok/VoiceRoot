import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/state/guest_save_account_reminder.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('first entry after nickname suppresses banner; second shows', () async {
    SharedPreferences.setMockInitialValues({});
    final guestStorage = InMemoryGuestCredentialsStorage();
    const accountId = 'guest-acct-1';
    await guestStorage.markNicknameCompleted(accountId);
    final controller = GuestSaveAccountReminderController(
      guestStorage: guestStorage,
    );

    expect(await controller.shouldShow(accountId), isFalse);
    expect(await controller.shouldShow(accountId), isTrue);

    await controller.markShown(accountId);
    expect(await controller.shouldShow(accountId), isFalse);
  });
}
