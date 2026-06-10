import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';

/// JSON mock responses with UTF-8 bodies (required when payload contains emoji).
http.Response utf8JsonResponse(String body, {int status = 200}) {
  return http.Response.bytes(
    utf8.encode(body),
    status,
    headers: const {'content-type': 'application/json; charset=utf-8'},
  );
}

GatewayHttpClient gatewayHttpForTest(
  http.Client httpClient, {
  GatewayConfig config = const GatewayConfig(baseUrl: 'http://api.test'),
}) {
  return GatewayHttpClient(httpClient: httpClient, config: config);
}
