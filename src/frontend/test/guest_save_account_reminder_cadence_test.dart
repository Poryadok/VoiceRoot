import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/state/guest_save_account_reminder.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  VoiceAuthClient authClient() {
    var claimed = false;
    return VoiceAuthClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          if (request.url.path == '/api/v1/auth/guest-reminder') {
            return http.Response('{"should_show": ${!claimed}}', 200);
          }
          if (request.url.path == '/api/v1/auth/guest-reminder/mark') {
            if (claimed) return http.Response('{}', 400);
            claimed = true;
            return http.Response('{}', 200);
          }
          return http.Response('not found', 404);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );
  }

  test(
    'first entry is quiet and a shown reminder stays suppressed on re-entry',
    () async {
      SharedPreferences.setMockInitialValues({});
      final guestStorage = InMemoryGuestCredentialsStorage();
      final client = authClient();
      const accountId = 'guest-acct-1';
      await guestStorage.markNicknameCompleted(accountId);
      final controller = GuestSaveAccountReminderController(
        guestStorage: guestStorage,
        authClient: client,
      );

      expect(await controller.shouldShow(accountId), isFalse);
      expect(
        await controller.showAndMark(accountId, authorization: 'Bearer guest'),
        isTrue,
      );
      expect(
        await controller.showAndMark(accountId, authorization: 'Bearer guest'),
        isFalse,
      );

      final reloaded = GuestSaveAccountReminderController(
        guestStorage: guestStorage,
        authClient: client,
      );
      expect(
        await reloaded.showAndMark(accountId, authorization: 'Bearer guest'),
        isFalse,
      );
    },
  );

  test('concurrent display checks claim only one reminder', () async {
    SharedPreferences.setMockInitialValues({
      'voice.auth.guest_reminder_first_entry.guest-acct-2': true,
    });
    final guestStorage = InMemoryGuestCredentialsStorage();
    final client = authClient();
    const accountId = 'guest-acct-2';
    await guestStorage.markNicknameCompleted(accountId);
    final controller = GuestSaveAccountReminderController(
      guestStorage: guestStorage,
      authClient: client,
    );

    final claims = await Future.wait([
      controller.showAndMark(accountId, authorization: 'Bearer guest'),
      controller.showAndMark(accountId, authorization: 'Bearer guest'),
    ]);
    expect(claims.where((claimed) => claimed), hasLength(1));
  });
}
