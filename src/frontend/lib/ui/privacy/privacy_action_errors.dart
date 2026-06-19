import '../../l10n/app_localizations.dart';

/// Maps backend privacy denial messages to user-facing l10n strings.
String privacyActionErrorMessage(AppLocalizations l10n, String raw) {
  final lower = raw.toLowerCase();
  if (lower.contains('call blocked by recipient privacy')) {
    return l10n.privacyDeniedCall;
  }
  if (lower.contains('invite blocked by recipient privacy')) {
    return l10n.privacyDeniedInvite;
  }
  if (lower.contains('file attachment blocked by recipient privacy')) {
    return l10n.privacyDeniedFile;
  }
  if (lower.contains('voice message blocked by recipient privacy')) {
    return l10n.privacyDeniedVoice;
  }
  if (lower.contains('dm blocked by recipient privacy')) {
    return l10n.privacyDeniedDm;
  }
  return raw;
}
