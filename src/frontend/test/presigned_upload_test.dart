import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/presigned_upload.dart';

void main() {
  test('rewritePresignedUrlForHost maps compose MinIO hosts to localhost', () {
    expect(
      rewritePresignedUrlForHost(
        Uri.parse('http://host.docker.internal:9000/bucket/key'),
      ).toString(),
      'http://127.0.0.1:9000/bucket/key',
    );
    expect(
      rewritePresignedUrlForHost(
        Uri.parse('http://minio:9000/bucket/key'),
      ).toString(),
      'http://127.0.0.1:9000/bucket/key',
    );
    expect(
      rewritePresignedUrlForHost(
        Uri.parse('https://cdn.example/avatars/x.png'),
      ).toString(),
      'https://cdn.example/avatars/x.png',
    );
  });

  test('presignConnectTarget keeps signed Host when rewriting compose hosts', () {
    final signed = Uri.parse(
      'http://host.docker.internal:9000/voice-dev-files/key?X-Amz-Algorithm=AWS4',
    );
    final target = presignConnectTarget(signed);
    expect(target.connectUri.host, '127.0.0.1');
    expect(target.connectUri.port, 9000);
    expect(target.connectUri.path, '/voice-dev-files/key');
    expect(target.signedHostHeader, 'host.docker.internal:9000');
  });

  test('presignConnectTarget omits signed Host when URL is already reachable', () {
    final signed = Uri.parse('http://127.0.0.1:9000/bucket/key');
    final target = presignConnectTarget(signed);
    expect(target.connectUri.host, '127.0.0.1');
    expect(target.signedHostHeader, isNull);
  });

  test('putPresigned connects to localhost but sends signed Host', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    String? seenHost;
    String? seenContentType;
    final seenBody = <int>[];
    server.listen((req) async {
      seenHost = req.headers.value(HttpHeaders.hostHeader);
      seenContentType = req.headers.contentType?.mimeType;
      await for (final chunk in req) {
        seenBody.addAll(chunk);
      }
      req.response.statusCode = 200;
      await req.response.close();
    });

    final signed = Uri.parse(
      'http://host.docker.internal:${server.port}/voice-dev-files/key',
    );
    final httpClient = http.Client();
    addTearDown(httpClient.close);
    final gateway = GatewayHttpClient(
      httpClient: httpClient,
      config: const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
    );

    final result = await putPresigned(
      gateway: gateway,
      uploadUrl: signed,
      requiredHeaders: {'Content-Type': 'text/plain'},
      bytes: const [101, 50, 101, 10],
    );
    expect(result, isA<GatewayHttpOk<void>>());
    expect(seenHost, 'host.docker.internal:${server.port}');
    expect(seenContentType, 'text/plain');
    expect(seenBody, [101, 50, 101, 10]);
  });
}
