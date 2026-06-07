import '../../l10n/app_localizations.dart';

/// Client-side and API error keys for auth UI.
abstract final class AuthErrorKeys {
  static const emptyFields = 'empty_fields';
  static const passwordTooShort = 'password_too_short';
  static const validationFailed = 'validation_failed';
  static const rateLimited = 'rate_limited';
  static const invalidCredentials = 'invalid_credentials';
}

/// Normalizes gateway/auth HTTP failures to an [AuthErrorKeys] value when possible.
String? resolveAuthErrorKey({String? errorCode, int? statusCode}) {
  if (statusCode == 429) return AuthErrorKeys.rateLimited;
  if (errorCode != null && errorCode.isNotEmpty) return errorCode;
  return null;
}

/// Localized message for [key] (API code or [AuthErrorKeys] constant).
String authErrorMessage(AppLocalizations l10n, String key) {
  switch (key) {
    case AuthErrorKeys.emptyFields:
      return l10n.authErrorEmptyFields;
    case AuthErrorKeys.passwordTooShort:
      return l10n.authErrorPasswordTooShort;
    case AuthErrorKeys.validationFailed:
      return l10n.authErrorValidationFailed;
    case AuthErrorKeys.rateLimited:
      return l10n.authErrorRateLimited;
    case AuthErrorKeys.invalidCredentials:
      return l10n.authErrorInvalidCredentials;
    default:
      return key;
  }
}
