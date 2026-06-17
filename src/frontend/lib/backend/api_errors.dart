/// HTTP status codes where upstream services are down (not resource-not-found).
bool isBackendUnavailable(int? statusCode) {
  return statusCode == 502 || statusCode == 503 || statusCode == 504;
}

/// Maps gRPC-transcoded `not_found` to a dedicated UX path.
bool isNotFoundError(String? errorCode, int? statusCode) {
  return statusCode == 404 || errorCode == 'not_found';
}

/// Thrown by Riverpod loaders when upstream social/chat APIs are unavailable.
class BackendUnavailableException implements Exception {
  const BackendUnavailableException();
}

/// Profile hidden (blocked user or opaque not-found from privacy enforcement).
class ProfileUnavailableException implements Exception {
  const ProfileUnavailableException();
}
