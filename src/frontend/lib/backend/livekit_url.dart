/// Picks a browser-reachable LiveKit URL from API response and client fallback.
String resolveLivekitConnectUrl({
  required String? apiUrl,
  required String clientFallback,
}) {
  final fallback = clientFallback.trim();
  final fromApi = (apiUrl ?? '').trim();
  if (fromApi.isEmpty) {
    return fallback;
  }
  if (fallback.isNotEmpty && _isDockerInternalLivekitHost(fromApi)) {
    return fallback;
  }
  return fromApi;
}

bool _isDockerInternalLivekitHost(String url) {
  final host = Uri.tryParse(url)?.host.trim().toLowerCase() ?? '';
  if (host.isEmpty) {
    return false;
  }
  if (host == 'localhost' || host == '127.0.0.1' || host == '::1') {
    return false;
  }
  if (RegExp(r'^\d{1,3}(\.\d{1,3}){3}$').hasMatch(host)) {
    return false;
  }
  return !host.contains('.');
}
