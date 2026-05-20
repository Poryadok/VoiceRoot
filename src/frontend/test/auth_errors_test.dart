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
        resolveAuthErrorKey(
          errorCode: 'invalid_credentials',
          statusCode: 401,
        ),
        'invalid_credentials',
      );
    });
  });

  group('authErrorMessage', () {
    test('validation_failed', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.validationFailed),
        'Use a valid email and a password of at least 8 characters.',
      );
    });

    test('rate_limited', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.rateLimited),
        'Too many attempts. Please wait and try again.',
      );
    });

    test('invalid_credentials', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.invalidCredentials),
        'Incorrect email or password.',
      );
    });

    test('empty_fields', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.emptyFields),
        'Enter your email and password.',
      );
    });

    test('password_too_short', () {
      expect(
        authErrorMessage(l10n, AuthErrorKeys.passwordTooShort),
        'Password must be at least 8 characters.',
      );
    });

    test('unknown key falls back to raw key', () {
      expect(authErrorMessage(l10n, 'some_unknown'), 'some_unknown');
    });
  });
}
