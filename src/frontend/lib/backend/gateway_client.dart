import 'package:http/http.dart' as http;

import 'gateway_config.dart';

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
      final res = await _http.get(resolved);
      if (res.statusCode == 200) {
        return const GatewayHealthOk();
      }
      return GatewayHealthFailure('HTTP ${res.statusCode}');
    } catch (e) {
      return GatewayHealthFailure('$e');
    }
  }

  /// `GET /api/v1/version` — public, no JWT.
  Future<String?> fetchVersionBody() async {
    if (!_config.hasBaseUrl) return null;
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/version');
    try {
      final res = await _http.get(uri);
      if (res.statusCode == 200) return res.body;
      return null;
    } catch (_) {
      return null;
    }
  }
}
