import 'package:protobuf/protobuf.dart';

import '../gen/voice/calls/v1/calls.pb.dart' as calls_pb;
import 'api_result.dart';
import 'gateway_http.dart';
import 'proto_mappers.dart';

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

enum VoiceSessionKind { dm, groupVoice, voiceRoom, unknown }

class VoiceCallSession {
  const VoiceCallSession({
    required this.roomId,
    required this.livekitRoomName,
    required this.chatId,
    required this.initiatorProfileId,
    required this.calleeProfileId,
    required this.mediaKind,
    required this.status,
    this.sessionKind = VoiceSessionKind.dm,
    this.expiresAt,
  });

  final String roomId;
  final String livekitRoomName;
  final String chatId;
  final String initiatorProfileId;
  final String calleeProfileId;
  final VoiceCallMediaKind mediaKind;
  final VoiceCallStatus status;
  final VoiceSessionKind sessionKind;
  final DateTime? expiresAt;

  bool get isGroupVoice => sessionKind == VoiceSessionKind.groupVoice;
}

class VoiceJoinToken {
  const VoiceJoinToken({
    required this.jwt,
    this.expiresAt,
    this.livekitUrl,
  });

  final String jwt;
  final DateTime? expiresAt;
  final String? livekitUrl;
}

class VoiceRoomSession {
  const VoiceRoomSession({
    required this.roomId,
    required this.livekitRoomName,
    required this.voiceRoomId,
  });

  final String roomId;
  final String livekitRoomName;
  final String voiceRoomId;
}

class VoiceRoomParticipantState {
  const VoiceRoomParticipantState({
    required this.profileId,
    this.isMuted = false,
    this.isDeafened = false,
    this.isVideoOn = false,
    this.isScreenSharing = false,
  });

  final String profileId;
  final bool isMuted;
  final bool isDeafened;
  final bool isVideoOn;
  final bool isScreenSharing;
}

class VoiceCallsClient {
  VoiceCallsClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<VoiceApiResult<VoiceCallSession?>> getActiveCall({
    required String authorization,
  }) async {
    return _getActiveCall(authorization: authorization);
  }

  /// Active group_voice session for a text group (member only).
  Future<VoiceApiResult<VoiceCallSession?>> getActiveGroupCallForChat({
    required String authorization,
    required String groupChatId,
  }) {
    return _getActiveCall(
      authorization: authorization,
      groupChatId: groupChatId,
    );
  }

  Future<VoiceApiResult<VoiceCallSession?>> _getActiveCall({
    required String authorization,
    String? groupChatId,
  }) async {
    final uri = groupChatId == null
        ? _gateway.resolve('/api/v1/voice/calls/active')
        : _gateway.resolve('/api/v1/voice/calls/active').replace(
            queryParameters: {'chat_id': groupChatId},
          );
    final result = await _gateway.getProto(
      uri,
      authorization: authorization,
      createEmpty: calls_pb.GetActiveCallResponse.create,
      allowNotFound: true,
    );
    return _map(result, (data) {
      if (!data.hasCallSession() || data.callSession.roomId.isEmpty) {
        return null;
      }
      return voiceCallSessionFromProto(data.callSession);
    });
  }

  Future<VoiceApiResult<VoiceCallSession>> startCall({
    required String authorization,
    required String chatId,
    required String calleeProfileId,
    VoiceCallMediaKind mediaKind = VoiceCallMediaKind.audio,
  }) {
    return _postSession(
      '/api/v1/voice/calls',
      authorization,
      startCallRequestToProto(
        chatId: chatId,
        calleeProfileId: calleeProfileId,
        mediaKind: mediaKind,
      ),
      calls_pb.StartCallResponse.create,
    );
  }

  /// Phase 4 group voice — temporary room bound to a group chat (no callee).
  Future<VoiceApiResult<VoiceCallSession>> startGroupVoice({
    required String authorization,
    required String groupChatId,
    VoiceCallMediaKind mediaKind = VoiceCallMediaKind.audio,
  }) {
    return _postSession(
      '/api/v1/voice/calls',
      authorization,
      startGroupVoiceRequestToProto(
        groupChatId: groupChatId,
        mediaKind: mediaKind,
      ),
      calls_pb.StartCallResponse.create,
    );
  }

  /// Join an active group voice call (Phase 4).
  Future<VoiceApiResult<VoiceCallSession>> joinCall({
    required String authorization,
    required String roomId,
  }) {
    return _postSession(
      '/api/v1/voice/calls/$roomId/join',
      authorization,
      calls_pb.JoinCallRequest(roomId: roomId),
      calls_pb.JoinCallResponse.create,
    );
  }

  Future<VoiceApiResult<VoiceCallSession>> acceptCall({
    required String authorization,
    required String roomId,
  }) {
    return _postSession(
      '/api/v1/voice/calls/$roomId/accept',
      authorization,
      calls_pb.AcceptCallRequest(roomId: roomId),
      calls_pb.AcceptCallResponse.create,
    );
  }

  Future<VoiceApiResult<VoiceCallSession>> declineCall({
    required String authorization,
    required String roomId,
  }) {
    return _postSession(
      '/api/v1/voice/calls/$roomId/decline',
      authorization,
      calls_pb.DeclineCallRequest(roomId: roomId),
      calls_pb.DeclineCallResponse.create,
    );
  }

  Future<VoiceApiResult<void>> endCall({
    required String authorization,
    required String roomId,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/voice/calls/$roomId/end'),
      authorization: authorization,
    );
    return _mapEmpty(result);
  }

  Future<VoiceApiResult<VoiceJoinToken>> getJoinToken({
    required String authorization,
    required String roomId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/voice/calls/$roomId/token'),
      authorization: authorization,
      createEmpty: calls_pb.GetJoinTokenResponse.create,
    );
    return _map(
      result,
      (data) => voiceJoinTokenFromProto(data as calls_pb.GetJoinTokenResponse),
    );
  }

  /// Phase 5 space voice room — join persistent room in a space.
  Future<VoiceApiResult<VoiceRoomSession>> joinVoiceRoom({
    required String authorization,
    required String voiceRoomId,
    required String spaceId,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/voice/rooms/$voiceRoomId/join'),
      authorization: authorization,
      body: {
        'space': {'id': spaceId},
      },
    );
    return _mapJson(result, voiceRoomSessionFromJson);
  }

  Future<VoiceApiResult<void>> leaveVoiceRoom({
    required String authorization,
    required String voiceRoomId,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/voice/rooms/$voiceRoomId/leave'),
      authorization: authorization,
    );
    return _mapEmpty(result);
  }

  Future<VoiceApiResult<List<VoiceRoomParticipantState>>> getVoiceRoomStates({
    required String authorization,
    required String voiceRoomId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/voice/rooms/$voiceRoomId/states'),
      authorization: authorization,
    );
    return _mapJson(result, voiceRoomParticipantStatesFromJson);
  }

  Future<VoiceApiResult<void>> updateVoiceState({
    required String authorization,
    required String roomId,
    bool? isMuted,
    bool? isDeafened,
    bool? isVideoOn,
  }) async {
    final result = await _gateway.patchProto(
      uri: _gateway.resolve('/api/v1/voice/calls/$roomId/state'),
      authorization: authorization,
      body: updateVoiceStateRequestToProto(
        roomId: roomId,
        isMuted: isMuted,
        isDeafened: isDeafened,
        isVideoOn: isVideoOn,
      ),
      createEmpty: calls_pb.UpdateVoiceStateResponse.create,
    );
    return _mapEmpty(result);
  }

  Future<VoiceApiResult<VoiceCallSession>> _postSession<T extends GeneratedMessage>(
    String path,
    String authorization,
    GeneratedMessage body,
    T Function() createEmpty,
  ) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve(path),
      authorization: authorization,
      body: body,
      createEmpty: createEmpty,
    );
    return _map(result, (data) {
      final session = _readCallSession(data);
      return voiceCallSessionFromProto(session);
    });
  }

  calls_pb.CallSession _readCallSession(GeneratedMessage response) {
    if (response is calls_pb.StartCallResponse) return response.callSession;
    if (response is calls_pb.AcceptCallResponse) return response.callSession;
    if (response is calls_pb.DeclineCallResponse) return response.callSession;
    if (response is calls_pb.JoinCallResponse) return response.callSession;
    return calls_pb.CallSession();
  }

  VoiceApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => VoiceApiOk(parse(data)),
      GatewayHttpFailure(:final error) => VoiceApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  VoiceApiResult<T> _mapJson<T>(
    GatewayHttpResult<Map<String, dynamic>> result,
    T Function(Map<String, dynamic> data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => VoiceApiOk(parse(data)),
      GatewayHttpFailure(:final error) => VoiceApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  VoiceApiResult<void> _mapEmpty(GatewayHttpResult<dynamic> result) {
    return switch (result) {
      GatewayHttpOk() => const VoiceApiOk(null),
      GatewayHttpFailure(:final error) => VoiceApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
