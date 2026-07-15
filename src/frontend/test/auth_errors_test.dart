import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/ui/auth/auth_errors.dart';

void main() {
  final l10n = AppLocalizationsEn();

  group('resolveAuthErrorKey', () {
    test('maps 429 to rate_limited when error body missing', () {
      expect(
        resolveAuthErrorKey(errorCode: null, statusCode: 429),
        AuthErrorKeys.rateLimited,
      );
    });

    test('prefers API error code', () {
      expect(
        resolveAuthErrorKey(errorCode: 'invalid_credentials', statusCode: 401),
        'invalid_credentials',
      );
    });

    test('maps grpc unauthenticated to message domain code', () {
      expect(
        resolveAuthErrorKey(
          errorCode: 'unauthenticated',
          statusCode: 401,
          message: 'invalid_credentials',
        ),
        'invalid_credentials',
      );
    });

    test('maps registration_conflict from grpc transcode', () {
      expect(
        resolveAuthErrorKey(
          errorCode: 'registration_conflict',
          statusCode: 400,
          message: 'registration_conflict',
        ),
        'registration_conflict',
      );
    });
  });

  group('authErrorMessage', () {
    test('validation_failed', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.validationFailed),
        l10n.authErrorValidationFailed,
      );
    });

    test('rate_limited', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.rateLimited),
        l10n.authErrorRateLimited,
      );
    });

    test('invalid_credentials', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.invalidCredentials),
        l10n.authErrorInvalidCredentials,
      );
    });

    test('registration_conflict', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.registrationConflict),
        l10n.authErrorRegistrationConflict,
      );
    });

    test('empty_fields', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.emptyFields),
        l10n.authErrorEmptyFields,
      );
    });

    test('password_too_short', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.passwordTooShort),
        l10n.authErrorPasswordTooShort,
      );
    });

    test('unknown key falls back to raw key', () {
      expect(authErrorMessage(l10n, 'some_unknown'), 'some_unknown');
    });
  });
}
