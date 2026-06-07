import '../gen/voice/chat/v1/chat.pb.dart' as chat_pb;
import '../gen/voice/chat/v1/chat.pbenum.dart';
import 'api_result.dart';
import 'gateway_http.dart';
import 'proto_mappers.dart';

const String kChatsMissingBaseUrlDetail = 'missing base URL';

sealed class ChatsApiResult<T> {
  const ChatsApiResult();
}

final class ChatsApiOk<T> extends ChatsApiResult<T> {
  const ChatsApiOk(this.data);
  final T data;
}

final class ChatsApiFailure extends ChatsApiResult<Never> {
  const ChatsApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class VoiceChat {
  const VoiceChat({
    required this.id,
    required this.type,
    required this.creatorProfileId,
    this.name,
  });

  final String id;
  final String type;
  final String creatorProfileId;
  final String? name;

  bool get isDm => type == ChatType.CHAT_TYPE_DM.name;

  factory VoiceChat.fromJson(Map<String, dynamic> json) {
    return VoiceChat(
      id: json['id'] as String,
      type: '${json['type']}',
      creatorProfileId: json['creator_profile_id'] as String? ?? '',
      name: json['name'] as String?,
    );
  }
}

class ChatListItem {
  const ChatListItem({
    required this.chat,
    this.lastMessagePreview,
    this.unreadCount = 0,
    this.inbox,
    this.isStranger = false,
    this.dmPeerProfileId,
  });

  final VoiceChat chat;
  final String? lastMessagePreview;
  final int unreadCount;
  final String? inbox;
  final bool isStranger;
  final String? dmPeerProfileId;

  String get chatId => chat.id;
}

class ChatListData {
  const ChatListData({required this.items, this.nextCursor});

  final List<ChatListItem> items;
  final String? nextCursor;
}

/// HTTP client for Chat routes (`/api/v1/chats/**`).
class VoiceChatsClient {
  VoiceChatsClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
  }) async {
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    if (inbox != null && inbox.isNotEmpty) params['inbox'] = inbox;
    final uri = _gateway.replace(
      path: '/api/v1/chats',
      queryParameters: params.isEmpty ? null : params,
    );
    final result = await _gateway.getProto(
      uri,
      authorization: authorization,
      createEmpty: chat_pb.ListChatsResponse.create,
    );
    return _map(
      result,
      (data) => chatListFromProto(
        data.hasChatList() ? data.chatList : chat_pb.ChatList(),
      ),
    );
  }

  Future<ChatsApiResult<void>> acceptDmRequest({
    required String authorization,
    required String chatId,
  }) {
    return _postEmpty('/api/v1/chats/$chatId/accept-request', authorization);
  }

  Future<ChatsApiResult<void>> declineDmRequest({
    required String authorization,
    required String chatId,
  }) {
    return _postEmpty('/api/v1/chats/$chatId/decline-request', authorization);
  }

  Future<ChatsApiResult<void>> _postEmpty(
    String path,
    String authorization,
  ) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve(path),
      authorization: authorization,
    );
    return _mapEmpty(result);
  }

  Future<ChatsApiResult<VoiceChat>> createDm({
    required String authorization,
    required String otherProfileId,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/chats/dm'),
      authorization: authorization,
      body: createDmRequestToProto(otherProfileId),
      createEmpty: chat_pb.CreateDMResponse.create,
    );
    return _map(result, (data) => voiceChatFromProto(data.chat));
  }

  ChatsApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => ChatsApiOk(parse(data)),
      GatewayHttpFailure(:final error) => ChatsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  ChatsApiResult<void> _mapEmpty(GatewayHttpResult<dynamic> result) {
    return switch (result) {
      GatewayHttpOk() => const ChatsApiOk(null),
      GatewayHttpFailure(:final error) => ChatsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
