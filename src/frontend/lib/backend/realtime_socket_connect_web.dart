import 'package:web_socket_channel/web_socket_channel.dart';

/// Browser WebSocket cannot set Authorization; use short-lived ticket from
/// POST /api/v1/realtime/ws-ticket (see docs/ARCHITECTURE_REQUIREMENTS.md).
WebSocketChannel connectRealtimeSocket(Uri uri, Map<String, String> headers) {
  return WebSocketChannel.connect(uri);
}
