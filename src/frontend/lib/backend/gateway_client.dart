import 'dart:convert';

import 'package:http/http.dart' as http;

import 'client_version.dart';
import 'gateway_config.dart';
import 'gateway_request_id.dart';

/// Detail string on [GatewayHealthFailure] when [GatewayConfig.hasBaseUrl] is false; used for i18n.
const String kGatewayMissingBaseUrlDetail = 'missing base URL';

sealed class GatewayHealthResult {
  const GatewayHealthResult();
}

final class GatewayHealthOk extends GatewayHealthResult {
  const GatewayHealthOk();
}

final class GatewayHealthFailure extends GatewayHealthResult {
  const GatewayHealthFailure(this.message);
  final String message;
}

/// Short-lived opaque ticket for browser WebSocket upgrade (`/ws?ticket=…`).
final class GatewayWsTicket {
  const GatewayWsTicket({required this.ticket, required this.expiresInSeconds});

  final String ticket;
  final int expiresInSeconds;
}

/// Minimal HTTP surface for API Gateway public `GET /health` / `GET /api/v1/version`.
class VoiceGatewayClient {
  VoiceGatewayClient({
    required http.Client httpClient,
    required GatewayConfig config,
  }) : _http = httpClient,
       _config = config;

  final http.Client _http;
  final GatewayConfig _config;

  Future<GatewayHealthResult> fetchHealth() async {
    if (!_config.hasBaseUrl) {
      return const GatewayHealthFailure(kGatewayMissingBaseUrlDetail);
    }
    final resolved = Uri.parse(_config.baseUrl).resolve('/health');
    try {
      final res = await _http.get(
        resolved,
        headers: {'X-Request-Id': newGatewayRequestId()},
      );
      if (res.statusCode == 200) {
        return const GatewayHealthOk();
      }
      return GatewayHealthFailure('HTTP ${res.statusCode}');
    } catch (e) {
      return GatewayHealthFailure('$e');
    }
  }

  /// `GET /api/v1/version` — public, no JWT.
  Future<String?> fetchVersionBody({
    required String platform,
    required String version,
  }) async {
    if (!_config.hasBaseUrl) return null;
    final uri = Uri.parse(_config.baseUrl).replace(
      path: '/api/v1/version',
      queryParameters: {'platform': platform, 'version': version},
    );
    try {
      final res = await _http.get(
        uri,
        headers: {
          ...ClientVersion.headers,
          'X-Request-Id': newGatewayRequestId(),
        },
      );
      if (res.statusCode == 200) return res.body;
      return null;
    } catch (_) {
      return null;
    }
  }

  /// `POST /api/v1/realtime/ws-ticket` — JWT in Authorization header only.
  Future<GatewayWsTicket?> requestWsTicket(String authorization) async {
    if (!_config.hasBaseUrl) return null;
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/realtime/ws-ticket');
    try {
      final res = await _http.post(
        uri,
        headers: {
          'Authorization': authorization,
          'Content-Type': 'application/json',
          'X-Request-Id': newGatewayRequestId(),
        },
      );
      if (res.statusCode != 200) return null;
      final decoded = jsonDecode(res.body);
      if (decoded is! Map<String, dynamic>) return null;
      final ticket = decoded['ticket'] as String?;
      final expires = decoded['expires_in_seconds'];
      if (ticket == null || ticket.isEmpty) return null;
      final expiresSec = expires is int
          ? expires
          : (expires is num ? expires.toInt() : 0);
      return GatewayWsTicket(ticket: ticket, expiresInSeconds: expiresSec);
    } catch (_) {
      return null;
    }
  }
}
