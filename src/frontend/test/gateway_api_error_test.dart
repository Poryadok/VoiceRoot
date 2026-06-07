import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/gateway_api_error.dart';

void main() {
  group('GatewayApiError.fromStatusAndBody', () {
    test('parses Auth proxy flat error', () {
      final err = GatewayApiError.fromStatusAndBody(
        401,
        '{"error":"invalid_credentials"}',
      );
      expect(err.errorCode, 'invalid_credentials');
      expect(err.message, 'invalid_credentials');
      expect(err.statusCode, 401);
    });

    test('parses gRPC transcode error_code + message', () {
      final err = GatewayApiError.fromStatusAndBody(
        403,
        '{"error_code":"permission_denied","message":"not allowed"}',
      );
      expect(err.errorCode, 'permission_denied');
      expect(err.message, 'not allowed');
    });

    test('parses 426 client_outdated with update_url', () {
      final err = GatewayApiError.fromStatusAndBody(
        426,
        '{"error_code":"client_outdated","message":"Update required","update_url":"https://store.example/app"}',
      );
      expect(err.errorCode, 'client_outdated');
      expect(err.updateUrl, 'https://store.example/app');
    });

    test('falls back to http status when body empty', () {
      final err = GatewayApiError.fromStatusAndBody(503, '');
      expect(err.errorCode, 'http_503');
      expect(err.statusCode, 503);
    });
  });

  group('GatewayApiError.fromResponse', () {
    test('returns null on success', () {
      expect(
        GatewayApiError.fromResponse(http.Response('ok', 200)),
        isNull,
      );
    });
  });
}
