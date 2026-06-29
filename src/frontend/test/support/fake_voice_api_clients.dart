import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'gateway_test_client.dart';

/// Stable chats API for widget tests (avoids [MockClient] throws on listChats).
class FakeVoiceChatsClient extends VoiceChatsClient {
  FakeVoiceChatsClient({List<ChatListData> pages = const [], this.dmChatId = 'dm-chat'})
    : _pages = [...pages],
      super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 404)),
        ),
      );

  final List<ChatListData> _pages;
  final String dmChatId;

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
  }) async {
    if (_pages.isEmpty) {
      return const ChatsApiOk(ChatListData(items: []));
    }
    return ChatsApiOk(_pages.removeAt(0));
  }

  @override
  Future<ChatsApiResult<VoiceChat>> createDm({
    required String authorization,
    required String otherProfileId,
  }) async {
    return ChatsApiOk(
      VoiceChat(
        id: dmChatId,
        type: 'CHAT_TYPE_DM',
        creatorProfileId: otherProfileId,
      ),
    );
  }
}

/// Stable messages API for widget tests (empty history by default).
class FakeVoiceMessagesClient extends VoiceMessagesClient {
  FakeVoiceMessagesClient({List<MessageListData> pages = const []})
    : _pages = [...pages],
      super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 404)),
        ),
      );

  final List<MessageListData> _pages;

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) async {
    if (_pages.isEmpty) {
      return const MessagesApiOk(MessageListData(messages: []));
    }
    return MessagesApiOk(_pages.removeAt(0));
  }
}
