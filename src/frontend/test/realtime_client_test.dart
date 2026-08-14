import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/realtime_client.dart';

void main() {
  test('realtimeWebSocketConnectUri adds ticket query param', () {
    final base = Uri.parse('ws://localhost:18080/ws');
    final withTicket = realtimeWebSocketConnectUri(
      base,
      wsTicket: 'opaque-ticket',
    );
    expect(withTicket.queryParameters['ticket'], 'opaque-ticket');
    expect(withTicket.queryParameters.containsKey('access_token'), isFalse);
  });

  test('realtimeWebSocketConnectUri without ticket leaves query empty', () {
    final base = Uri.parse('ws://localhost:18080/ws');
    final plain = realtimeWebSocketConnectUri(base);
    expect(plain.query, isEmpty);
  });
}
