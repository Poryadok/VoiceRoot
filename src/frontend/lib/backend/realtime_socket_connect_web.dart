import 'package:web_socket_channel/web_socket_channel.dart';

/// Browser WebSocket cannot set Authorization; token is passed as query param.
WebSocketChannel connectRealtimeSocket(
  Uri uri,
  Map<String, String> headers,
) {
  final auth = headers['Authorization'] ?? '';
  final token =
      auth.startsWith('Bearer ') ? auth.substring(7).trim() : auth.trim();
  final target = token.isEmpty
      ? uri
      : uri.replace(
          queryParameters: {
            ...uri.queryParameters,
            'access_token': token,
          },
        );
  return WebSocketChannel.connect(target);
}
