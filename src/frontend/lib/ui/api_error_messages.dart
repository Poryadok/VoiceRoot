import '../backend/api_errors.dart' show BackendUnavailableException, ProfileUnavailableException, isBackendUnavailable;
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

String chatActionErrorMessage(
  AppLocalizations l10n,
  String message, {
  int? statusCode,
}) {
  if (isBackendUnavailable(statusCode)) {
    return l10n.backendUnavailable;
  }
  return l10n.chatForwardError(message);
}

String spaceRolesErrorMessage(AppLocalizations l10n, Object error) {
  if (error is BackendUnavailableException) {
    return l10n.backendUnavailable;
  }
  return l10n.spaceRolesLoadError;
}

String spaceTreeErrorMessage(AppLocalizations l10n, Object error) {
  if (error is BackendUnavailableException) {
    return l10n.backendUnavailable;
  }
  return l10n.spaceTreeLoadError;
}

String gameCatalogErrorMessage(AppLocalizations l10n, Object error) {
  if (error is BackendUnavailableException) {
    return l10n.backendUnavailable;
  }
  return l10n.gameCatalogLoadError;
}

String storyArchiveErrorMessage(AppLocalizations l10n, Object error) {
  if (error is BackendUnavailableException) {
    return l10n.backendUnavailable;
  }
  return l10n.storyArchiveLoadError;
}

String storyViewerErrorMessage(AppLocalizations l10n, Object error) {
  if (error is BackendUnavailableException) {
    return l10n.backendUnavailable;
  }
  return l10n.storyViewerLoadError;
}

String storyViewersErrorMessage(AppLocalizations l10n, Object error) {
  if (error is BackendUnavailableException) {
    return l10n.backendUnavailable;
  }
  return l10n.storyViewersLoadError;
}

String chatThreadErrorMessage(
  AppLocalizations l10n,
  Object error, {
  int? statusCode,
}) {
  if (error is BackendUnavailableException ||
      isBackendUnavailable(statusCode)) {
    return l10n.backendUnavailable;
  }
  if (error is String && error.isNotEmpty) {
    return error;
  }
  return l10n.chatThreadLoadError;
}

String socialProfileErrorMessage(AppLocalizations l10n, Object error) {
  if (error is ProfileUnavailableException) {
    return l10n.socialProfileUnavailable;
  }
  if (error is BackendUnavailableException) {
    return l10n.backendUnavailable;
  }
  return l10n.socialProfileLoadError;
}
