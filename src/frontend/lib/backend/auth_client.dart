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

sealed class Enable2FAResult {
  const Enable2FAResult();
}

final class Enable2FAOk extends Enable2FAResult {
  const Enable2FAOk(this.enrollment);
  final TotpEnrollmentData enrollment;
}

final class Enable2FAFailure extends Enable2FAResult {
  const Enable2FAFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class TotpEnrollmentData {
  const TotpEnrollmentData({
    required this.totpUri,
    required this.secretBackupHint,
    required this.backupCodes,
  });

  final String totpUri;
  final String secretBackupHint;
  final List<String> backupCodes;
}

sealed class AuthApiResult<T> {
  const AuthApiResult();
}

final class AuthApiOk<T> extends AuthApiResult<T> {
  const AuthApiOk(this.data);
  final T data;
}

final class AuthApiFailure extends AuthApiResult<Never> {
  const AuthApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class LinkedAccount {
  const LinkedAccount({
    required this.platform,
    this.externalId,
    this.displayName,
    this.linkedAt,
  });

  final String platform;
  final String? externalId;
  final String? displayName;
  final DateTime? linkedAt;
}

List<LinkedAccount> linkedAccountsFromJson(Map<String, dynamic> json) {
  final raw = json['linked_accounts'];
  if (raw is! List<dynamic>) return const [];
  return raw
      .whereType<Map<String, dynamic>>()
      .map(
        (item) => LinkedAccount(
          platform: item['platform'] as String? ?? '',
          externalId: item['external_id'] as String?,
          displayName: item['display_name'] as String?,
        ),
      )
      .where((a) => a.platform.isNotEmpty)
      .toList(growable: false);
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

  Future<AuthSessionResult> registerGuest({required String password}) {
    return _postSession(
      '/api/v1/auth/register',
      auth_pb.RegisterRequest(
        password: password,
        guest: true,
      ),
      auth_pb.RegisterResponse.create,
      (response) => response.session,
    );
  }

  Future<AuthSessionResult> convertGuest({
    required AuthSession session,
    required String email,
    required String password,
  }) {
    return _postSession(
      '/api/v1/auth/convert-guest',
      auth_pb.ConvertGuestRequest(
        email: email,
        password: password,
      ),
      auth_pb.ConvertGuestResponse.create,
      (response) => response.session,
      authorization: session.authorizationHeader,
    );
  }

  Future<AuthSessionResult> login({
    required String email,
    required String password,
    String? totpCode,
  }) {
    return _postSession(
      '/api/v1/auth/login',
      auth_pb.LoginRequest(
        email: email,
        password: password,
        totpCode: totpCode,
        deviceInfoJson: _deviceInfoJson,
      ),
      auth_pb.LoginResponse.create,
      (response) => response.session,
    );
  }

  Future<Enable2FAResult> enable2FA({
    required AuthSession session,
    required String password,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/auth/2fa/enable'),
      authorization: session.authorizationHeader,
      body: {'password': password},
    );
    return switch (result) {
      GatewayHttpOk(:final data) => Enable2FAOk(
        TotpEnrollmentData(
          totpUri: data['totp_uri'] as String? ?? '',
          secretBackupHint: data['secret_backup_hint'] as String? ?? '',
          backupCodes:
              (data['backup_codes'] as List<dynamic>?)
                  ?.map((e) => e.toString())
                  .toList() ??
              const [],
        ),
      ),
      GatewayHttpFailure(:final error) => Enable2FAFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<AuthSessionResult> verify2FA({
    required AuthSession session,
    required String totpCode,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/auth/2fa/verify'),
      authorization: session.authorizationHeader,
      body: {'totp_code': totpCode},
    );
    return switch (result) {
      GatewayHttpOk(:final data) => AuthSessionOk(
        AuthSession.fromAuthResponse(data),
      ),
      GatewayHttpFailure(:final error) => AuthSessionFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<AuthSessionResult> switchActiveProfile({
    required AuthSession session,
    required String profileId,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/auth/switch-profile'),
      authorization: session.authorizationHeader,
      body: {
        'profile_id': profileId,
        'device_info_json': _deviceInfoJson,
      },
    );
    return switch (result) {
      GatewayHttpOk(:final data) => AuthSessionOk(
        AuthSession.fromAuthResponse(data),
      ),
      GatewayHttpFailure(:final error) => AuthSessionFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<AuthApiResult<List<LinkedAccount>>> listLinkedAccounts({
    required AuthSession session,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/auth/linked-accounts'),
      authorization: session.authorizationHeader,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => AuthApiOk(
        linkedAccountsFromJson(data),
      ),
      GatewayHttpFailure(:final error) => AuthApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<AuthApiResult<String>> startLinkedAccountLink({
    required AuthSession session,
    required String platform,
    required String redirectUri,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/auth/linked-accounts/$platform/link'),
      authorization: session.authorizationHeader,
      body: {'redirect_uri': redirectUri},
    );
    return switch (result) {
      GatewayHttpOk(:final data) => AuthApiOk(
        data['authorization_url'] as String? ?? '',
      ),
      GatewayHttpFailure(:final error) => AuthApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
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
    auth_pb.AuthSession Function(T response) readSession, {
    String? authorization,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve(path),
      authorization: authorization,
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
