import 'package:http/http.dart' as http;

import 'gateway_api_error.dart';
import 'gateway_config.dart';
import 'gateway_http.dart';

/// Frozen settings-oauth-transport-v1-5f445604. No session refresh/retry.
class LinkedOAuthClient {
  LinkedOAuthClient({
    required http.Client httpClient,
    required GatewayConfig config,
    required Map<String, Uri> authorizationEndpoints,
  }) : _gateway = GatewayHttpClient(httpClient: httpClient, config: config),
       _authorizationEndpoints = Map.unmodifiable(authorizationEndpoints) {
    if (authorizationEndpoints.entries.any(
      (entry) =>
          !_provider(entry.key) ||
          entry.value.scheme != 'https' ||
          entry.value.host.isEmpty ||
          entry.value.userInfo.isNotEmpty ||
          entry.value.hasQuery ||
          entry.value.hasFragment,
    )) {
      throw ArgumentError(
        'Exact configured HTTPS authorization endpoints required',
      );
    }
  }

  final GatewayHttpClient _gateway;
  final Map<String, Uri> _authorizationEndpoints;

  Future<GatewayHttpResult<LinkedOAuthAttempt>> start({
    required String provider,
    required String authorization,
    required String surface,
    required String challenge,
  }) async {
    final endpoint = _authorizationEndpoints[provider];
    if (!_provider(provider) ||
        endpoint == null ||
        !const ['web', 'windows'].contains(surface)) {
      return _invalid('oauth_unavailable');
    }
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/auth/linked-accounts/$provider/link'),
      authorization: authorization,
      body: {
        'client_surface': surface,
        'return_challenge': challenge,
        'return_challenge_method': 'S256',
      },
    );
    if (result case GatewayHttpFailure()) return result;
    final data = (result as GatewayHttpOk<Map<String, dynamic>>).data;
    final id = data['attempt_id'];
    final url = data['authorization_url'];
    final expires = data['expires_at'];
    final uri = url is String ? Uri.tryParse(url) : null;
    final expiry = expires is String ? DateTime.tryParse(expires) : null;
    if (id is! String ||
        !linkedOAuthAttemptId.hasMatch(id) ||
        uri == null ||
        uri.scheme != 'https' ||
        uri.host.isEmpty ||
        uri.userInfo.isNotEmpty ||
        uri.hasFragment ||
        uri.origin != endpoint.origin ||
        uri.path != endpoint.path ||
        expiry == null ||
        !expiry.isUtc) {
      return _invalid('invalid_response');
    }
    return GatewayHttpOk(LinkedOAuthAttempt(id, uri, expiry));
  }

  Future<GatewayHttpResult<LinkedOAuthOutcome>> consume({
    required String provider,
    required String authorization,
    required String attemptId,
    required String resultHandle,
    required String verifier,
  }) async {
    if (!_provider(provider)) return _invalid('invalid_link_attempt');
    final result = await _gateway.postJson(
      uri: _gateway.resolve(
        '/api/v1/auth/linked-accounts/$provider/results/consume',
      ),
      authorization: authorization,
      body: {
        'attempt_id': attemptId,
        'result_handle': resultHandle,
        'return_verifier': verifier,
      },
    );
    if (result case GatewayHttpFailure()) return result;
    final value =
        (result as GatewayHttpOk<Map<String, dynamic>>).data['outcome'];
    final outcome = switch (value) {
      'success' => LinkedOAuthOutcome.success,
      'cancelled' => LinkedOAuthOutcome.cancelled,
      'denied' => LinkedOAuthOutcome.denied,
      'provider_error' => LinkedOAuthOutcome.providerError,
      'unavailable' => LinkedOAuthOutcome.unavailable,
      _ => null,
    };
    return outcome == null
        ? _invalid('invalid_response')
        : GatewayHttpOk(outcome);
  }

  Future<GatewayHttpResult<void>> cancel({
    required String provider,
    required String authorization,
    required String attemptId,
  }) {
    if (!_provider(provider) || !linkedOAuthAttemptId.hasMatch(attemptId)) {
      return Future.value(_invalid('invalid_link_attempt'));
    }
    return _gateway.postEmpty(
      uri: _gateway.resolve(
        '/api/v1/auth/linked-accounts/$provider/attempts/$attemptId/cancel',
      ),
      authorization: authorization,
    );
  }

  static bool _provider(String value) =>
      const ['twitch', 'youtube'].contains(value);
  static GatewayHttpFailure _invalid(String code) => GatewayHttpFailure(
    GatewayApiError(errorCode: code, message: code, statusCode: 0),
  );
}

final linkedOAuthAttemptId = RegExp(
  r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$',
);

class LinkedOAuthAttempt {
  const LinkedOAuthAttempt(this.id, this.authorizationUri, this.expiresAt);
  final String id;
  final Uri authorizationUri;
  final DateTime expiresAt;
}

enum LinkedOAuthOutcome {
  success,
  cancelled,
  denied,
  providerError,
  unavailable,
}
