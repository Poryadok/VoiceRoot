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
      GatewayApiError(errorCode: 'network_error', message: '$e', statusCode: 0),
    );
  } finally {
    client.close(force: true);
  }
}

Future<GatewayHttpResult<Uint8List>> getBytesPreservingHost({
  required Uri connectUri,
  required String signedHostHeader,
  int? maxBytes,
}) async {
  if (maxBytes != null && (maxBytes <= 0 || maxBytes > 10 * 1024 * 1024)) {
    return const GatewayHttpFailure(
      GatewayApiError(
        errorCode: 'download_size_invalid',
        message: 'Invalid download limit',
        statusCode: 0,
      ),
    );
  }
  final client = HttpClient();
  client.userAgent = null;
  if (maxBytes != null) client.connectionTimeout = const Duration(seconds: 10);
  try {
    final opening = client.openUrl('GET', connectUri);
    final request = await (maxBytes == null
        ? opening
        : opening.timeout(const Duration(seconds: 10)));
    request.followRedirects = false;
    request.headers.set(HttpHeaders.hostHeader, signedHostHeader);
    final receiving = request.close();
    final response = await (maxBytes == null
        ? receiving
        : receiving.timeout(const Duration(seconds: 10)));
    if (maxBytes != null) {
      if (response.contentLength > maxBytes) {
        return const GatewayHttpFailure(
          GatewayApiError(
            errorCode: 'download_too_large',
            message: 'Download exceeds limit',
            statusCode: 0,
          ),
        );
      }
      if (response.statusCode < 200 || response.statusCode >= 300) {
        return GatewayHttpFailure(
          GatewayApiError(
            errorCode: 'download_failed',
            message: 'Download failed',
            statusCode: response.statusCode,
          ),
        );
      }
      final bytes = BytesBuilder(copy: false);
      final elapsed = Stopwatch()..start();
      await for (final chunk in response.timeout(const Duration(seconds: 10))) {
        if (elapsed.elapsed > const Duration(seconds: 30)) {
          return const GatewayHttpFailure(
            GatewayApiError(
              errorCode: 'download_timeout',
              message: 'Download timed out',
              statusCode: 0,
            ),
          );
        }
        if (bytes.length + chunk.length > maxBytes) {
          return const GatewayHttpFailure(
            GatewayApiError(
              errorCode: 'download_too_large',
              message: 'Download exceeds limit',
              statusCode: 0,
            ),
          );
        }
        bytes.add(chunk);
      }
      if ((response.headers.value(HttpHeaders.contentEncodingHeader) ??
                  'identity') ==
              'identity' &&
          response.contentLength >= 0 &&
          response.contentLength != bytes.length) {
        return const GatewayHttpFailure(
          GatewayApiError(
            errorCode: 'download_incomplete',
            message: 'Incomplete download',
            statusCode: 0,
          ),
        );
      }
      return GatewayHttpOk(bytes.takeBytes());
    }
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
        message: maxBytes == null ? '$e' : 'Download unavailable',
        statusCode: 0,
      ),
    );
  } finally {
    client.close(force: true);
  }
}
