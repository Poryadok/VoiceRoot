import 'dart:convert';

import 'package:http/http.dart' as http;

import 'gateway_config.dart';

/// Environment-owned public configuration, fetched before any app/API client.
/// This release path intentionally has no compile-time or hostname fallback.
Future<GatewayConfig> loadWebGatewayConfig({
  required Uri appUri,
  required http.Client client,
}) async {
  final origin = _origin(appUri.origin, 'https', 'http');
  final request = http.Request(
    'GET',
    origin.resolve('/voice-runtime-config.json'),
  )..followRedirects = false;
  final response = await client
      .send(request)
      .timeout(const Duration(seconds: 10));
  if (response.statusCode != 200) {
    throw const FormatException('Web configuration unavailable');
  }
  final bytes = <int>[];
  await for (final chunk in response.stream.timeout(
    const Duration(seconds: 10),
  )) {
    bytes.addAll(chunk);
    if (bytes.length > 8192) {
      throw const FormatException('Web configuration too large');
    }
  }
  final value = jsonDecode(utf8.decode(bytes));
  const keys = {'version', 'app_origin', 'api_base_url', 'livekit_url'};
  if (value is! Map<String, dynamic> ||
      value.keys.toSet().difference(keys).isNotEmpty ||
      value.length != keys.length ||
      value['version'] is! int ||
      value['version'] != 1) {
    throw const FormatException('Invalid web configuration');
  }
  final binding = _origin(value['app_origin'], 'https', 'http');
  if (binding != origin) {
    throw const FormatException('Web configuration origin mismatch');
  }
  return GatewayConfig(
    baseUrl: _origin(value['api_base_url'], 'https', 'http').toString(),
    livekitUrl: _origin(value['livekit_url'], 'wss', 'ws').toString(),
  );
}

Uri _origin(Object? value, String secureScheme, String loopbackScheme) {
  if (value is! String || value.isEmpty || value.trim() != value) {
    throw const FormatException('Invalid configuration origin');
  }
  final uri = Uri.tryParse(value);
  if (uri == null ||
      !uri.hasAuthority ||
      uri.host.isEmpty ||
      uri.userInfo.isNotEmpty ||
      uri.path.isNotEmpty ||
      uri.hasQuery ||
      uri.hasFragment ||
      (uri.scheme != secureScheme &&
          !(uri.scheme == loopbackScheme &&
              const {'localhost', '127.0.0.1', '::1'}.contains(uri.host)))) {
    throw const FormatException('Invalid configuration origin');
  }
  return uri;
}
