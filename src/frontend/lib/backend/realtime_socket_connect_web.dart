import 'package:web_socket_channel/web_socket_channel.dart';

/// Browser WebSocket cannot set Authorization; token is passed as query param.
/// Gateway copies `access_token` into Authorization for Realtime upstream.
/// Risk: token may appear in proxy logs — see docs/ARCHITECTURE_REQUIREMENTS.md.
WebSocketChannel connectRealtimeSocket(Uri uri, Map<String, String> headers) {
  final auth = headers['Authorization'] ?? '';
  final token = auth.startsWith('Bearer ')
      ? auth.substring(7).trim()
      : auth.trim();
  final target = token.isEmpty
      ? uri
      : uri.replace(
          queryParameters: {...uri.queryParameters, 'access_token': token},
        );
  return WebSocketChannel.connect(target);
}
