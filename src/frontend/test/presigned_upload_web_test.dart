@TestOn('browser')
library;

import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/presigned_upload.dart';

void main() {
  for (final host in ['minio', 'host.docker.internal', 'cdn.example']) {
    test('browser preserves signed $host URL for PUT and GET', () async {
      final signed = Uri.parse(
        'http://$host:9000/bucket/key%20name?X-Amz-Signature=fixture%2Bsignature',
      );
      final requests = <http.Request>[];
      final client = MockClient((request) async {
        requests.add(request);
        return http.Response.bytes([31, 42], 200);
      });
      addTearDown(client.close);
      final gateway = GatewayHttpClient(
        httpClient: client,
        config: const GatewayConfig(baseUrl: 'https://api.example'),
        authorizationProvider: () => 'Bearer must-stay-at-gateway',
      );

      final uploaded = await putPresigned(
        gateway: gateway,
        uploadUrl: signed,
        requiredHeaders: const {'Content-Type': 'application/octet-stream'},
        bytes: const [7, 8, 9],
      );
      expect(uploaded, isA<GatewayHttpOk<void>>());
      final downloaded = await getPresignedBytes(
        gateway: gateway,
        downloadUrl: signed,
        maxBytes: 32,
      );
      expect(downloaded, isA<GatewayHttpOk<Uint8List>>());
      expect((downloaded as GatewayHttpOk<Uint8List>).data, [31, 42]);
      expect(requests.map((request) => request.method), ['PUT', 'GET']);
      for (final request in requests) {
        expect(request.url.toString(), signed.toString());
        expect(request.headers.containsKey('host'), isFalse);
        expect(request.headers.containsKey('authorization'), isFalse);
      }
      expect(requests.first.bodyBytes, [7, 8, 9]);
      expect(
        requests.first.headers['content-type'],
        'application/octet-stream',
      );
    });
  }
}
