import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';

import '../e2e/e2e_crypto_adapter.dart';
import '../e2e/e2e_key_backup.dart';
import '../e2e/e2e_prekey_sync.dart';
import 'api_result.dart';
import 'gateway_http.dart';
import 'jwt_claims.dart';

const String kE2eMissingBaseUrlDetail = 'missing base URL';

sealed class E2eApiResult<T> {
  const E2eApiResult();
}

final class E2eApiOk<T> extends E2eApiResult<T> {
  const E2eApiOk(this.data);
  final T data;
}

final class E2eApiFailure extends E2eApiResult<Never> {
  const E2eApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class E2eKeyBackupData {
  const E2eKeyBackupData({
    required this.encryptedBlob,
    this.passwordHint,
  });

  final String encryptedBlob;
  final String? passwordHint;
}

/// REST client for Phase 15 E2E (pre-keys, chat opt-in, key backup).
class VoiceE2eClient {
  VoiceE2eClient({
    required GatewayHttpClient gateway,
    E2eCryptoAdapter? adapter,
  }) : _gateway = gateway,
       _adapter = adapter ?? E2eCryptoAdapter() {
    _preKeys = E2ePreKeySync(sessionManager: _adapter.sessionManager);
  }

  final GatewayHttpClient _gateway;
  final E2eCryptoAdapter _adapter;
  late final E2ePreKeySync _preKeys;

  E2eCryptoAdapter get cryptoAdapter => _adapter;

  Future<E2eApiResult<void>> uploadPreKeyBundle({
    required String authorization,
    String? bundle,
  }) async {
    final wire = bundle ?? await _preKeys.bundleForProfile(
      _requireProfileId(authorization),
    );
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/messages/prekeys'),
      authorization: authorization,
      body: {'bundle': wire},
      allowNoContent: true,
    );
    return _mapVoid(result);
  }

  Future<E2eApiResult<String>> getPreKeyBundle({
    required String authorization,
    required String profileId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.replace(
        path: '/api/v1/messages/prekeys',
        queryParameters: {'profile_id': profileId},
      ),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => E2eApiOk(
        data['bundle'] as String? ?? '',
      ),
      GatewayHttpFailure(:final error) => E2eApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<E2eApiResult<void>> enableChatE2e({
    required String authorization,
    required String chatId,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/chats/$chatId/e2e-enable'),
      authorization: authorization,
      body: {'chat_id': chatId},
      allowNoContent: true,
    );
    return _mapVoid(result);
  }

  Future<E2eApiResult<void>> disableChatE2e({
    required String authorization,
    required String chatId,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/chats/$chatId/e2e-disable'),
      authorization: authorization,
      body: {'chat_id': chatId},
      allowNoContent: true,
    );
    return _mapVoid(result);
  }

  Future<String> encryptForChat({
    required String authorization,
    required String chatId,
    required String peerProfileId,
    required String plaintext,
  }) async {
    final localProfileId = _requireProfileId(authorization);
    PreKeyBundle? remoteBundle;
    final peerBundle = await getPreKeyBundle(
      authorization: authorization,
      profileId: peerProfileId,
    );
    if (peerBundle is E2eApiOk<String> && peerBundle.data.isNotEmpty) {
      remoteBundle = _preKeys.bundleFromWire(peerBundle.data);
    }

    final session = await _adapter.ensureSession(
      localProfileId: localProfileId,
      remoteProfileId: peerProfileId,
      remoteBundle: remoteBundle,
    );
    return _adapter.encryptToWire(session: session, plaintext: plaintext);
  }

  Future<E2eApiResult<void>> putKeyBackup({
    required String authorization,
    required String password,
    String? passwordHint,
  }) async {
    final profileId = _requireProfileId(authorization);
    final codec = const E2eKeyBackupCodec();
    final blob = codec.encryptPayload(
      password: password,
      passwordHint: passwordHint,
      payload: {
        'profile_id': profileId,
        'version': 1,
        'exported_at': DateTime.now().toUtc().toIso8601String(),
      },
    );
    final result = await _gateway.putJson(
      uri: _gateway.resolve('/api/v1/auth/e2e-key-backup'),
      authorization: authorization,
      body: {
        'encrypted_blob': blob,
        if (passwordHint != null && passwordHint.isNotEmpty)
          'password_hint': passwordHint,
      },
      allowNoContent: true,
    );
    return _mapVoid(result);
  }

  Future<E2eApiResult<E2eKeyBackupData>> getKeyBackup({
    required String authorization,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/auth/e2e-key-backup'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => E2eApiOk(
        E2eKeyBackupData(
          encryptedBlob: data['encrypted_blob'] as String? ?? '',
          passwordHint: data['password_hint'] as String?,
        ),
      ),
      GatewayHttpFailure(:final error) => E2eApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  String _requireProfileId(String authorization) {
    final token = authorization.replaceFirst(RegExp(r'^Bearer\s+', caseSensitive: false), '');
    final payload = decodeJwtPayload(token);
    final profileId = payload?['profile_id'] as String?;
    if (profileId == null || profileId.isEmpty) {
      throw StateError('authorization missing profile_id claim');
    }
    return profileId;
  }

  E2eApiResult<void> _mapVoid(GatewayHttpResult<Map<String, dynamic>> result) {
    return switch (result) {
      GatewayHttpOk() => const E2eApiOk(null),
      GatewayHttpFailure(:final error) => E2eApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}

String? profileIdFromAuthorization(String? authorization) {
  if (authorization == null || authorization.isEmpty) return null;
  final token = authorization.replaceFirst(RegExp(r'^Bearer\s+', caseSensitive: false), '');
  final payload = decodeJwtPayload(token);
  final profileId = payload?['profile_id'];
  return profileId is String && profileId.isNotEmpty ? profileId : null;
}
