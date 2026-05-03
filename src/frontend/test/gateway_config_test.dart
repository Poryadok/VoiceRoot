import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

void main() {
  test('fromEnvironment default is empty base URL', () {
    expect(GatewayConfig.fromEnvironment().baseUrl, '');
  });

  test('explicit config hasBaseUrl', () {
    expect(
      const GatewayConfig(baseUrl: 'http://localhost:8080').hasBaseUrl,
      isTrue,
    );
  });
}
