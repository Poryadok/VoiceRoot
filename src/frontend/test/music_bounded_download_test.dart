import 'dart:async';
import 'dart:typed_data';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';

class _StreamingClient extends http.BaseClient {
  _StreamingClient(this.response);
  final http.StreamedResponse Function() response;
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    expect(request.headers.containsKey('authorization'), isFalse);
    expect(request.followRedirects, isFalse);
    return response();
  }
}

class _LateHeadersClient extends http.BaseClient {
  final headers = Completer<http.StreamedResponse>();
  bool aborted = false;
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) {
    (request as http.AbortableRequest).abortTrigger!.then(
      (_) => aborted = true,
    );
    return headers.future;
  }
}

void main() {
  test('header timeout aborts transport and cancels late response', () async {
    final transport = _LateHeadersClient();
    final client = GatewayHttpClient(
      httpClient: transport,
      config: const GatewayConfig(baseUrl: 'https://api.example'),
    );
    final result = client.getBytes(
      uri: Uri.parse('https://storage.example/signed'),
      maxBytes: 100,
    );
    // Future.timeout uses a real timer here; advancing a widget test's fake
    // clock leaves the request pending and makes this test hang.
    await Future<void>.delayed(const Duration(seconds: 31));
    expect(await result, isA<GatewayHttpFailure>());
    expect(transport.aborted, isTrue);
    var cancelled = false;
    final body = StreamController<List<int>>(onCancel: () => cancelled = true);
    transport.headers.complete(http.StreamedResponse(body.stream, 200));
    await Future<void>.delayed(Duration.zero);
    expect(cancelled, isTrue);
    await body.close();
  });
  GatewayHttpClient gateway(http.StreamedResponse Function() response) =>
      GatewayHttpClient(
        httpClient: _StreamingClient(response),
        config: const GatewayConfig(baseUrl: 'https://api.example'),
      );
  final uri = Uri.parse('https://storage.example/private?signature=secret');
  test('invalid bounds reject before sending a signed URL', () async {
    var sent = false;
    final client = gateway(() {
      sent = true;
      throw StateError('must not send');
    });
    for (final limit in [0, -1, 10 * 1024 * 1024 + 1]) {
      final result = await client.getBytes(uri: uri, maxBytes: limit);
      expect(result, isA<GatewayHttpFailure>());
    }
    expect(sent, isFalse);
  });
  test('bounded download returns only complete admitted bytes', () async {
    final result = await gateway(
      () => http.StreamedResponse(
        Stream.fromIterable([
          [1, 2],
          [3],
        ]),
        200,
        contentLength: 3,
      ),
    ).getBytes(uri: uri, maxBytes: 3);
    expect((result as GatewayHttpOk<Uint8List>).data, [1, 2, 3]);
  });
  test(
    'chunk overflow cancels consumption instead of buffering the object',
    () async {
      var read = 0;
      var cancelled = false;
      Stream<List<int>> body() async* {
        try {
          for (var i = 0; i < 100; i++) {
            read++;
            yield [1, 2];
          }
        } finally {
          cancelled = true;
        }
      }

      final result = await gateway(
        () => http.StreamedResponse(body(), 200),
      ).getBytes(uri: uri, maxBytes: 3);
      expect(result, isA<GatewayHttpFailure>());
      expect(read, 2);
      expect(cancelled, isTrue);
    },
  );
  test('partial and denied responses never become playable bytes', () async {
    for (final response in [
      () => http.StreamedResponse(Stream.value([1]), 200, contentLength: 3),
      () => http.StreamedResponse(Stream.value([1]), 403),
      () => http.StreamedResponse(Stream.value([1]), 302),
    ]) {
      final result = await gateway(response).getBytes(uri: uri, maxBytes: 3);
      expect(result, isA<GatewayHttpFailure>());
    }
  });
}
