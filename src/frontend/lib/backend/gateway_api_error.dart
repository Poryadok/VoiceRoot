import 'dart:convert';

import 'package:http/http.dart' as http;

/// Parsed Gateway error body (proxy `error` or gRPC `error_code` + `message`).
class GatewayApiError {
  const GatewayApiError({
    required this.errorCode,
    required this.message,
    required this.statusCode,
    this.updateUrl,
  });

  final String errorCode;
  final String message;
  final int statusCode;
  final String? updateUrl;

  static GatewayApiError? fromResponse(http.Response response) {
    if (response.statusCode >= 200 && response.statusCode < 300) {
      return null;
    }
    return fromStatusAndBody(response.statusCode, response.body);
  }

  static GatewayApiError fromStatusAndBody(int statusCode, String body) {
    String? errorCode;
    String? message;
    String? updateUrl;
    if (body.isNotEmpty) {
      try {
        final decoded = jsonDecode(body);
        if (decoded is Map<String, dynamic>) {
          final err = decoded['error'] ?? decoded['error_code'];
          if (err is String && err.isNotEmpty) {
            errorCode = err;
          }
          final msg = decoded['message'];
          if (msg is String && msg.isNotEmpty) {
            message = msg;
          }
          final url = decoded['update_url'];
          if (url is String && url.isNotEmpty) {
            updateUrl = url;
          }
        }
      } catch (_) {
        // ignore malformed body
      }
    }
    errorCode ??= 'http_$statusCode';
    message ??= errorCode;
    return GatewayApiError(
      errorCode: errorCode,
      message: message,
      statusCode: statusCode,
      updateUrl: updateUrl,
    );
  }
}
