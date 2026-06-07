import 'package:protobuf/protobuf.dart';

import '../gen/voice/auth/v1/auth.pb.dart' as auth_pb;
import 'api_result.dart';
import 'auth_session.dart';
import 'gateway_http.dart';

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
  VoiceAuthClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  static const _deviceInfoJson = '{"platform":"flutter"}';

  Future<AuthSessionResult> register({
    required String email,
    required String password,
  }) {
    return _postSession(
      '/api/v1/auth/register',
      auth_pb.RegisterRequest(
        email: email,
        password: password,
        guest: false,
      ),
      auth_pb.RegisterResponse.create,
      (response) => response.session,
    );
  }

  Future<AuthSessionResult> login({
    required String email,
    required String password,
  }) {
    return _postSession(
      '/api/v1/auth/login',
      auth_pb.LoginRequest(
        email: email,
        password: password,
        deviceInfoJson: _deviceInfoJson,
      ),
      auth_pb.LoginResponse.create,
      (response) => response.session,
    );
  }

  Future<AuthSessionResult> refresh({required String refreshToken}) {
    return _postSession(
      '/api/v1/auth/refresh',
      auth_pb.RefreshTokenRequest(
        refreshToken: refreshToken,
        deviceInfoJson: _deviceInfoJson,
      ),
      auth_pb.RefreshTokenResponse.create,
      (response) => response.session,
    );
  }

  /// Returns an error message on failure; null on success (204).
  Future<String?> logout({required AuthSession session}) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/auth/logout'),
      authorization: session.authorizationHeader,
      jsonBody: {'refresh_token': session.refreshToken},
    );
    return switch (result) {
      GatewayHttpOk<void>() => null,
      GatewayHttpFailure(:final error) =>
        GatewayApiResultMapper.failureMessage(error),
    };
  }

  Future<AuthSessionResult> _postSession<T extends GeneratedMessage>(
    String path,
    GeneratedMessage body,
    T Function() createEmpty,
    auth_pb.AuthSession Function(T response) readSession,
  ) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve(path),
      body: body,
      createEmpty: createEmpty,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => AuthSessionOk(
        AuthSession.fromProto(readSession(data)),
      ),
      GatewayHttpFailure(:final error) => AuthSessionFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
