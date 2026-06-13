import 'package:protobuf/protobuf.dart';

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
    this.avatarUrl,
    this.spaceId,
    this.slowModeSeconds = 0,
    this.threadsEnabled = false,
    this.allowUserMainFeed = true,
  });

  final String id;
  final String type;
  final String creatorProfileId;
  final String? name;
  final String? avatarUrl;
  final String? spaceId;
  final int slowModeSeconds;
  final bool threadsEnabled;
  final bool allowUserMainFeed;

  bool get isDm => type == ChatType.CHAT_TYPE_DM.name;
  bool get isGroup => type == ChatType.CHAT_TYPE_GROUP.name;
  bool get isChannel => type == ChatType.CHAT_TYPE_CHANNEL.name;

  factory VoiceChat.fromJson(Map<String, dynamic> json) {
    return VoiceChat(
      id: json['id'] as String,
      type: '${json['type']}',
      creatorProfileId: json['creator_profile_id'] as String? ?? '',
      name: json['name'] as String?,
      avatarUrl: json['avatar_url'] as String?,
      spaceId: json['space_id'] as String?,
      slowModeSeconds: json['slow_mode_seconds'] as int? ?? 0,
      threadsEnabled: json['threads_enabled'] as bool? ?? false,
      allowUserMainFeed: json['allow_user_main_feed'] as bool? ?? true,
    );
  }

  bool get isSpaceChannel => spaceId != null && spaceId!.isNotEmpty;
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

/// Simple group roles (Phase 4): creator is [kChatRoleOwner], invitees [kChatRoleMember].
const String kChatRoleOwner = 'owner';
const String kChatRoleMember = 'member';

class ChatMember {
  const ChatMember({
    required this.profileId,
    required this.role,
    this.joinedAt,
    this.isArchived = false,
  });

  final String profileId;
  final String role;
  final DateTime? joinedAt;
  final bool isArchived;

  bool get isOwner => role == kChatRoleOwner;
}

class MemberListData {
  const MemberListData({required this.members, this.nextCursor});

  final List<ChatMember> members;
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

  Future<ChatsApiResult<VoiceChat>> createGroup({
    required String authorization,
    required String name,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/chats'),
      authorization: authorization,
      body: createGroupRequestToProto(name: name),
      createEmpty: chat_pb.CreateChatResponse.create,
    );
    return _map(result, (data) => voiceChatFromProto(data.chat));
  }

  Future<ChatsApiResult<void>> addGroupMembers({
    required String authorization,
    required String chatId,
    required List<String> profileIds,
  }) {
    return _postEmpty(
      '/api/v1/chats/$chatId/members',
      authorization,
      body: addMembersRequestToProto(profileIds: profileIds),
    );
  }

  Future<ChatsApiResult<void>> removeGroupMember({
    required String authorization,
    required String chatId,
    required String profileId,
  }) {
    return _deleteEmpty('/api/v1/chats/$chatId/members/$profileId', authorization);
  }

  Future<ChatsApiResult<MemberListData>> listGroupMembers({
    required String authorization,
    required String chatId,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = _gateway.replace(
      path: '/api/v1/chats/$chatId/members',
      queryParameters: params.isEmpty ? null : params,
    );
    final result = await _gateway.getProto(
      uri,
      authorization: authorization,
      createEmpty: chat_pb.ListMembersResponse.create,
    );
    return _map(
      result,
      (data) => memberListFromProto(
        data.hasMemberList() ? data.memberList : chat_pb.MemberList(),
      ),
    );
  }

  Future<ChatsApiResult<void>> leaveGroup({
    required String authorization,
    required String chatId,
  }) {
    return _postEmpty('/api/v1/chats/$chatId/leave', authorization);
  }

  Future<ChatsApiResult<VoiceChat>> updateGroup({
    required String authorization,
    required String chatId,
    String? name,
    String? avatarUrl,
    int? slowModeSeconds,
  }) async {
    final result = await _gateway.patchProto(
      uri: _gateway.resolve('/api/v1/chats/$chatId'),
      authorization: authorization,
      body: updateChatRequestToProto(
        name: name,
        avatarUrl: avatarUrl,
        slowModeSeconds: slowModeSeconds,
      ),
      createEmpty: chat_pb.UpdateChatResponse.create,
    );
    return _map(result, (data) => voiceChatFromProto(data.chat));
  }

  Future<ChatsApiResult<void>> _postEmpty(
    String path,
    String authorization, {
    GeneratedMessage? body,
  }) async {
    final result = body == null
        ? await _gateway.postEmpty(
            uri: _gateway.resolve(path),
            authorization: authorization,
          )
        : await _gateway.postProto(
            uri: _gateway.resolve(path),
            authorization: authorization,
            body: body,
            createEmpty: chat_pb.AddMembersResponse.create,
            allowNoContent: true,
          );
    return _mapEmpty(result);
  }

  Future<ChatsApiResult<void>> _deleteEmpty(
    String path,
    String authorization,
  ) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve(path),
      authorization: authorization,
    );
    return _mapEmpty(result);
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
