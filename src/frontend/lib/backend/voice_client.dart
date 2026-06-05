import 'dart:convert';

import 'package:http/http.dart' as http;

import 'gateway_config.dart';

const String kVoiceMissingBaseUrlDetail = 'missing base URL';

sealed class VoiceApiResult<T> {
  const VoiceApiResult();
}

final class VoiceApiOk<T> extends VoiceApiResult<T> {
  const VoiceApiOk(this.data);
  final T data;
}

final class VoiceApiFailure extends VoiceApiResult<Never> {
  const VoiceApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

enum VoiceCallMediaKind { audio, video }

enum VoiceCallStatus { ringing, active, declined, missed, ended, unknown }

class VoiceCallSession {
  const VoiceCallSession({
    required this.roomId,
    required this.livekitRoomName,
    required this.chatId,
    required this.initiatorProfileId,
    required this.calleeProfileId,
    required this.mediaKind,
    required this.status,
    this.expiresAt,
  });

  final String roomId;
  final String livekitRoomName;
  final String chatId;
  final String initiatorProfileId;
  final String calleeProfileId;
  final VoiceCallMediaKind mediaKind;
  final VoiceCallStatus status;
  final DateTime? expiresAt;

  factory VoiceCallSession.fromJson(Map<String, dynamic> json) {
    final chat = json['linked_chat'] as Map<String, dynamic>? ?? {};
    return VoiceCallSession(
      roomId: json['room_id'] as String? ?? '',
      livekitRoomName: json['livekit_room_name'] as String? ?? '',
      chatId: chat['id'] as String? ?? '',
      initiatorProfileId: json['initiator_profile_id'] as String? ?? '',
      calleeProfileId: json['callee_profile_id'] as String? ?? '',
      mediaKind: _parseMediaKind(json['media_kind']),
      status: _parseStatus(json['status']),
      expiresAt: _parseDate(json['expires_at'] as String?),
    );
  }
}

class VoiceJoinToken {
  const VoiceJoinToken({required this.jwt, this.expiresAt});

  final String jwt;
  final DateTime? expiresAt;

  factory VoiceJoinToken.fromJson(Map<String, dynamic> json) {
    return VoiceJoinToken(
      jwt: json['jwt'] as String? ?? '',
      expiresAt: _parseDate(json['expires_at'] as String?),
    );
  }
}

class VoiceCallsClient {
  VoiceCallsClient({
    required http.Client httpClient,
    required GatewayConfig config,
  }) : _http = httpClient,
       _config = config;

  final http.Client _http;
  final GatewayConfig _config;

  Future<VoiceApiResult<VoiceCallSession?>> getActiveCall({
    required String authorization,
  }) {
    return _request(
      () => _http.get(
        _uri('/api/v1/voice/calls/active'),
        headers: _headers(authorization),
      ),
      (json) {
        final session =
            json['call_session'] as Map<String, dynamic>? ?? const {};
        if (session.isEmpty) return null;
        return VoiceCallSession.fromJson(session);
      },
      allowNotFound: true,
    );
  }

  Future<VoiceApiResult<VoiceCallSession>> startCall({
    required String authorization,
    required String chatId,
    required String calleeProfileId,
    VoiceCallMediaKind mediaKind = VoiceCallMediaKind.audio,
  }) {
    return _postSession('/api/v1/voice/calls', authorization, {
      'linked_chat': {'id': chatId},
      'callee_profile_id': calleeProfileId,
      'media_kind': mediaKind.name,
    });
  }

  Future<VoiceApiResult<VoiceCallSession>> acceptCall({
    required String authorization,
    required String roomId,
  }) {
    return _postSession(
      '/api/v1/voice/calls/$roomId/accept',
      authorization,
      null,
    );
  }

  Future<VoiceApiResult<VoiceCallSession>> declineCall({
    required String authorization,
    required String roomId,
  }) {
    return _postSession(
      '/api/v1/voice/calls/$roomId/decline',
      authorization,
      null,
    );
  }

  Future<VoiceApiResult<void>> endCall({
    required String authorization,
    required String roomId,
  }) {
    return _postEmpty('/api/v1/voice/calls/$roomId/end', authorization);
  }

  Future<VoiceApiResult<VoiceJoinToken>> getJoinToken({
    required String authorization,
    required String roomId,
  }) {
    return _request(
      () => _http.get(
        _uri('/api/v1/voice/calls/$roomId/token'),
        headers: _headers(authorization),
      ),
      (body) => VoiceJoinToken.fromJson(body),
    );
  }

  Future<VoiceApiResult<void>> updateVoiceState({
    required String authorization,
    required String roomId,
    bool? isMuted,
    bool? isDeafened,
    bool? isVideoOn,
  }) {
    final body = <String, dynamic>{};
    if (isMuted != null) body['is_muted'] = isMuted;
    if (isDeafened != null) body['is_deafened'] = isDeafened;
    if (isVideoOn != null) body['is_video_on'] = isVideoOn;
    return _request(
      () => _http.patch(
        _uri('/api/v1/voice/calls/$roomId/state'),
        headers: _headers(authorization),
        body: jsonEncode(body),
      ),
      (_) {},
    );
  }

  Future<VoiceApiResult<VoiceCallSession>> _postSession(
    String path,
    String authorization,
    Map<String, dynamic>? body,
  ) {
    return _request(
      () => _http.post(
        _uri(path),
        headers: _headers(authorization),
        body: body == null ? null : jsonEncode(body),
      ),
      (json) => VoiceCallSession.fromJson(
        json['call_session'] as Map<String, dynamic>? ?? {},
      ),
    );
  }

  Future<VoiceApiResult<void>> _postEmpty(String path, String authorization) {
    return _request(
      () => _http.post(_uri(path), headers: _headers(authorization)),
      (_) {},
    );
  }

  Future<VoiceApiResult<T>> _request<T>(
    Future<http.Response> Function() send,
    T Function(Map<String, dynamic>) parse, {
    bool allowNotFound = false,
  }) async {
    if (!_config.hasBaseUrl) {
      return const VoiceApiFailure(message: kVoiceMissingBaseUrlDetail);
    }
    try {
      final response = await send();
      if (response.statusCode == 404 && allowNotFound) {
        return VoiceApiOk(parse(const {}));
      }
      if (response.statusCode >= 200 && response.statusCode < 300) {
        if (response.body.isEmpty) {
          return VoiceApiOk(parse(const {}));
        }
        return VoiceApiOk(
          parse(jsonDecode(response.body) as Map<String, dynamic>),
        );
      }
      final body = response.body.isEmpty
          ? <String, dynamic>{}
          : jsonDecode(response.body) as Map<String, dynamic>;
      return VoiceApiFailure(
        message: body['message'] as String? ?? 'voice request failed',
        errorCode: body['error_code'] as String?,
        statusCode: response.statusCode,
      );
    } catch (e) {
      return VoiceApiFailure(message: '$e');
    }
  }

  Uri _uri(String path) => Uri.parse(_config.baseUrl).replace(path: path);

  Map<String, String> _headers(String authorization) => {
    'Authorization': authorization,
    'Content-Type': 'application/json',
  };
}

VoiceCallMediaKind _parseMediaKind(dynamic raw) {
  final value = '$raw'.toLowerCase();
  if (value.contains('video')) return VoiceCallMediaKind.video;
  return VoiceCallMediaKind.audio;
}

VoiceCallStatus _parseStatus(dynamic raw) {
  final value = '$raw'.toLowerCase();
  if (value.contains('ringing')) return VoiceCallStatus.ringing;
  if (value.contains('active')) return VoiceCallStatus.active;
  if (value.contains('declined')) return VoiceCallStatus.declined;
  if (value.contains('missed')) return VoiceCallStatus.missed;
  if (value.contains('ended')) return VoiceCallStatus.ended;
  return VoiceCallStatus.unknown;
}

DateTime? _parseDate(String? raw) {
  if (raw == null || raw.isEmpty) return null;
  return DateTime.tryParse(raw);
}
