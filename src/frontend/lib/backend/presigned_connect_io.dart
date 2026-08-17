import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'gateway_api_error.dart';
import 'gateway_http.dart';

Future<GatewayHttpResult<void>> putBytesPreservingHost({
  required Uri connectUri,
  required String signedHostHeader,
  required Map<String, String> headers,
  required List<int> bytes,
}) async {
  final client = HttpClient();
  client.userAgent = null;
  try {
    final request = await client.openUrl('PUT', connectUri);
    request.followRedirects = false;
    request.contentLength = bytes.length;
    request.headers.set(HttpHeaders.hostHeader, signedHostHeader);
    headers.forEach((name, value) {
      final lower = name.toLowerCase();
      if (lower == 'host' || lower == 'content-length') {
        return;
      }
      request.headers.set(name, value);
    });
    request.add(bytes);
    final response = await request.close();
    final body = await utf8.decodeStream(response);
    if (response.statusCode >= 200 && response.statusCode < 300) {
      return const GatewayHttpOk(null);
    }
    return GatewayHttpFailure(
      GatewayApiError.fromStatusAndBody(response.statusCode, body),
    );
  } catch (e) {
    return GatewayHttpFailure(
      GatewayApiError(
        errorCode: 'network_error',
        message: '$e',
        statusCode: 0,
      ),
    );
  } finally {
    client.close(force: true);
  }
}

Future<GatewayHttpResult<Uint8List>> getBytesPreservingHost({
  required Uri connectUri,
  required String signedHostHeader,
}) async {
  final client = HttpClient();
  client.userAgent = null;
  try {
    final request = await client.openUrl('GET', connectUri);
    request.followRedirects = false;
    request.headers.set(HttpHeaders.hostHeader, signedHostHeader);
    final response = await request.close();
    final raw = await response.fold<List<int>>(
      <int>[],
      (prev, chunk) => prev..addAll(chunk),
    );
    final bytes = Uint8List.fromList(raw);
    if (response.statusCode >= 200 && response.statusCode < 300) {
      return GatewayHttpOk(bytes);
    }
    return GatewayHttpFailure(
      GatewayApiError.fromStatusAndBody(
        response.statusCode,
        utf8.decode(raw, allowMalformed: true),
      ),
    );
  } catch (e) {
    return GatewayHttpFailure(
      GatewayApiError(
        errorCode: 'network_error',
        message: '$e',
        statusCode: 0,
      ),
    );
  } finally {
    client.close(force: true);
  }
}
