import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

void main() {
  test('effectiveLivekitFallback uses localhost default for local gateway', () {
    const config = GatewayConfig(baseUrl: 'http://127.0.0.1:18080');
    expect(config.effectiveLivekitFallback, 'ws://127.0.0.1:7880');
    expect(config.canPlaceVoiceCalls, isTrue);
  });

  test('effectiveLivekitFallback prefers compile-time livekit URL', () {
    const config = GatewayConfig(
      baseUrl: 'http://127.0.0.1:18080',
      livekitUrl: 'wss://livekit.example.com',
    );
    expect(config.effectiveLivekitFallback, 'wss://livekit.example.com');
  });
}
