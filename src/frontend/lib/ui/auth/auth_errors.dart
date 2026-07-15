import '../../l10n/app_localizations.dart';

/// Client-side and API error keys for auth UI.
abstract final class AuthErrorKeys {
  static const emptyFields = 'empty_fields';
  static const passwordTooShort = 'password_too_short';
  static const validationFailed = 'validation_failed';
  static const rateLimited = 'rate_limited';
  static const invalidCredentials = 'invalid_credentials';
  static const registrationConflict = 'registration_conflict';
  static const totpRequired = 'totp_required';
  static const invalidTotp = 'invalid_totp';
}

const _knownAuthApiCodes = <String>{
  AuthErrorKeys.validationFailed,
  AuthErrorKeys.invalidCredentials,
  AuthErrorKeys.registrationConflict,
  AuthErrorKeys.totpRequired,
  AuthErrorKeys.invalidTotp,
  'invalid_token',
  'token_revoked',
  'token_expired',
  'not_found',
  'auth_unavailable',
};

/// Normalizes gateway/auth HTTP failures to an [AuthErrorKeys] value when possible.
String? resolveAuthErrorKey({
  String? errorCode,
  int? statusCode,
  String? message,
}) {
  if (statusCode == 429) return AuthErrorKeys.rateLimited;
  final normalizedMessage = message?.trim();
  if (errorCode == 'unauthenticated' &&
      normalizedMessage != null &&
      normalizedMessage.isNotEmpty &&
      _knownAuthApiCodes.contains(normalizedMessage)) {
    return normalizedMessage;
  }
  if (errorCode != null && errorCode.isNotEmpty) return errorCode;
  if (normalizedMessage != null &&
      normalizedMessage.isNotEmpty &&
      _knownAuthApiCodes.contains(normalizedMessage)) {
    return normalizedMessage;
  }
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
    case AuthErrorKeys.registrationConflict:
      return l10n.authErrorRegistrationConflict;
    case AuthErrorKeys.totpRequired:
      return l10n.authErrorTotpRequired;
    case AuthErrorKeys.invalidTotp:
      return l10n.authErrorInvalidTotp;
    default:
      return key;
  }
}
