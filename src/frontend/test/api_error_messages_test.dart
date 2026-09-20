import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/ui/api_error_messages.dart';

void main() {
  final l10n = AppLocalizationsEn();

  test('chat room error mapping never exposes unknown upstream text', () {
    final mapped = chatRoomErrorMessage(l10n, 'internal trace id=secret-123');

    expect(mapped, l10n.chatRoomLoadError);
    expect(mapped, isNot(contains('secret-123')));
  });

  test('chat room error mapping renders permission denial locally', () {
    expect(
      chatRoomErrorMessage(l10n, 'permission_denied'),
      l10n.chatRoomPermissionDenied,
    );
  });

  test(
    'chat room error mapping retains known privacy and unavailable states',
    () {
      expect(
        chatRoomErrorMessage(l10n, 'dm blocked by recipient privacy settings'),
        l10n.chatRoomError(l10n.privacyDeniedDm),
      );
      expect(
        chatRoomErrorMessage(l10n, 'ignored', statusCode: 503),
        l10n.backendUnavailable,
      );
    },
  );
}
