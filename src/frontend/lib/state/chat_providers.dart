import 'dart:async';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:meta/meta.dart';

import '../backend/api_errors.dart';
import '../backend/chats_client.dart';
import '../backend/files_client.dart';
import '../backend/gateway_request_id.dart';
import '../backend/messaging_read_sync.dart';
import '../backend/messages_client.dart';
import '../backend/realtime_client.dart';
import 'auth_providers.dart';
import 'gateway_providers.dart';

final voiceChatsClientProvider = Provider<VoiceChatsClient>((ref) {
  return VoiceChatsClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final voiceMessagesClientProvider = Provider<VoiceMessagesClient>((ref) {
  return VoiceMessagesClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final voiceFilesClientProvider = Provider<VoiceFilesClient>((ref) {
  return VoiceFilesClient(gateway: ref.watch(gatewayHttpClientProvider));
});

/// Resolves a presigned GET URL for chat attachment display.
final fileAttachmentUrlProvider = FutureProvider.family<String?, String>((
  ref,
  fileId,
) async {
  if (fileId.isEmpty) return null;
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) return null;
  final result = await ref
      .read(voiceFilesClientProvider)
      .getFileUrl(authorization: auth, fileId: fileId);
  return switch (result) {
    FilesApiOk(:final data) => data.isEmpty ? null : data,
    FilesApiFailure() => null,
  };
});

/// Active DM chat id in the main column, or null.
final selectedChatIdProvider = StateProvider<String?>((ref) => null);

/// Peer profile id per DM chat id (filled when opening DM from profile).
final dmPeerProfileByChatIdProvider = StateProvider<Map<String, String>>(
  (ref) => {},
);

/// Resolves the other participant in a DM for list/room/call UI.
String? resolveDmPeerProfileId({
  required ChatListItem item,
  String? knownPeerId,
  String? activeProfileId,
}) {
  if (knownPeerId != null && knownPeerId.isNotEmpty) return knownPeerId;
  final fromList = item.dmPeerProfileId;
  if (fromList != null && fromList.isNotEmpty) return fromList;
  if (!item.chat.isDm || activeProfileId == null) return null;
  final creator = item.chat.creatorProfileId;
  if (creator.isEmpty || creator == activeProfileId) return null;
  return creator;
}

/// Fallback when list metadata has no peer (e.g. caller created the DM).
String? inferDmPeerFromMessages(
  Iterable<VoiceMessage> messages,
  String? activeProfileId,
) {
  if (activeProfileId == null) return null;
  for (final msg in messages) {
    final sender = msg.senderProfileId;
    if (sender != activeProfileId) return sender;
  }
  return null;
}

String? resolveDmPeerForChatId({
  required String chatId,
  required Map<String, String> knownPeers,
  required Iterable<ChatListItem> listItems,
  required String? activeProfileId,
  Iterable<VoiceMessage> messages = const [],
}) {
  final cached = knownPeers[chatId];
  if (cached != null && cached.isNotEmpty) return cached;
  for (final item in listItems) {
    if (item.chatId != chatId) continue;
    final fromList = resolveDmPeerProfileId(
      item: item,
      knownPeerId: null,
      activeProfileId: activeProfileId,
    );
    if (fromList != null) return fromList;
    break;
  }
  return inferDmPeerFromMessages(messages, activeProfileId);
}

final _chatListRefreshTokenProvider = StateProvider<int>((ref) => 0);
final chatInboxProvider = StateProvider<String>((ref) => 'main');

class ChatListState {
  const ChatListState({
    this.items = const [],
    this.nextCursor,
    this.isLoading = false,
    this.isLoadingMore = false,
    this.errorMessage,
    this.errorStatusCode,
  });

  final List<ChatListItem> items;
  final String? nextCursor;
  final bool isLoading;
  final bool isLoadingMore;
  final String? errorMessage;
  final int? errorStatusCode;

  bool get hasMore => nextCursor != null && nextCursor!.isNotEmpty;

  ChatListState copyWith({
    List<ChatListItem>? items,
    String? nextCursor,
    bool clearNextCursor = false,
    bool? isLoading,
    bool? isLoadingMore,
    String? errorMessage,
    int? errorStatusCode,
    bool clearError = false,
  }) {
    return ChatListState(
      items: items ?? this.items,
      nextCursor: clearNextCursor ? null : (nextCursor ?? this.nextCursor),
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      errorStatusCode: clearError
          ? null
          : (errorStatusCode ?? this.errorStatusCode),
    );
  }
}

class ChatListController extends StateNotifier<ChatListState> {
  ChatListController(this._ref) : super(const ChatListState()) {
    _authSub = _ref.listen<AuthState>(
      authControllerProvider,
      _onAuthStateChanged,
      fireImmediately: true,
    );
  }

  final Ref _ref;
  ProviderSubscription<AuthState>? _authSub;

  @override
  void dispose() {
    _authSub?.close();
    super.dispose();
  }

  void _onAuthStateChanged(AuthState? previous, AuthState next) {
    if (!next.isAuthenticated) {
      if (previous?.isAuthenticated ?? false) {
        state = const ChatListState();
      }
      return;
    }
    if (next.isRestoring) return;

    final becameAuthenticated =
        next.isAuthenticated && !(previous?.isAuthenticated ?? false);
    final restoreFinished =
        (previous?.isRestoring ?? false) && !next.isRestoring;
    if (becameAuthenticated || restoreFinished) {
      unawaited(loadInitial());
    }
  }

  Future<void> loadInitial() async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    state = state.copyWith(isLoading: true, clearError: true);
    final result = await _ref
        .read(voiceChatsClientProvider)
        .listChats(authorization: auth, inbox: _ref.read(chatInboxProvider));
    if (!mounted) return;
    switch (result) {
      case ChatsApiOk(:final data):
        _syncDmPeersFromList(data.items);
        state = ChatListState(items: data.items, nextCursor: data.nextCursor);
      case ChatsApiFailure(:final message, :final statusCode):
        state = state.copyWith(
          isLoading: false,
          errorMessage: message,
          errorStatusCode: statusCode,
          clearNextCursor: true,
        );
    }
  }

  Future<void> loadMore() async {
    final cursor = state.nextCursor;
    if (cursor == null || cursor.isEmpty || state.isLoadingMore) return;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    state = state.copyWith(isLoadingMore: true, clearError: true);
    final result = await _ref
        .read(voiceChatsClientProvider)
        .listChats(
          authorization: auth,
          cursor: cursor,
          inbox: _ref.read(chatInboxProvider),
        );
    if (!mounted) return;
    switch (result) {
      case ChatsApiOk(:final data):
        _syncDmPeersFromList(data.items);
        state = state.copyWith(
          items: _mergeChatItems(state.items, data.items),
          nextCursor: data.nextCursor,
          clearNextCursor: data.nextCursor == null,
          isLoadingMore: false,
          clearError: true,
        );
      case ChatsApiFailure(:final message, :final statusCode):
        state = state.copyWith(
          isLoadingMore: false,
          errorMessage: message,
          errorStatusCode: statusCode,
        );
    }
  }

  void _syncDmPeersFromList(Iterable<ChatListItem> items) {
    final peers = Map<String, String>.from(
      _ref.read(dmPeerProfileByChatIdProvider),
    );
    final activeId = _ref.read(authControllerProvider).activeProfileId;
    var changed = false;
    for (final item in items) {
      final peerId = resolveDmPeerProfileId(
        item: item,
        knownPeerId: peers[item.chatId],
        activeProfileId: activeId,
      );
      if (peerId == null) continue;
      if (peers[item.chatId] != peerId) {
        peers[item.chatId] = peerId;
        changed = true;
      }
    }
    if (changed) {
      _ref.read(dmPeerProfileByChatIdProvider.notifier).state = peers;
    }
  }

  Future<void> setInbox(String inbox) async {
    _ref.read(chatInboxProvider.notifier).state = inbox;
    await loadInitial();
  }

  Future<String?> acceptRequest(String chatId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .acceptDmRequest(authorization: auth, chatId: chatId);
    return switch (result) {
      ChatsApiOk<void>() => await _afterRequestAction(),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> declineRequest(String chatId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .declineDmRequest(authorization: auth, chatId: chatId);
    return switch (result) {
      ChatsApiOk<void>() => await _afterRequestAction(),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> _afterRequestAction() async {
    await loadInitial();
    _invalidateChatLists(_ref);
    return null;
  }
}

List<ChatListItem> _mergeChatItems(
  Iterable<ChatListItem> current,
  Iterable<ChatListItem> incoming,
) {
  final byId = <String, ChatListItem>{};
  for (final item in current) {
    byId[item.chatId] = item;
  }
  for (final item in incoming) {
    byId[item.chatId] = item;
  }
  return byId.values.toList();
}

final chatListControllerProvider =
    StateNotifierProvider<ChatListController, ChatListState>((ref) {
      ref.watch(_chatListRefreshTokenProvider);
      return ChatListController(ref);
    });

final chatListProvider = FutureProvider<ChatListData>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .watch(voiceChatsClientProvider)
      .listChats(authorization: auth);
  return switch (result) {
    ChatsApiOk(:final data) => data,
    ChatsApiFailure(:final statusCode)
        when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    ChatsApiFailure(:final errorCode, :final statusCode, :final message)
        when isNotFoundError(errorCode, statusCode) =>
      throw Exception(message),
    ChatsApiFailure(:final message) => throw Exception(message),
  };
});

class ChatRoomState {
  const ChatRoomState({
    this.messages = const [],
    this.isLoading = false,
    this.isSending = false,
    this.isLoadingOlder = false,
    this.errorMessage,
    this.realtimeStatus = RealtimeLinkStatus.disconnected,
    this.nextCursor,
    this.hasMore = false,
    this.typingProfileIds = const {},
    this.deliveredMessageIds = const {},
    this.readMessageIds = const {},
  });

  final List<VoiceMessage> messages;
  final bool isLoading;
  final bool isSending;
  final bool isLoadingOlder;
  final String? errorMessage;
  final RealtimeLinkStatus realtimeStatus;
  final String? nextCursor;
  final bool hasMore;
  final Set<String> typingProfileIds;
  final Set<String> deliveredMessageIds;
  final Set<String> readMessageIds;

  String? get lastMessageId => messages.isEmpty ? null : messages.last.id;

  ChatRoomState copyWith({
    List<VoiceMessage>? messages,
    bool? isLoading,
    bool? isSending,
    bool? isLoadingOlder,
    String? errorMessage,
    bool clearError = false,
    RealtimeLinkStatus? realtimeStatus,
    String? nextCursor,
    bool clearNextCursor = false,
    bool? hasMore,
    Set<String>? typingProfileIds,
    Set<String>? deliveredMessageIds,
    Set<String>? readMessageIds,
  }) {
    return ChatRoomState(
      messages: messages ?? this.messages,
      isLoading: isLoading ?? this.isLoading,
      isSending: isSending ?? this.isSending,
      isLoadingOlder: isLoadingOlder ?? this.isLoadingOlder,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      realtimeStatus: realtimeStatus ?? this.realtimeStatus,
      nextCursor: clearNextCursor ? null : (nextCursor ?? this.nextCursor),
      hasMore: hasMore ?? this.hasMore,
      typingProfileIds: typingProfileIds ?? this.typingProfileIds,
      deliveredMessageIds: deliveredMessageIds ?? this.deliveredMessageIds,
      readMessageIds: readMessageIds ?? this.readMessageIds,
    );
  }
}

enum RealtimeLinkStatus { disconnected, connecting, connected, reconnecting }

class ChatRoomController extends StateNotifier<ChatRoomState> {
  ChatRoomController(this._ref, this.chatId) : super(const ChatRoomState()) {
    _realtimeSub = _ref.listen<RealtimeLinkStatus>(realtimeLinkStatusProvider, (
      _,
      next,
    ) {
      final prev = state.realtimeStatus;
      state = state.copyWith(realtimeStatus: next);
      if (prev == RealtimeLinkStatus.reconnecting &&
          next == RealtimeLinkStatus.connected) {
        unawaited(_catchUpAfterReconnect());
      }
    });
    _eventSub = _ref.listen<AsyncValue<RealtimeFrame>>(realtimeEventProvider, (
      _,
      next,
    ) {
      next.whenData((frame) {
        if (frame.op == 'message_create') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId == this.chatId) {
            final senderProfileId = frame.data?['sender_profile_id'] as String?;
            final messageId = frame.data?['message_id'] as String?;
            final activeProfile = _ref
                .read(authControllerProvider)
                .activeProfileId;
            if (senderProfileId != null &&
                senderProfileId != activeProfile &&
                messageId != null) {
              _ref
                  .read(realtimeHubProvider)
                  .deliveryAck(
                    chatId: this.chatId,
                    messageId: messageId,
                    senderProfileId: senderProfileId,
                  );
            }
            unawaited(_catchUpAfterEvent());
          }
        } else if (frame.op == 'mark_read') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId == this.chatId) {
            _ref.invalidate(chatListProvider);
          }
        } else if (frame.op == 'typing') {
          final chatId = frame.data?['chat_id'] as String?;
          final profileId = frame.data?['profile_id'] as String?;
          final kind = frame.data?['kind'] as String?;
          final activeProfile = _ref
              .read(authControllerProvider)
              .activeProfileId;
          if (chatId == this.chatId &&
              profileId != null &&
              profileId != activeProfile) {
            final nextTyping = {...state.typingProfileIds};
            if (kind == 'stop') {
              nextTyping.remove(profileId);
            } else {
              nextTyping.add(profileId);
            }
            state = state.copyWith(typingProfileIds: nextTyping);
          }
        } else if (frame.op == 'message_delivered') {
          final chatId = frame.data?['chat_id'] as String?;
          final messageId = frame.data?['message_id'] as String?;
          if (chatId == this.chatId && messageId != null) {
            state = state.copyWith(
              deliveredMessageIds: {...state.deliveredMessageIds, messageId},
            );
          }
        } else if (frame.op == 'message_read') {
          final chatId = frame.data?['chat_id'] as String?;
          final messageId = frame.data?['message_id'] as String?;
          if (chatId == this.chatId && messageId != null) {
            state = state.copyWith(
              deliveredMessageIds: {...state.deliveredMessageIds, messageId},
              readMessageIds: {...state.readMessageIds, messageId},
            );
          }
        } else if (frame.op == 'message_update' ||
            frame.op == 'message_delete') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId == this.chatId) {
            unawaited(loadInitial());
          }
        }
      });
    });
    unawaited(loadInitial());
    _ref.read(realtimeHubProvider).ensureSubscribed(chatId);
  }

  final Ref _ref;
  final String chatId;
  ProviderSubscription<RealtimeLinkStatus>? _realtimeSub;
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _eventSub;
  String? _lastMarkedReadMessageId;

  @override
  void dispose() {
    _realtimeSub?.close();
    _eventSub?.close();
    super.dispose();
  }

  Future<void> loadInitial() async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    state = state.copyWith(isLoading: true, clearError: true);
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .getMessages(authorization: auth, chatId: chatId);
    if (!mounted) return;
    switch (result) {
      case MessagesApiOk(:final data):
        state = state.copyWith(
          messages: _sortMessages(data.messages),
          isLoading: false,
          nextCursor: data.nextCursor,
          clearNextCursor: data.nextCursor == null,
          hasMore: data.hasMore && data.nextCursor != null,
          clearError: true,
        );
        unawaited(_markLatestRead());
      case MessagesApiFailure(:final message):
        state = state.copyWith(isLoading: false, errorMessage: message);
    }
  }

  Future<void> _catchUpAfterReconnect() async {
    final lastId = state.lastMessageId;
    if (lastId == null) {
      await loadInitial();
      return;
    }
    await _fetchDelta(lastMessageId: lastId);
  }

  Future<void> _catchUpAfterEvent() async {
    final lastId = state.lastMessageId;
    if (lastId == null) {
      await loadInitial();
      return;
    }
    await _fetchDelta(afterMessageId: lastId);
  }

  Future<void> _fetchDelta({
    String? afterMessageId,
    String? lastMessageId,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .getMessages(
          authorization: auth,
          chatId: chatId,
          afterMessageId: afterMessageId,
          lastMessageId: lastMessageId,
        );
    if (result case MessagesApiOk(:final data)) {
      if (!mounted) return;
      if (data.messages.isEmpty) return;
      final merged = [...state.messages];
      for (final m in data.messages) {
        if (!merged.any((x) => x.id == m.id)) {
          merged.add(m);
        }
      }
      state = state.copyWith(messages: _sortMessages(merged), clearError: true);
      unawaited(_markLatestRead());
      _invalidateChatLists(_ref);
    }
  }

  Future<void> loadOlderMessages() async {
    final cursor = state.nextCursor;
    if (cursor == null || cursor.isEmpty || state.isLoadingOlder) return;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    state = state.copyWith(isLoadingOlder: true, clearError: true);
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .getMessages(authorization: auth, chatId: chatId, cursor: cursor);
    if (!mounted) return;
    switch (result) {
      case MessagesApiOk(:final data):
        final merged = [...state.messages];
        for (final m in data.messages) {
          if (!merged.any((x) => x.id == m.id)) {
            merged.add(m);
          }
        }
        state = state.copyWith(
          messages: _sortMessages(merged),
          isLoadingOlder: false,
          nextCursor: data.nextCursor,
          clearNextCursor: data.nextCursor == null,
          hasMore: data.hasMore && data.nextCursor != null,
          clearError: true,
        );
      case MessagesApiFailure(:final message):
        state = state.copyWith(isLoadingOlder: false, errorMessage: message);
    }
  }

  Future<String?> sendMessage(
    String content, {
    List<MessageAttachment> attachments = const [],
  }) async {
    final trimmed = content.trim();
    if (trimmed.isEmpty && attachments.isEmpty) return null;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    state = state.copyWith(isSending: true, clearError: true);
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .sendMessage(
          authorization: auth,
          chatId: chatId,
          content: trimmed,
          attachments: attachments,
        );
    if (!mounted) return null;
    switch (result) {
      case MessagesApiOk(:final data):
        final merged = [...state.messages];
        if (!merged.any((m) => m.id == data.id)) {
          merged.add(data);
        }
        state = state.copyWith(
          messages: _sortMessages(merged),
          isSending: false,
          clearError: true,
        );
        unawaited(_markLatestRead());
        _invalidateChatLists(_ref);
        return null;
      case MessagesApiFailure(:final message):
        state = state.copyWith(isSending: false, errorMessage: message);
        return message;
    }
  }

  Future<void> _markLatestRead() async {
    final lastId = state.lastMessageId;
    if (lastId == null || lastId == _lastMarkedReadMessageId) {
      return;
    }
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final ok = await MessagingReadSync(
      messagesClient: _ref.read(voiceMessagesClientProvider),
      realtimeMarkRead: (cid, mid) =>
          _ref.read(realtimeHubProvider).markRead(cid, mid),
    ).markRead(authorization: auth, chatId: chatId, messageId: lastId);
    if (!mounted) return;
    if (ok) {
      _lastMarkedReadMessageId = lastId;
      _invalidateChatLists(_ref);
    }
  }

  Future<String?> editMessage(String messageId, String content) async {
    final trimmed = content.trim();
    if (trimmed.isEmpty) return null;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .editMessage(
          authorization: auth,
          messageId: messageId,
          content: trimmed,
        );
    if (!mounted) return null;
    return switch (result) {
      MessagesApiOk(:final data) => _replaceMessage(data),
      MessagesApiFailure(:final message) => message,
    };
  }

  Future<String?> deleteMessage(String messageId, {required bool forMe}) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .deleteMessage(
          authorization: auth,
          messageId: messageId,
          scope: forMe ? 'me' : 'everyone',
        );
    if (!mounted) return null;
    switch (result) {
      case MessagesApiOk<void>():
        state = state.copyWith(
          messages: state.messages.where((m) => m.id != messageId).toList(),
          clearError: true,
        );
        _invalidateChatLists(_ref);
        return null;
      case MessagesApiFailure(:final message):
        state = state.copyWith(errorMessage: message);
        return message;
    }
  }

  String? _replaceMessage(VoiceMessage message) {
    state = state.copyWith(
      messages: state.messages
          .map((m) => m.id == message.id ? message : m)
          .toList(),
      clearError: true,
    );
    _invalidateChatLists(_ref);
    return null;
  }
}

void _invalidateChatLists(Ref ref) {
  ref.invalidate(chatListProvider);
  ref.read(_chatListRefreshTokenProvider.notifier).state++;
}

List<VoiceMessage> _sortMessages(Iterable<VoiceMessage> messages) {
  final sorted = [...messages];
  sorted.sort((a, b) {
    final at = a.createdAt ?? DateTime.fromMillisecondsSinceEpoch(0);
    final bt = b.createdAt ?? DateTime.fromMillisecondsSinceEpoch(0);
    final byTime = at.compareTo(bt);
    if (byTime != 0) return byTime;
    return a.id.compareTo(b.id);
  });
  return sorted;
}

final chatRoomControllerProvider = StateNotifierProvider.autoDispose
    .family<ChatRoomController, ChatRoomState, String>((ref, chatId) {
      return ChatRoomController(ref, chatId);
    });

/// Keeps a single Realtime WebSocket while authenticated.
class RealtimeHub {
  RealtimeHub(this._ref);

  final Ref _ref;
  VoiceRealtimeConnection? _connection;
  StreamSubscription<RealtimeFrame>? _frameSub;
  final _eventController = StreamController<RealtimeFrame>.broadcast();
  var _status = RealtimeLinkStatus.disconnected;
  final _subscribedChats = <String>{};
  Timer? _reconnectTimer;
  var _reconnectAttempt = 0;
  var _disposed = false;

  RealtimeLinkStatus get status => _status;
  Stream<RealtimeFrame> get events => _eventController.stream;

  Future<void> ensureConnected() async {
    if (_disposed) return;
    if (_eventController.isClosed || _connection != null) return;
    final auth = _ref.read(authControllerProvider).session;
    final config = _ref.read(gatewayConfigProvider);
    if (auth == null || !config.hasBaseUrl) return;

    _setStatus(RealtimeLinkStatus.connecting);
    final uri = gatewayWebSocketUri(config.baseUrl);
    // Web targets cannot set custom WS headers; Gateway generates X-Request-Id on upgrade.
    final headers = <String, String>{
      'Authorization': auth.authorizationHeader,
      'X-Voice-Profile-Id': auth.activeProfileId,
      if (!kIsWeb) 'X-Request-Id': newGatewayRequestId(),
    };
    final connection = VoiceRealtimeConnection(uri: uri, headers: headers);
    _connection = connection;
    _frameSub = connection.events.listen(
      _onFrame,
      onError: (_) => _scheduleReconnect(),
      onDone: () => _scheduleReconnect(),
    );
    try {
      await connection.connect();
    } catch (_) {
      if (_disposed) {
        await connection.dispose();
        return;
      }
      _scheduleReconnect();
      return;
    }
    if (_disposed) {
      await connection.dispose();
      return;
    }
    _reconnectAttempt = 0;
    for (final chatId in _subscribedChats) {
      connection.sendSubscribe(chatId);
    }
  }

  void ensureSubscribed(String chatId) {
    _subscribedChats.add(chatId);
    _connection?.sendSubscribe(chatId);
    unawaited(ensureConnected());
  }

  /// WS fanout only — call via [MessagingReadSync] after REST mark_read succeeds.
  @visibleForTesting
  void markRead(String chatId, String messageId) {
    _connection?.sendMarkRead(chatId: chatId, messageId: messageId);
  }

  void typingStart(String chatId) {
    _connection?.sendTypingStart(chatId);
  }

  void typingStop(String chatId) {
    _connection?.sendTypingStop(chatId);
  }

  void deliveryAck({
    required String chatId,
    required String messageId,
    required String senderProfileId,
  }) {
    _connection?.sendDeliveryAck(
      chatId: chatId,
      messageId: messageId,
      senderProfileId: senderProfileId,
    );
  }

  void _onFrame(RealtimeFrame frame) {
    if (!_eventController.isClosed) {
      _eventController.add(frame);
    }
    if (frame.op == 'hello') {
      _setStatus(RealtimeLinkStatus.connected);
      // Message catch-up after reconnect is REST-only (see ARCHITECTURE_REQUIREMENTS).
    }
  }

  void _scheduleReconnect() {
    if (_disposed) return;
    if (_ref.read(authControllerProvider).session == null) return;
    _setStatus(RealtimeLinkStatus.reconnecting);
    _reconnectTimer?.cancel();
    final delay = Duration(
      seconds: [1, 2, 4, 8, 16, 30][_reconnectAttempt.clamp(0, 5)],
    );
    _reconnectAttempt++;
    _reconnectTimer = Timer(delay, () async {
      if (_disposed) return;
      await _tearDownConnection();
      await ensureConnected();
    });
  }

  Future<void> _tearDownConnection() async {
    await _frameSub?.cancel();
    _frameSub = null;
    await _connection?.dispose();
    _connection = null;
  }

  Future<void> disconnect() async {
    _reconnectTimer?.cancel();
    _reconnectAttempt = 0;
    _subscribedChats.clear();
    await _tearDownConnection();
    _setStatus(RealtimeLinkStatus.disconnected);
  }

  void _setStatus(RealtimeLinkStatus next) {
    if (_disposed) return;
    _status = next;
    _ref.read(realtimeLinkStatusProvider.notifier).state = next;
  }

  Future<void> dispose() async {
    _disposed = true;
    await disconnect();
    await _eventController.close();
  }
}

/// When false, [RealtimeHub] does not open WebSocket (widget tests).
final realtimeAutoConnectProvider = Provider<bool>((ref) => true);

final realtimeHubProvider = Provider<RealtimeHub>((ref) {
  final hub = RealtimeHub(ref);
  final autoConnect = ref.watch(realtimeAutoConnectProvider);
  ref.onDispose(hub.dispose);
  ref.listen<AuthState>(authControllerProvider, (prev, next) {
    if (!autoConnect) return;
    if (next.isAuthenticated && !(prev?.isAuthenticated ?? false)) {
      unawaited(hub.ensureConnected());
    }
    if (!next.isAuthenticated && (prev?.isAuthenticated ?? false)) {
      unawaited(hub.disconnect());
    }
  });
  if (autoConnect && ref.read(authControllerProvider).isAuthenticated) {
    Future.microtask(hub.ensureConnected);
  }
  return hub;
});

final realtimeLinkStatusProvider = StateProvider<RealtimeLinkStatus>(
  (ref) => RealtimeLinkStatus.disconnected,
);

final realtimeEventProvider = StreamProvider<RealtimeFrame>((ref) {
  final hub = ref.watch(realtimeHubProvider);
  return hub.events;
});

class ChatActions {
  ChatActions(this._ref);

  final Ref _ref;

  Future<String?> openDmWithProfile(String otherProfileId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .createDm(authorization: auth, otherProfileId: otherProfileId);
    return switch (result) {
      ChatsApiOk(:final data) => _selectDmChat(data.id, otherProfileId),
      ChatsApiFailure(:final message) => message,
    };
  }

  /// Creates a standalone group and invites members (min 2 invitees per API).
  Future<String?> createGroupWithMembers({
    required String name,
    required List<String> memberProfileIds,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final createResult = await _ref
        .read(voiceChatsClientProvider)
        .createGroup(authorization: auth, name: name);
    return switch (createResult) {
      ChatsApiFailure(:final message) => message,
      ChatsApiOk(:final data) => _inviteGroupMembers(
        auth: auth,
        chatId: data.id,
        memberProfileIds: memberProfileIds,
      ),
    };
  }

  Future<String?> _inviteGroupMembers({
    required String auth,
    required String chatId,
    required List<String> memberProfileIds,
  }) async {
    final inviteResult = await _ref.read(voiceChatsClientProvider).addGroupMembers(
      authorization: auth,
      chatId: chatId,
      profileIds: memberProfileIds,
    );
    return switch (inviteResult) {
      ChatsApiFailure(:final message) => message,
      ChatsApiOk() => _selectGroupChat(chatId),
    };
  }

  String? _selectGroupChat(String chatId) {
    _ref.read(selectedChatIdProvider.notifier).state = chatId;
    _ref.read(realtimeHubProvider).ensureSubscribed(chatId);
    _invalidateChatLists(_ref);
    return null;
  }

  String? _selectDmChat(String chatId, String peerProfileId) {
    final peers = Map<String, String>.from(
      _ref.read(dmPeerProfileByChatIdProvider),
    );
    peers[chatId] = peerProfileId;
    _ref.read(dmPeerProfileByChatIdProvider.notifier).state = peers;
    _ref.read(selectedChatIdProvider.notifier).state = chatId;
    _invalidateChatLists(_ref);
    return null;
  }

  void selectChat(String chatId) {
    _ref.read(selectedChatIdProvider.notifier).state = chatId;
    _ref.read(realtimeHubProvider).ensureSubscribed(chatId);
    _rememberDmPeerForChat(chatId);
  }

  Future<String?> removeGroupMember({
    required String chatId,
    required String profileId,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref.read(voiceChatsClientProvider).removeGroupMember(
      authorization: auth,
      chatId: chatId,
      profileId: profileId,
    );
    return switch (result) {
      ChatsApiFailure(:final message) => message,
      ChatsApiOk() => () {
        _ref.invalidate(groupMembersProvider(chatId));
        _invalidateChatLists(_ref);
        return null;
      }(),
    };
  }

  Future<String?> leaveGroup(String chatId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref.read(voiceChatsClientProvider).leaveGroup(
      authorization: auth,
      chatId: chatId,
    );
    return switch (result) {
      ChatsApiFailure(:final message) => message,
      ChatsApiOk() => () {
        final selected = _ref.read(selectedChatIdProvider);
        if (selected == chatId) {
          _ref.read(selectedChatIdProvider.notifier).state = null;
        }
        _ref.invalidate(groupMembersProvider(chatId));
        _invalidateChatLists(_ref);
        return null;
      }(),
    };
  }

  void _rememberDmPeerForChat(String chatId) {
    final activeId = _ref.read(authControllerProvider).activeProfileId;
    final listItems = _ref.read(chatListControllerProvider).items;
    final peers = Map<String, String>.from(
      _ref.read(dmPeerProfileByChatIdProvider),
    );
    final peerId = resolveDmPeerForChatId(
      chatId: chatId,
      knownPeers: peers,
      listItems: listItems,
      activeProfileId: activeId,
    );
    if (peerId == null || peers[chatId] == peerId) return;
    peers[chatId] = peerId;
    _ref.read(dmPeerProfileByChatIdProvider.notifier).state = peers;
  }
}

final chatActionsProvider = Provider<ChatActions>((ref) {
  return ChatActions(ref);
});

/// Group member list with roles (`owner` / `member`) from `GET /api/v1/chats/{id}/members`.
final groupMembersProvider = FutureProvider.family<MemberListData, String>((
  ref,
  chatId,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .read(voiceChatsClientProvider)
      .listGroupMembers(authorization: auth, chatId: chatId);
  return switch (result) {
    ChatsApiOk(:final data) => data,
    ChatsApiFailure(:final message) => throw Exception(message),
  };
});
