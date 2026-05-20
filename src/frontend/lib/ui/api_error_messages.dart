import '../backend/api_errors.dart';
import '../l10n/app_localizations.dart';

String socialListErrorMessage(AppLocalizations l10n, Object error) {
  if (error is BackendUnavailableException) {
    return l10n.socialFriendsBackendUnavailable;
  }
  return l10n.socialFriendsLoadError;
}

String socialRequestsErrorMessage(AppLocalizations l10n, Object error) {
  if (error is BackendUnavailableException) {
    return l10n.backendUnavailable;
  }
  return l10n.socialRequestsLoadError;
}

String chatListErrorMessage(AppLocalizations l10n, Object error) {
  if (error is BackendUnavailableException) {
    return l10n.chatListBackendUnavailable;
  }
  return l10n.chatListLoadError;
}

String socialActionErrorMessage(
  AppLocalizations l10n,
  String message, {
  int? statusCode,
}) {
  if (isBackendUnavailable(statusCode)) {
    return l10n.backendUnavailable;
  }
  return l10n.socialActionError(message);
}
