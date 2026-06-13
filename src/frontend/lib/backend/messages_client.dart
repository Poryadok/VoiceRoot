import 'dart:convert';

import '../gen/voice/messaging/v1/messaging.pb.dart' as messaging_pb;
import 'api_result.dart';
import 'gateway_http.dart';
import 'proto_mappers.dart';

const String kMessagesMissingBaseUrlDetail = 'missing base URL';

sealed class MessagesApiResult<T> {
  const MessagesApiResult();
}

final class MessagesApiOk<T> extends MessagesApiResult<T> {
  const MessagesApiOk(this.data);
  final T data;
}

final class MessagesApiFailure extends MessagesApiResult<Never> {
  const MessagesApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

enum VoiceMessageKind { regular, system, forward, unknown }

/// Structured mention entry (`mentions_json` array element).
class MessageMention {
  const MessageMention({required this.type, this.targetId});

  final String type;
  final String? targetId;

  Map<String, dynamic> toJson() {
    return {
      'type': type,
      if (targetId != null && targetId!.isNotEmpty) 'target_id': targetId,
    };
  }

  factory MessageMention.fromJson(Map<String, dynamic> json) {
    return MessageMention(
      type: json['type'] as String? ?? '',
      targetId: json['target_id'] as String?,
    );
  }

  static List<MessageMention> listFromWire(dynamic raw) {
    Object? decoded = raw;
    if (raw is String && raw.trim().isNotEmpty) {
      try {
        decoded = jsonDecode(raw);
      } catch (_) {
        decoded = const [];
      }
    }
    if (decoded is! List) return const [];
    return decoded
        .whereType<Map<String, dynamic>>()
        .map(MessageMention.fromJson)
        .where((m) => m.type.isNotEmpty)
        .toList(growable: false);
  }

  static String encodeJson(List<MessageMention> mentions) {
    if (mentions.isEmpty) return '[]';
    return jsonEncode(
      mentions.map((m) => m.toJson()).toList(growable: false),
    );
  }
}

/// Aggregated emoji reaction on a message (PLAN Phase 4 / text-chat.md).
class MessageReaction {
  const MessageReaction({
    required this.emoji,
    required this.count,
    this.reactedByMe = false,
  });

  final String emoji;
  final int count;
  final bool reactedByMe;

  factory MessageReaction.fromJson(Map<String, dynamic> json) {
    return MessageReaction(
      emoji: json['emoji'] as String? ?? '',
      count: (json['count'] as num?)?.toInt() ?? 0,
      reactedByMe: json['reacted_by_me'] as bool? ?? false,
    );
  }

  static List<MessageReaction> listFromWire(dynamic raw) {
    Object? decoded = raw;
    if (raw is String && raw.trim().isNotEmpty) {
      try {
        decoded = jsonDecode(raw);
      } catch (_) {
        decoded = const [];
      }
    }
    if (decoded is! List) return const [];
    return decoded
        .whereType<Map<String, dynamic>>()
        .map(MessageReaction.fromJson)
        .where((r) => r.emoji.isNotEmpty && r.count > 0)
        .toList(growable: false);
  }
}

class VoiceMessage {
  const VoiceMessage({
    required this.id,
    required this.chatId,
    required this.senderProfileId,
    required this.content,
    this.attachments = const [],
    this.reactions = const [],
    this.mentions = const [],
    this.messageKind = VoiceMessageKind.regular,
    this.forwardFromId,
    this.forwardFromSender,
    this.editedAt,
    this.deletedAt,
    this.createdAt,
    this.isPinned = false,
    this.threadParentId,
  });

  final String id;
  final String chatId;
  final String senderProfileId;
  final String content;
  final List<MessageAttachment> attachments;
  final List<MessageReaction> reactions;
  final List<MessageMention> mentions;
  final VoiceMessageKind messageKind;
  final String? forwardFromId;
  final String? forwardFromSender;
  final DateTime? editedAt;
  final DateTime? deletedAt;
  final DateTime? createdAt;
  final bool isPinned;
  final String? threadParentId;

  factory VoiceMessage.fromJson(Map<String, dynamic> json) {
    final chat = json['chat'] as Map<String, dynamic>? ?? {};
    return VoiceMessage(
      id: json['id'] as String,
      chatId: chat['id'] as String? ?? json['chat_id'] as String? ?? '',
      senderProfileId: json['sender_profile_id'] as String? ?? '',
      content: json['content'] as String? ?? '',
      attachments: MessageAttachment.listFromWire(json['attachments_json']),
      reactions: MessageReaction.listFromWire(json['reactions_json']),
      mentions: MessageMention.listFromWire(json['mentions_json']),
      messageKind: _messageKindFromWire(json['message_kind'] as String?),
      forwardFromId: json['forward_from_id'] as String?,
      forwardFromSender: json['forward_from_sender'] as String?,
      editedAt: VoiceMessage.parseTimestamp(json['edited_at']),
      deletedAt: VoiceMessage.parseTimestamp(json['deleted_at']),
      createdAt: VoiceMessage.parseTimestamp(json['created_at']),
      isPinned: json['is_pinned'] as bool? ?? false,
      threadParentId: json['thread_parent_id'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'chat': {'id': chatId},
      'chat_id': chatId,
      'sender_profile_id': senderProfileId,
      'content': content,
      if (attachments.isNotEmpty)
        'attachments_json': attachments.map((a) => a.toJson()).toList(),
      if (reactions.isNotEmpty)
        'reactions_json': reactions
            .map(
              (r) => {
                'emoji': r.emoji,
                'count': r.count,
                'reacted_by_me': r.reactedByMe,
              },
            )
            .toList(),
      if (mentions.isNotEmpty)
        'mentions_json': mentions.map((m) => m.toJson()).toList(),
      'message_kind': _messageKindToWire(messageKind),
      if (forwardFromId != null) 'forward_from_id': forwardFromId,
      if (forwardFromSender != null) 'forward_from_sender': forwardFromSender,
      if (editedAt != null) 'edited_at': editedAt!.toUtc().toIso8601String(),
      if (deletedAt != null) 'deleted_at': deletedAt!.toUtc().toIso8601String(),
      if (createdAt != null) 'created_at': createdAt!.toUtc().toIso8601String(),
      'is_pinned': isPinned,
      if (threadParentId != null) 'thread_parent_id': threadParentId,
    };
  }

  static VoiceMessageKind _messageKindFromWire(String? raw) {
    return switch (raw) {
      'system' => VoiceMessageKind.system,
      'forward' => VoiceMessageKind.forward,
      'regular' => VoiceMessageKind.regular,
      _ => VoiceMessageKind.unknown,
    };
  }

  static String _messageKindToWire(VoiceMessageKind kind) {
    return switch (kind) {
      VoiceMessageKind.system => 'system',
      VoiceMessageKind.forward => 'forward',
      VoiceMessageKind.regular => 'regular',
      VoiceMessageKind.unknown => 'unknown',
    };
  }

  static DateTime? parseTimestamp(dynamic raw) {
    if (raw is! String || raw.isEmpty) return null;
    return DateTime.tryParse(raw);
  }

  VoiceMessage copyWith({
    List<MessageReaction>? reactions,
    List<MessageMention>? mentions,
    bool? isPinned,
  }) {
    return VoiceMessage(
      id: id,
      chatId: chatId,
      senderProfileId: senderProfileId,
      content: content,
      attachments: attachments,
      reactions: reactions ?? this.reactions,
      mentions: mentions ?? this.mentions,
      messageKind: messageKind,
      forwardFromId: forwardFromId,
      forwardFromSender: forwardFromSender,
      editedAt: editedAt,
      deletedAt: deletedAt,
      createdAt: createdAt,
      isPinned: isPinned ?? this.isPinned,
    );
  }
}

class MessageAttachment {
  const MessageAttachment({
    required this.fileId,
    required this.type,
    this.url,
    this.previewUrl,
    this.name,
    this.sizeBytes,
  });

  final String fileId;
  final String type;
  final String? url;
  final String? previewUrl;
  final String? name;
  final int? sizeBytes;

  bool get isImage => type == 'image';

  Map<String, dynamic> toJson() {
    return {
      'file_id': fileId,
      'type': type,
      if (url != null && url!.isNotEmpty) 'url': url,
      if (previewUrl != null && previewUrl!.isNotEmpty)
        'preview_url': previewUrl,
      if (name != null && name!.isNotEmpty) 'name': name,
      if (sizeBytes != null) 'size_bytes': sizeBytes,
    };
  }

  factory MessageAttachment.fromJson(Map<String, dynamic> json) {
    return MessageAttachment(
      fileId: json['file_id'] as String? ?? '',
      type: json['type'] as String? ?? 'other',
      url: json['url'] as String?,
      previewUrl: json['preview_url'] as String?,
      name: json['name'] as String?,
      sizeBytes: (json['size_bytes'] as num?)?.toInt(),
    );
  }

  static List<MessageAttachment> listFromWire(dynamic raw) {
    Object? decoded = raw;
    if (raw is String && raw.trim().isNotEmpty) {
      try {
        decoded = jsonDecode(raw);
      } catch (_) {
        decoded = const [];
      }
    }
    if (decoded is! List) return const [];
    return decoded
        .whereType<Map<String, dynamic>>()
        .map(MessageAttachment.fromJson)
        .where((a) => a.fileId.isNotEmpty)
        .toList(growable: false);
  }
}

class MessageListData {
  const MessageListData({
    required this.messages,
    this.nextCursor,
    this.hasMore = false,
  });

  final List<VoiceMessage> messages;
  final String? nextCursor;
  final bool hasMore;

  factory MessageListData.fromJson(Map<String, dynamic> json) {
    final list = json['message_list'] as Map<String, dynamic>? ?? json;
    final raw = list['messages'] as List<dynamic>? ?? const [];
    return MessageListData(
      messages: raw
          .whereType<Map<String, dynamic>>()
          .map(VoiceMessage.fromJson)
          .toList(growable: false),
      nextCursor: list['next_cursor'] as String?,
      hasMore: list['has_more'] as bool? ?? false,
    );
  }
}

class ReadStateData {
  const ReadStateData({
    required this.chatId,
    required this.profileId,
    required this.lastReadMessageId,
  });

  final String chatId;
  final String profileId;
  final String lastReadMessageId;

  factory ReadStateData.fromJson(Map<String, dynamic> json) {
    final state = json['read_state'] as Map<String, dynamic>? ?? json;
    final chat = state['chat'] as Map<String, dynamic>? ?? {};
    return ReadStateData(
      chatId: chat['id'] as String? ?? '',
      profileId: state['profile_id'] as String? ?? '',
      lastReadMessageId: state['last_read_message_id'] as String? ?? '',
    );
  }
}

/// HTTP client for Messaging routes (`/api/v1/messages/**`).
class VoiceMessagesClient {
  VoiceMessagesClient({required GatewayHttpClient gateway})
    : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{'chat_id': chatId};
    if (afterMessageId != null && afterMessageId.isNotEmpty) {
      params['after_message_id'] = afterMessageId;
    }
    if (beforeMessageId != null && beforeMessageId.isNotEmpty) {
      params['before_message_id'] = beforeMessageId;
    }
    if (lastMessageId != null && lastMessageId.isNotEmpty) {
      params['last_message_id'] = lastMessageId;
    }
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';

    final uri = _gateway.replace(path: '/api/v1/messages', queryParameters: params);
    final result = await _gateway.getProto(
      uri,
      authorization: authorization,
      createEmpty: messaging_pb.GetMessagesResponse.create,
    );
    return _map(
      result,
      (data) => messageListFromProto(
        data.hasMessageList()
            ? data.messageList
            : messaging_pb.MessageList(),
      ),
    );
  }

  Future<MessagesApiResult<MessageListData>> getThreadMessages({
    required String authorization,
    required String chatId,
    required String threadParentId,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{
      'chat_id': chatId,
      'thread_parent_id': threadParentId,
    };
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';

    final uri = _gateway.replace(
      path: '/api/v1/messages/thread',
      queryParameters: params,
    );
    final result = await _gateway.getProto(
      uri,
      authorization: authorization,
      createEmpty: messaging_pb.GetThreadMessagesResponse.create,
    );
    return _map(
      result,
      (data) => messageListFromProto(
        data.hasMessageList()
            ? data.messageList
            : messaging_pb.MessageList(),
      ),
    );
  }

  Future<MessagesApiResult<VoiceMessage>> sendMessage({
    required String authorization,
    required String chatId,
    required String content,
    List<MessageAttachment> attachments = const [],
    List<MessageMention> mentions = const [],
    String? clientMessageId,
    String? threadParentId,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/messages/send'),
      authorization: authorization,
      body: sendMessageRequestToProto(
        chatId: chatId,
        content: content,
        attachments: attachments,
        mentions: mentions,
        clientMessageId: clientMessageId,
        threadParentId: threadParentId,
      ),
      createEmpty: messaging_pb.SendMessageResponse.create,
    );
    return _map(result, (data) => voiceMessageFromProto(data.message));
  }

  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/messages/read'),
      authorization: authorization,
      body: markReadRequestToProto(
        chatId: chatId,
        lastReadMessageId: lastReadMessageId,
      ),
      createEmpty: messaging_pb.MarkReadResponse.create,
      allowNoContent: true,
    );
    return _mapEmpty(result);
  }

  Future<MessagesApiResult<VoiceMessage>> editMessage({
    required String authorization,
    required String messageId,
    required String content,
  }) async {
    final result = await _gateway.patchProto(
      uri: _gateway.resolve('/api/v1/messages/$messageId'),
      authorization: authorization,
      body: editMessageRequestToProto(messageId: messageId, content: content),
      createEmpty: messaging_pb.EditMessageResponse.create,
    );
    return _map(result, (data) => voiceMessageFromProto(data.message));
  }

  Future<MessagesApiResult<void>> deleteMessage({
    required String authorization,
    required String messageId,
    String scope = 'everyone',
  }) async {
    final uri = _gateway.replace(
      path: '/api/v1/messages/$messageId',
      queryParameters: {'scope': scope},
    );
    final result = await _gateway.deleteEmpty(
      uri: uri,
      authorization: authorization,
    );
    return _mapEmpty(result);
  }

  /// Phase 4 emoji reaction — docs/features/text-chat.md.
  Future<MessagesApiResult<void>> addReaction({
    required String authorization,
    required String messageId,
    required String emoji,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/messages/$messageId/reactions'),
      authorization: authorization,
      body: messaging_pb.AddReactionRequest(emoji: emoji),
      createEmpty: messaging_pb.AddReactionResponse.create,
      allowNoContent: true,
    );
    return _mapEmpty(result);
  }

  /// Phase 4 emoji reaction toggle-off.
  Future<MessagesApiResult<void>> removeReaction({
    required String authorization,
    required String messageId,
    required String emoji,
  }) async {
    final uri = _gateway.replace(
      path: '/api/v1/messages/$messageId/reactions',
      queryParameters: {'emoji': emoji},
    );
    final result = await _gateway.deleteEmpty(
      uri: uri,
      authorization: authorization,
    );
    return _mapEmpty(result);
  }

  /// Phase 6 pin message — docs/microservices/messaging-service.md.
  Future<MessagesApiResult<void>> pinMessage({
    required String authorization,
    required String messageId,
    required String chatId,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/messages/$messageId/pin'),
      authorization: authorization,
      body: pinMessageRequestToProto(chatId: chatId, messageId: messageId),
      createEmpty: messaging_pb.PinMessageResponse.create,
      allowNoContent: true,
    );
    return _mapEmpty(result);
  }

  /// Phase 6 unpin message.
  Future<MessagesApiResult<void>> unpinMessage({
    required String authorization,
    required String messageId,
    required String chatId,
  }) async {
    final uri = _gateway.replace(
      path: '/api/v1/messages/$messageId/pin',
      queryParameters: {'chat_id': chatId},
    );
    final result = await _gateway.deleteEmpty(
      uri: uri,
      authorization: authorization,
    );
    return _mapEmpty(result);
  }

  /// Phase 6 list pinned messages for a chat.
  Future<MessagesApiResult<MessageListData>> getPinnedMessages({
    required String authorization,
    required String chatId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/chats/$chatId/pinned-messages'),
      authorization: authorization,
      createEmpty: messaging_pb.GetPinnedMessagesResponse.create,
    );
    return _map(
      result,
      (data) => messageListFromProto(
        data.hasMessageList()
            ? data.messageList
            : messaging_pb.MessageList(),
      ),
    );
  }

  /// Phase 4 forward with attribution — docs/features/forward-messages.md.
  Future<MessagesApiResult<VoiceMessage>> forwardMessage({
    required String authorization,
    required String sourceMessageId,
    required String targetChatId,
    String? commentary,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/messages/forward'),
      authorization: authorization,
      body: forwardMessageRequestToProto(
        sourceMessageId: sourceMessageId,
        targetChatId: targetChatId,
        commentary: commentary,
      ),
      createEmpty: messaging_pb.ForwardMessageResponse.create,
    );
    return _map(result, (data) => voiceMessageFromProto(data.message));
  }

  Future<MessagesApiResult<ReadStateData>> getReadState({
    required String authorization,
    required String chatId,
  }) async {
    final uri = _gateway.replace(
      path: '/api/v1/messages/read-state',
      queryParameters: {'chat_id': chatId},
    );
    final result = await _gateway.getProto(
      uri,
      authorization: authorization,
      createEmpty: messaging_pb.GetReadStateResponse.create,
    );
    return _map(
      result,
      (data) => readStateFromProto(
        data.hasReadState() ? data.readState : messaging_pb.ReadState(),
      ),
    );
  }

  MessagesApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => MessagesApiOk(parse(data)),
      GatewayHttpFailure(:final error) => MessagesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  MessagesApiResult<void> _mapEmpty(GatewayHttpResult<dynamic> result) {
    return switch (result) {
      GatewayHttpOk() => const MessagesApiOk(null),
      GatewayHttpFailure(:final error) => MessagesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
