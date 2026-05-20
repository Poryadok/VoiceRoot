/// HTTP status codes where Phase-1 social/chat routes are missing or down.
bool isBackendUnavailable(int? statusCode) {
  return statusCode == 404 || statusCode == 503;
}

/// Thrown by Riverpod loaders when upstream social/chat APIs are unavailable.
class BackendUnavailableException implements Exception {
  const BackendUnavailableException();
}
