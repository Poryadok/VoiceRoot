import 'dart:convert';

import 'package:http/http.dart' as http;

import 'auth_session.dart';
import 'gateway_config.dart';

/// Detail when [GatewayConfig.hasBaseUrl] is false; aligned with gateway client i18n key pattern.
const String kAuthMissingBaseUrlDetail = 'missing base URL';

sealed class AuthSessionResult {
  const AuthSessionResult();
}

final class AuthSessionOk extends AuthSessionResult {
  const AuthSessionOk(this.session);
  final AuthSession session;
}

final class AuthSessionFailure extends AuthSessionResult {
  const AuthSessionFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

/// HTTP client for public Auth routes via API Gateway (`/api/v1/auth/*`).
class VoiceAuthClient {
  VoiceAuthClient({
    required http.Client httpClient,
    required GatewayConfig config,
  }) : _http = httpClient,
       _config = config;

  final http.Client _http;
  final GatewayConfig _config;

  static const _deviceInfoJson = '{"platform":"flutter"}';

  Future<AuthSessionResult> register({
    required String email,
    required String password,
  }) => _postSession('/api/v1/auth/register', {
    'email': email,
    'password': password,
    'guest': false,
    'device_info_json': _deviceInfoJson,
  });

  Future<AuthSessionResult> login({
    required String email,
    required String password,
  }) => _postSession('/api/v1/auth/login', {
    'email': email,
    'password': password,
    'device_info_json': _deviceInfoJson,
  });

  Future<AuthSessionResult> refresh({required String refreshToken}) =>
      _postSession('/api/v1/auth/refresh', {
        'refresh_token': refreshToken,
        'device_info_json': _deviceInfoJson,
      });

  /// Returns an error message on failure; null on success (204).
  Future<String?> logout({required AuthSession session}) async {
    if (!_config.hasBaseUrl) {
      return kAuthMissingBaseUrlDetail;
    }
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/auth/logout');
    try {
      final res = await _http.post(
        uri,
        headers: {
          'Content-Type': 'application/json',
          'Authorization': session.authorizationHeader,
        },
        body: jsonEncode({'refresh_token': session.refreshToken}),
      );
      if (res.statusCode == 204) return null;
      return _failureMessage(res);
    } catch (e) {
      return '$e';
    }
  }

  Future<AuthSessionResult> _postSession(
    String path,
    Map<String, dynamic> body,
  ) async {
    if (!_config.hasBaseUrl) {
      return const AuthSessionFailure(message: kAuthMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl).resolve(path);
    try {
      final res = await _http.post(
        uri,
        headers: {'Content-Type': 'application/json'},
        body: jsonEncode(body),
      );
      if (res.statusCode == 200) {
        final decoded = jsonDecode(res.body) as Map<String, dynamic>;
        return AuthSessionOk(AuthSession.fromAuthResponse(decoded));
      }
      return AuthSessionFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return AuthSessionFailure(message: '$e');
    }
  }

  static String _failureMessage(http.Response res) {
    final code = _errorCode(res);
    if (code != null) return code;
    return 'HTTP ${res.statusCode}';
  }

  static String? _errorCode(http.Response res) {
    try {
      final decoded = jsonDecode(res.body);
      if (decoded is Map<String, dynamic>) {
        final err = decoded['error'];
        if (err is String && err.isNotEmpty) return err;
      }
    } catch (_) {
      // ignore malformed body
    }
    return null;
  }
}
