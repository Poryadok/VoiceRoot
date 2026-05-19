import 'dart:async';
import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';

import 'realtime_socket_connect.dart';

/// Builds Gateway `/ws` URI from REST base URL.
Uri gatewayWebSocketUri(String baseUrl) {
  final rest = Uri.parse(baseUrl);
  final scheme = switch (rest.scheme) {
    'https' => 'wss',
    'http' => 'ws',
    _ => rest.scheme,
  };
  return rest.replace(
    scheme: scheme,
    path: '/ws',
    query: null,
    fragment: null,
  );
}

/// Parsed Realtime WebSocket frame (server or client).
class RealtimeFrame {
  const RealtimeFrame({
    required this.op,
    this.data,
    this.sequence,
  });

  final String op;
  final Map<String, dynamic>? data;
  final int? sequence;
}

/// Pure protocol helpers for Realtime WS (testable without a live socket).
abstract final class RealtimeProtocol {
  static RealtimeFrame parseFrame(String raw) {
    final decoded = jsonDecode(raw);
    if (decoded is! Map<String, dynamic>) {
      return const RealtimeFrame(op: '');
    }
    final op = decoded['op'] as String? ?? '';
    final s = decoded['s'];
    Map<String, dynamic>? data;
    final d = decoded['d'];
    if (d is Map<String, dynamic>) {
      data = d;
    } else if (d is Map) {
      data = Map<String, dynamic>.from(d);
    }
    return RealtimeFrame(
      op: op,
      data: data,
      sequence: s is int ? s : (s is num ? s.toInt() : null),
    );
  }

  static int? trackSequence(int? current, int? incoming) {
    if (incoming == null) return current;
    return incoming;
  }

  static String buildClientOp(String op, Map<String, dynamic> data) {
    return jsonEncode({'op': op, 'd': data});
  }
}

/// Live WebSocket session to Realtime via API Gateway `/ws`.
class VoiceRealtimeConnection {
  VoiceRealtimeConnection({
    required Uri uri,
    required Map<String, String> headers,
    WebSocketChannel Function(Uri uri, {Map<String, String>? headers})?
        channelFactory,
  })  : _uri = uri,
        _headers = headers,
        _channelFactory = channelFactory ?? _defaultChannelFactory;

  final Uri _uri;
  final Map<String, String> _headers;
  final WebSocketChannel Function(Uri uri, {Map<String, String>? headers})
      _channelFactory;

  static WebSocketChannel _defaultChannelFactory(
    Uri uri, {
    Map<String, String>? headers,
  }) {
    return connectRealtimeSocket(uri, headers ?? const {});
  }

  final _events = StreamController<RealtimeFrame>.broadcast();
  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _subscription;
  Timer? _heartbeatTimer;
  int? _lastSequence;
  var _disposed = false;

  Stream<RealtimeFrame> get events => _events.stream;

  int? get lastSequence => _lastSequence;

  Future<void> connect() async {
    if (_disposed) return;
    await disconnect();
    _channel = _channelFactory(_uri, headers: _headers);
    _subscription = _channel!.stream.listen(
      _onMessage,
      onError: (Object e, StackTrace st) {
        if (!_events.isClosed) {
          _events.addError(e, st);
        }
      },
      onDone: () {
        if (!_events.isClosed) {
          _events.addError(StateError('websocket_closed'));
        }
      },
    );
    _heartbeatTimer?.cancel();
    _heartbeatTimer = Timer.periodic(
      const Duration(seconds: 30),
      (_) => sendHeartbeat(),
    );
  }

  void _onMessage(dynamic message) {
    if (message is! String) return;
    final frame = RealtimeProtocol.parseFrame(message);
    _lastSequence = RealtimeProtocol.trackSequence(_lastSequence, frame.sequence);
    if (!_events.isClosed) {
      _events.add(frame);
    }
  }

  void sendResume() {
    final last = _lastSequence;
    if (last == null) return;
    sendOp('resume', {'last_s': last});
  }

  void sendSubscribe(String chatId) {
    sendOp('subscribe', {'chat_id': chatId});
  }

  void sendMarkRead({required String chatId, required String messageId}) {
    sendOp('mark_read', {
      'chat_id': chatId,
      'message_id': messageId,
    });
  }

  void sendHeartbeat() {
    sendOp('heartbeat', {});
  }

  void sendOp(String op, Map<String, dynamic> data) {
    final ch = _channel;
    if (ch == null) return;
    ch.sink.add(RealtimeProtocol.buildClientOp(op, data));
  }

  Future<void> disconnect() async {
    _heartbeatTimer?.cancel();
    _heartbeatTimer = null;
    await _subscription?.cancel();
    _subscription = null;
    await _channel?.sink.close();
    _channel = null;
  }

  Future<void> dispose() async {
    _disposed = true;
    await disconnect();
    await _events.close();
  }
}
