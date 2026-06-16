import 'package:http/http.dart' as http;

import 'api_result.dart';
import 'gateway_http.dart';

sealed class DeepLinksApiResult<T> {
  const DeepLinksApiResult();
}

final class DeepLinksApiOk<T> extends DeepLinksApiResult<T> {
  const DeepLinksApiOk(this.data);
  final T data;
}

final class DeepLinksApiFailure extends DeepLinksApiResult<Never> {
  const DeepLinksApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class ResolvedDeepLink {
  const ResolvedDeepLink({
    required this.kind,
    this.spaceId,
    this.chatId,
    this.voiceRoomId,
    this.messageId,
    this.inviteCode,
    this.username,
    this.userId,
    this.webPath,
    this.appUri,
  });

  final String kind;
  final String? spaceId;
  final String? chatId;
  final String? voiceRoomId;
  final String? messageId;
  final String? inviteCode;
  final String? username;
  final String? userId;
  final String? webPath;
  final String? appUri;

  factory ResolvedDeepLink.fromJson(Map<String, dynamic> json) {
    return ResolvedDeepLink(
      kind: json['kind'] as String? ?? '',
      spaceId: json['space_id'] as String?,
      chatId: json['chat_id'] as String?,
      voiceRoomId: json['voice_room_id'] as String?,
      messageId: json['message_id'] as String?,
      inviteCode: json['invite_code'] as String?,
      username: json['username'] as String?,
      userId: json['user_id'] as String?,
      webPath: json['web_path'] as String?,
      appUri: json['app_uri'] as String?,
    );
  }
}

class VoiceDeepLinksClient {
  VoiceDeepLinksClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<DeepLinksApiResult<String>> fetchInviteLanding({required String code}) async {
    if (!_gateway.hasBaseUrl) {
      return const DeepLinksApiFailure(message: 'missing base URL');
    }
    final uri = _gateway.resolve('/invite/$code');
    try {
      final resp = await http.get(uri);
      if (resp.statusCode != 200) {
        return DeepLinksApiFailure(
          message: 'HTTP ${resp.statusCode}',
          statusCode: resp.statusCode,
        );
      }
      return DeepLinksApiOk(resp.body);
    } catch (e) {
      return DeepLinksApiFailure(message: e.toString());
    }
  }

  Future<DeepLinksApiResult<ResolvedDeepLink>> resolve({
    required String authorization,
    required String url,
  }) async {
    final result = await _gateway.getJson(
      _gateway.replace(
        path: '/api/v1/links/resolve',
        queryParameters: {'url': url},
      ),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => DeepLinksApiOk(ResolvedDeepLink.fromJson(data)),
      GatewayHttpFailure(:final error) => DeepLinksApiFailure(
          message: GatewayApiResultMapper.failureMessage(error),
          errorCode: GatewayApiResultMapper.failureCode(error),
          statusCode: GatewayApiResultMapper.failureStatus(error),
        ),
    };
  }
}
