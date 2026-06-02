import 'dart:convert';

import 'package:http/http.dart' as http;

import 'gateway_config.dart';

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

class VoiceMessage {
  const VoiceMessage({
    required this.id,
    required this.chatId,
    required this.senderProfileId,
    required this.content,
    this.createdAt,
  });

  final String id;
  final String chatId;
  final String senderProfileId;
  final String content;
  final DateTime? createdAt;

  factory VoiceMessage.fromJson(Map<String, dynamic> json) {
    final chat = json['chat'] as Map<String, dynamic>? ?? {};
    return VoiceMessage(
      id: json['id'] as String,
      chatId: chat['id'] as String? ?? '',
      senderProfileId: json['sender_profile_id'] as String? ?? '',
      content: json['content'] as String? ?? '',
      createdAt: VoiceMessage.parseTimestamp(json['created_at']),
    );
  }

  static DateTime? parseTimestamp(dynamic raw) {
    if (raw is! String || raw.isEmpty) return null;
    return DateTime.tryParse(raw);
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
  VoiceMessagesClient({
    required http.Client httpClient,
    required GatewayConfig config,
  }) : _http = httpClient,
       _config = config;

  final http.Client _http;
  final GatewayConfig _config;

  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) async {
    if (!_config.hasBaseUrl) {
      return const MessagesApiFailure(message: kMessagesMissingBaseUrlDetail);
    }
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

    final uri = Uri.parse(
      _config.baseUrl,
    ).replace(path: '/api/v1/messages', queryParameters: params);
    return _get(uri, authorization, (body) {
      final list = body['message_list'] as Map<String, dynamic>? ?? {};
      final raw = list['messages'] as List<dynamic>? ?? [];
      return MessageListData(
        messages: raw
            .map((e) => VoiceMessage.fromJson(e as Map<String, dynamic>))
            .toList(),
        nextCursor: _emptyToNull(list['next_cursor'] as String?),
        hasMore: list['has_more'] as bool? ?? false,
      );
    });
  }

  Future<MessagesApiResult<VoiceMessage>> sendMessage({
    required String authorization,
    required String chatId,
    required String content,
    String? clientMessageId,
  }) async {
    if (!_config.hasBaseUrl) {
      return const MessagesApiFailure(message: kMessagesMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/messages/send');
    final body = <String, dynamic>{
      'chat': {'id': chatId},
      'content': content,
    };
    if (clientMessageId != null) {
      body['client_message_id'] = clientMessageId;
    }
    try {
      final res = await _http.post(
        uri,
        headers: {
          'Authorization': authorization,
          'Content-Type': 'application/json',
        },
        body: jsonEncode(body),
      );
      if (res.statusCode == 200) {
        final decoded = jsonDecode(res.body) as Map<String, dynamic>;
        final msg = decoded['message'] as Map<String, dynamic>;
        return MessagesApiOk(VoiceMessage.fromJson(msg));
      }
      return MessagesApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return MessagesApiFailure(message: '$e');
    }
  }

  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async {
    if (!_config.hasBaseUrl) {
      return const MessagesApiFailure(message: kMessagesMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/messages/read');
    try {
      final res = await _http.post(
        uri,
        headers: {
          'Authorization': authorization,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({
          'chat': {'id': chatId},
          'last_read_message_id': lastReadMessageId,
        }),
      );
      if (res.statusCode == 200) {
        return const MessagesApiOk(null);
      }
      return MessagesApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return MessagesApiFailure(message: '$e');
    }
  }

  Future<MessagesApiResult<ReadStateData>> getReadState({
    required String authorization,
    required String chatId,
  }) async {
    if (!_config.hasBaseUrl) {
      return const MessagesApiFailure(message: kMessagesMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl).replace(
      path: '/api/v1/messages/read-state',
      queryParameters: {'chat_id': chatId},
    );
    return _get(uri, authorization, ReadStateData.fromJson);
  }

  Future<MessagesApiResult<T>> _get<T>(
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
        return MessagesApiOk(parse(decoded));
      }
      return MessagesApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return MessagesApiFailure(message: '$e');
    }
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
