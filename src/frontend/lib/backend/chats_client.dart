import 'dart:convert';

import 'package:http/http.dart' as http;

import 'gateway_config.dart';

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

  bool get isDm => type == 'CHAT_TYPE_DM' || type == '1';

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
  });

  final VoiceChat chat;
  final String? lastMessagePreview;
  final int unreadCount;

  String get chatId => chat.id;
}

class ChatListData {
  const ChatListData({required this.items, this.nextCursor});

  final List<ChatListItem> items;
  final String? nextCursor;
}

/// HTTP client for Chat routes (`/api/v1/chats/**`).
class VoiceChatsClient {
  VoiceChatsClient({
    required http.Client httpClient,
    required GatewayConfig config,
  }) : _http = httpClient,
       _config = config;

  final http.Client _http;
  final GatewayConfig _config;

  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
  }) async {
    if (!_config.hasBaseUrl) {
      return const ChatsApiFailure(message: kChatsMissingBaseUrlDetail);
    }
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = Uri.parse(_config.baseUrl).replace(
      path: '/api/v1/chats',
      queryParameters: params.isEmpty ? null : params,
    );
    return _get(uri, authorization, (body) {
      final list = body['chat_list'] as Map<String, dynamic>? ?? {};
      final rawItems = list['items'] as List<dynamic>? ?? [];
      return ChatListData(
        items: rawItems.map((e) {
          final item = e as Map<String, dynamic>;
          final chatJson = item['chat'] as Map<String, dynamic>;
          return ChatListItem(
            chat: VoiceChat.fromJson(chatJson),
            lastMessagePreview: item['last_message_preview'] as String?,
            unreadCount: _parseInt64(item['unread_count']),
          );
        }).toList(),
        nextCursor: _emptyToNull(list['next_cursor'] as String?),
      );
    });
  }

  Future<ChatsApiResult<VoiceChat>> createDm({
    required String authorization,
    required String otherProfileId,
  }) async {
    if (!_config.hasBaseUrl) {
      return const ChatsApiFailure(message: kChatsMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/chats/dm');
    try {
      final res = await _http.post(
        uri,
        headers: {
          'Authorization': authorization,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({'other_profile_id': otherProfileId}),
      );
      if (res.statusCode == 200) {
        final decoded = jsonDecode(res.body) as Map<String, dynamic>;
        final chat = decoded['chat'] as Map<String, dynamic>;
        return ChatsApiOk(VoiceChat.fromJson(chat));
      }
      return ChatsApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return ChatsApiFailure(message: '$e');
    }
  }

  Future<ChatsApiResult<T>> _get<T>(
    Uri uri,
    String authorization,
    T Function(Map<String, dynamic> body) parse,
  ) async {
    try {
      final res = await _http.get(
        uri,
        headers: {'Authorization': authorization},
      );
      if (res.statusCode == 200) {
        final decoded = jsonDecode(res.body) as Map<String, dynamic>;
        return ChatsApiOk(parse(decoded));
      }
      return ChatsApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return ChatsApiFailure(message: '$e');
    }
  }

  static int _parseInt64(dynamic raw) {
    if (raw == null) return 0;
    if (raw is int) return raw;
    if (raw is String) return int.tryParse(raw) ?? 0;
    if (raw is num) return raw.toInt();
    return 0;
  }

  static String? _emptyToNull(String? value) {
    if (value == null || value.isEmpty) return null;
    return value;
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
