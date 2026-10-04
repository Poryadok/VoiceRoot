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

  test('active-session load mapping never exposes upstream details', () {
    const raw = 'internal trace id=secret-session-123';

    final mapped = activeSessionsLoadErrorMessage(l10n, raw);

    expect(mapped, l10n.securitySessionsLoadError);
    expect(mapped, isNot(contains('secret-session-123')));
  });

  test(
    'action failures use safe localized fallbacks and known unavailable map',
    () {
      expect(
        activeSessionRevokeErrorMessage(l10n),
        l10n.securitySessionRevokeError,
      );
      expect(
        activeSessionRevokeErrorMessage(l10n, statusCode: 503),
        l10n.backendUnavailable,
      );
      expect(botInstallActionErrorMessage(l10n), l10n.botInstallActionError);
      expect(
        botInstallActionErrorMessage(l10n, statusCode: 504),
        l10n.backendUnavailable,
      );
    },
  );

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

  test('social and chat actions never expose upstream failure details', () {
    const raw = 'database password=private-trace-123';

    expect(socialActionErrorMessage(l10n, raw), l10n.commonActionFailed);
    expect(chatActionErrorMessage(l10n, raw), l10n.commonActionFailed);
    expect(socialActionErrorMessage(l10n, raw), isNot(contains('private')));
    expect(chatActionErrorMessage(l10n, raw), isNot(contains('private')));
    expect(
      commonActionErrorMessage(l10n, statusCode: 503),
      l10n.backendUnavailable,
    );
    expect(commonActionErrorMessage(l10n), l10n.commonActionFailed);
  });

  test('social search has its own safe fallback and preserves 503 mapping', () {
    expect(socialSearchErrorMessage(l10n), l10n.socialSearchFailed);
    expect(
      socialSearchErrorMessage(l10n, statusCode: 503),
      l10n.backendUnavailable,
    );
  });
}
