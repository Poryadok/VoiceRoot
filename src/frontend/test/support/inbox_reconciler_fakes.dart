import 'dart:async';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'gateway_test_client.dart';

/// A deterministic page script for reconnect tests.
class InboxChatPageScript {
  const InboxChatPageScript({
    required this.inbox,
    required this.cursor,
    required this.result,
    this.profileId = 'prof-test',
    this.authorization = 'Bearer test-access',
    this.pageSize,
    this.folderId,
    this.manual = false,
  });

  final String inbox;
  final String? cursor;
  final ChatsApiResult<ChatListData> result;
  final String profileId;
  final String authorization;
  final int? pageSize;
  final String? folderId;
  final bool manual;
}

class InboxChatCall {
  InboxChatCall({
    required this.authorization,
    required this.profileId,
    required this.inbox,
    required this.cursor,
    required this.pageSize,
    required this.folderId,
  });

  final String authorization;
  final String? profileId;
  final String? inbox;
  final String? cursor;
  final int? pageSize;
  final String? folderId;
  bool completed = false;
  Completer<ChatsApiResult<ChatListData>>? completer;
}

/// Scripted [VoiceChatsClient] that can resolve pages immediately or manually.
///
/// Calls are matched by the exact `(profile, authorization, inbox, cursor,
/// pageSize, folderId)` key. This makes a test fail if retry restarts at page
/// one, crosses a profile boundary, or silently changes an opaque cursor.
class InboxReconcilerChatsFake extends VoiceChatsClient {
  InboxReconcilerChatsFake({
    this.profileByAuthorization = const {'Bearer test-access': 'prof-test'},
  }) : super(
         gateway: gatewayHttpForTest(
           MockClient((_) async => http.Response('{}', 500)),
         ),
       );

  final Map<String, String> profileByAuthorization;
  final List<InboxChatPageScript> _scripts = [];
  final List<InboxChatCall> calls = [];
  final List<InboxChatCall> unmatchedCalls = [];

  int get pendingScripts => _scripts.length;

  void enqueue(InboxChatPageScript script) => _scripts.add(script);

  InboxChatCall? findCall({
    required String? inbox,
    required String? cursor,
    String? profileId,
    String? authorization,
  }) {
    for (final call in calls.reversed) {
      if (call.inbox != inbox || call.cursor != cursor) continue;
      if (profileId != null && call.profileId != profileId) continue;
      if (authorization != null && call.authorization != authorization)
        continue;
      return call;
    }
    return null;
  }

  Future<void> completeCall(
    InboxChatCall call, {
    required ChatsApiResult<ChatListData> result,
  }) async {
    call.completed = true;
    final completer = call.completer;
    if (completer == null || completer.isCompleted) return;
    completer.complete(result);
    await Future<void>.value();
  }

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
    String? folderId,
  }) {
    final call = InboxChatCall(
      authorization: authorization,
      profileId: profileByAuthorization[authorization],
      inbox: inbox,
      cursor: cursor,
      pageSize: pageSize,
      folderId: folderId,
    );
    calls.add(call);

    final index = _scripts.indexWhere(
      (script) =>
          script.inbox == (inbox ?? 'main') &&
          script.cursor == cursor &&
          script.pageSize == pageSize &&
          script.folderId == folderId &&
          script.profileId == profileByAuthorization[authorization] &&
          script.authorization == authorization,
    );
    if (index < 0) {
      unmatchedCalls.add(call);
      return Future.error(
        StateError(
          'Unexpected ListChats call: ${call.profileId}/${call.authorization}/'
          '${call.inbox}/${call.cursor}/${call.pageSize}/${call.folderId}',
        ),
      );
    }
    final script = _scripts.removeAt(index);
    if (!script.manual) return Future.value(script.result);
    final completer = Completer<ChatsApiResult<ChatListData>>();
    call.completer = completer;
    return completer.future;
  }
}

class InboxMessageCall {
  InboxMessageCall({
    required this.authorization,
    required this.profileId,
    required this.chatId,
    required this.afterMessageId,
    required this.lastMessageId,
    required this.cursor,
  });

  final String authorization;
  final String? profileId;
  final String chatId;
  final String? afterMessageId;
  final String? lastMessageId;
  final String? cursor;
  Completer<MessagesApiResult<MessageListData>>? completer;
}

class _InboxMessagePageScript {
  const _InboxMessagePageScript({required this.data, required this.manual});

  final MessagesApiResult<MessageListData> data;
  final bool manual;
}

/// Messaging fake used to prove the global inbox reconciler never replays
/// history, while still supporting the selected-room REST boundary test.
class InboxReconcilerMessagesFake extends VoiceMessagesClient {
  InboxReconcilerMessagesFake({this.profileByAuthorization = const {}})
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final Map<String, String> profileByAuthorization;
  final List<_InboxMessagePageScript> _pages = [];
  final List<InboxMessageCall> getCalls = [];
  final List<String> markReadChatIds = [];

  void enqueue(MessageListData page, {bool manual = false}) {
    _pages.add(
      _InboxMessagePageScript(data: MessagesApiOk(page), manual: manual),
    );
  }

  Future<void> completeCall(
    InboxMessageCall call, {
    required MessagesApiResult<MessageListData> result,
  }) async {
    final completer = call.completer;
    if (completer == null || completer.isCompleted) return;
    completer.complete(result);
    await Future<void>.value();
  }

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) {
    final call = InboxMessageCall(
      authorization: authorization,
      profileId: profileByAuthorization[authorization],
      chatId: chatId,
      afterMessageId: afterMessageId,
      lastMessageId: lastMessageId,
      cursor: cursor,
    );
    getCalls.add(call);
    if (_pages.isEmpty) {
      return Future.value(const MessagesApiOk(MessageListData(messages: [])));
    }
    final script = _pages.removeAt(0);
    if (!script.manual) return Future.value(script.data);
    final completer = Completer<MessagesApiResult<MessageListData>>();
    call.completer = completer;
    return completer.future;
  }

  @override
  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async {
    markReadChatIds.add(chatId);
    return const MessagesApiOk(null);
  }
}

ChatListItem inboxChatItem(
  String id, {
  String? preview,
  String? inbox,
  String? creatorProfileId,
  int unreadCount = 0,
}) {
  return ChatListItem(
    chat: VoiceChat(
      id: id,
      type: 'CHAT_TYPE_DM',
      creatorProfileId: creatorProfileId ?? 'peer-$id',
    ),
    lastMessagePreview: preview,
    unreadCount: unreadCount,
    inbox: inbox,
  );
}

VoiceMessage inboxMessage(String id, {String chatId = 'selected-chat'}) {
  return VoiceMessage(
    id: id,
    chatId: chatId,
    senderProfileId: 'peer-$chatId',
    content: 'message $id',
    createdAt: DateTime.utc(2024, 1, 1),
  );
}
