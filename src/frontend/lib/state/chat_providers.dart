import 'dart:async';
import 'dart:typed_data';

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
import '../e2e/e2e_exceptions.dart';
import '../e2e/e2e_file_crypto.dart';
import 'auth_providers.dart';
import 'bot_deferred_providers.dart';
import 'connectivity_providers.dart';
import 'gateway_providers.dart';
import 'message_cache_providers.dart';
import 'e2e_providers.dart';
import 'space_providers.dart';

/// Returned by [ChatRoomController.sendMessage] when offline send is blocked.
const String kChatOfflineBlockedError = 'offline_blocked';

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

/// Request to decrypt an E2E file attachment for display.
class E2eAttachmentDecryptRequest {
  const E2eAttachmentDecryptRequest({
    required this.fileId,
    required this.e2eKeyWire,
    required this.senderProfileId,
    required this.chatId,
  });

  final String fileId;
  final String e2eKeyWire;
  final String senderProfileId;
  final String chatId;

  @override
  bool operator ==(Object other) {
    return other is E2eAttachmentDecryptRequest &&
        other.fileId == fileId &&
        other.e2eKeyWire == e2eKeyWire &&
        other.senderProfileId == senderProfileId &&
        other.chatId == chatId;
  }

  @override
  int get hashCode => Object.hash(fileId, e2eKeyWire, senderProfileId, chatId);
}

/// Downloads and decrypts E2E ciphertext blobs for attachment preview.
final e2eDecryptedAttachmentBytesProvider =
    FutureProvider.family<Uint8List?, E2eAttachmentDecryptRequest>((
  ref,
  request,
) async {
  if (request.fileId.isEmpty || request.e2eKeyWire.isEmpty) return null;
  final auth = ref.watch(authorizationHeaderProvider);
  final localProfileId = ref.watch(authControllerProvider).activeProfileId;
  if (auth == null || localProfileId == null || localProfileId.isEmpty) {
    return null;
  }
  final files = ref.read(voiceFilesClientProvider);
  final downloaded = await files.fetchFileBytes(
    authorization: auth,
    fileId: request.fileId,
  );
  if (downloaded is! FilesApiOk<Uint8List>) return null;
  final crypto = const E2eFileCrypto();
  final messageService = ref.read(e2eMessageServiceProvider);
  try {
    return await crypto.decryptBytes(
      ciphertext: downloaded.data,
      keyWire: request.e2eKeyWire,
      messageService: messageService,
      localProfileId: localProfileId,
      peerProfileId: request.senderProfileId,
      authorization: auth,
    );
  } on Object {
    return null;
  }
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
    if (item.chat.isGroup) return null;
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

/// Active reply target per chat (thread parent message id for composer).
final chatReplyTargetProvider =
    StateProvider.family<VoiceMessage?, String>((ref, chatId) => null);

/// Open thread panel parent message id per chat.
final chatActiveThreadProvider =
    StateProvider.family<String?, String>((ref, chatId) => null);

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

  /// Optimistic unread bump when an in-app notification arrives for a background chat.
  void bumpUnread(String chatId, {int delta = 1}) {
    if (delta <= 0) return;
    final index = state.items.indexWhere((item) => item.chatId == chatId);
    if (index < 0) return;
    final item = state.items[index];
    final updated = ChatListItem(
      chat: item.chat,
      lastMessagePreview: item.lastMessagePreview,
      unreadCount: item.unreadCount + delta,
      inbox: item.inbox,
      isStranger: item.isStranger,
      dmPeerProfileId: item.dmPeerProfileId,
    );
    final items = [...state.items];
    items[index] = updated;
    state = state.copyWith(items: items);
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
    this.pinnedMessages = const [],
    this.isOfflineCache = false,
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
  final List<VoiceMessage> pinnedMessages;
  final bool isOfflineCache;

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
    List<VoiceMessage>? pinnedMessages,
    bool? isOfflineCache,
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
      pinnedMessages: pinnedMessages ?? this.pinnedMessages,
      isOfflineCache: isOfflineCache ?? this.isOfflineCache,
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
            if (_ref.read(deferredBotInteractionProvider(this.chatId)) != null) {
              _ref.read(deferredBotInteractionProvider(this.chatId).notifier).clear();
            }
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
        } else if (frame.op == 'mention') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId == this.chatId) {
            unawaited(_catchUpAfterEvent());
          }
        } else if (frame.op == 'reaction_add' || frame.op == 'reaction_remove') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId != this.chatId) return;
          final profileId = frame.data?['profile_id'] as String?;
          final activeProfile = _ref.read(authControllerProvider).activeProfileId;
          if (profileId == activeProfile) return;
          final messageId = frame.data?['message_id'] as String?;
          final emoji = frame.data?['emoji'] as String?;
          if (messageId == null || emoji == null || emoji.isEmpty) return;
          _applyReactionDelta(
            messageId: messageId,
            emoji: emoji,
            add: frame.op == 'reaction_add',
            reactedByMe: false,
          );
        } else if (frame.op == 'message_pinned' ||
            frame.op == 'message_unpinned') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId != this.chatId) return;
          final messageId = frame.data?['message_id'] as String?;
          if (messageId == null) return;
          final pinnedBy = frame.data?['pinned_by'] as String?;
          final unpinnedBy = frame.data?['unpinned_by'] as String?;
          final actor = pinnedBy ?? unpinnedBy;
          final activeProfile = _ref.read(authControllerProvider).activeProfileId;
          if (actor == activeProfile) return;
          _applyPinDelta(
            messageId: messageId,
            pinned: frame.op == 'message_pinned',
          );
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

  bool _isE2eChat() => _ref.read(chatE2eEnabledProvider(chatId));

  String? _dmPeerProfileId() {
    return resolveDmPeerForChatId(
      chatId: chatId,
      knownPeers: _ref.read(dmPeerProfileByChatIdProvider),
      listItems: _ref.read(chatListControllerProvider).items,
      activeProfileId: _activeProfileId(),
      messages: state.messages,
    );
  }

  Future<List<VoiceMessage>> _finalizeMessages(List<VoiceMessage> messages) async {
    if (!_isE2eChat()) return messages;
    final localId = _activeProfileId();
    final peerId = _dmPeerProfileId();
    if (localId == null || peerId == null) return messages;
    final auth = _ref.read(authorizationHeaderProvider);
    return _ref.read(e2eMessageServiceProvider).decryptAllForDisplay(
      messages: messages,
      localProfileId: localId,
      peerProfileId: peerId,
      authorization: auth,
    );
  }

  @override
  void dispose() {
    _realtimeSub?.close();
    _eventSub?.close();
    super.dispose();
  }

  Future<void> loadInitial() async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    state = state.copyWith(isLoading: true, clearError: true, isOfflineCache: false);
    if (_isDeviceOffline()) {
      final served = await _serveCachedMessages();
      if (served) return;
    }
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .getMessages(authorization: auth, chatId: chatId);
    if (!mounted) return;
    switch (result) {
      case MessagesApiOk(:final data):
        final sorted = await _finalizeMessages(_sortMessages(data.messages));
        state = state.copyWith(
          messages: sorted,
          isLoading: false,
          isOfflineCache: false,
          nextCursor: data.nextCursor,
          clearNextCursor: data.nextCursor == null,
          hasMore: data.hasMore && data.nextCursor != null,
          clearError: true,
        );
        unawaited(_writeCache(sorted));
        unawaited(_markLatestRead());
        unawaited(_refreshPinnedMessages(auth));
      case MessagesApiFailure(:final message, :final errorCode):
        if (errorCode == 'network_error') {
          final served = await _serveCachedMessages();
          if (served) return;
        }
        state = state.copyWith(
          isLoading: false,
          errorMessage: message,
          isOfflineCache: false,
        );
    }
  }

  Future<void> _refreshPinnedMessages(String auth) async {
    final pinned = await _ref
        .read(voiceMessagesClientProvider)
        .getPinnedMessages(authorization: auth, chatId: chatId);
    if (!mounted) return;
    if (pinned case MessagesApiOk(:final data)) {
      state = state.copyWith(pinnedMessages: data.messages);
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
      final sorted = await _finalizeMessages(_sortMessages(merged));
      state = state.copyWith(
        messages: sorted,
        clearError: true,
        isOfflineCache: false,
      );
      unawaited(_writeCache(sorted));
      unawaited(_markLatestRead());
      _invalidateChatLists(_ref);
    }
  }

  Future<void> loadOlderMessages() async {
    if (_isDeviceOffline()) return;
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
        final sorted = await _finalizeMessages(_sortMessages(merged));
        state = state.copyWith(
          messages: sorted,
          isLoadingOlder: false,
          isOfflineCache: false,
          nextCursor: data.nextCursor,
          clearNextCursor: data.nextCursor == null,
          hasMore: data.hasMore && data.nextCursor != null,
          clearError: true,
        );
        unawaited(_writeCache(sorted));
      case MessagesApiFailure(:final message):
        state = state.copyWith(isLoadingOlder: false, errorMessage: message);
    }
  }

  Future<String?> sendMessage(
    String content, {
    List<MessageAttachment> attachments = const [],
    List<MessageMention> mentions = const [],
    String? threadParentId,
  }) async {
    final trimmed = content.trim();
    if (trimmed.isEmpty && attachments.isEmpty) return null;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    if (_isDeviceOffline()) {
      return kChatOfflineBlockedError;
    }

    var outbound = trimmed;
    var isE2e = false;
    if (_isE2eChat()) {
      final peerId = _dmPeerProfileId();
      final localId = _activeProfileId();
      if (peerId != null && localId != null && outbound.isNotEmpty) {
        try {
          outbound = await _ref.read(e2eMessageServiceProvider).encryptOutgoing(
            localProfileId: localId,
            peerProfileId: peerId,
            plaintext: outbound,
            authorization: auth,
            chatId: chatId,
          );
          isE2e = true;
        } on E2eEncryptException catch (e) {
          if (!mounted) return e.message;
          state = state.copyWith(
            isSending: false,
            errorMessage: e.message,
          );
          return e.message;
        }
      }
    }

    state = state.copyWith(isSending: true, clearError: true);
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .sendMessage(
          authorization: auth,
          chatId: chatId,
          content: outbound,
          attachments: attachments,
          mentions: mentions,
          threadParentId: threadParentId,
          isE2e: isE2e,
        );
    if (!mounted) return null;
    switch (result) {
      case MessagesApiOk(:final data):
        final merged = [...state.messages];
        if (!merged.any((m) => m.id == data.id)) {
          merged.add(data);
        }
        final sorted = await _finalizeMessages(_sortMessages(merged));
        state = state.copyWith(
          messages: sorted,
          isSending: false,
          isOfflineCache: false,
          clearError: true,
        );
        unawaited(_writeCache(sorted));
        unawaited(_markLatestRead());
        _invalidateChatLists(_ref);
        return null;
      case MessagesApiFailure(:final message):
        state = state.copyWith(isSending: false, errorMessage: message);
        return message;
    }
  }

  bool _isDeviceOffline() => _ref.read(isDeviceOfflineProvider);

  String? _activeProfileId() =>
      _ref.read(authControllerProvider).activeProfileId;

  Future<bool> _serveCachedMessages() async {
    final profileId = _activeProfileId();
    if (profileId == null) return false;
    final cached = await _ref
        .read(messageCacheStoreProvider)
        .getMessages(profileId: profileId, chatId: chatId);
    if (!mounted) return false;
    if (cached.isEmpty) return false;
    final sorted = await _finalizeMessages(_sortMessages(cached));
    state = state.copyWith(
      messages: sorted,
      isLoading: false,
      isOfflineCache: true,
      clearError: true,
      hasMore: false,
      clearNextCursor: true,
    );
    return true;
  }

  Future<void> _writeCache(List<VoiceMessage> messages) async {
    final profileId = _activeProfileId();
    if (profileId == null || messages.isEmpty) return;
    await _ref.read(messageCacheStoreProvider).replaceChatMessages(
      profileId: profileId,
      chatId: chatId,
      messages: messages,
    );
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

    var outbound = trimmed;
    VoiceMessage? existing;
    for (final message in state.messages) {
      if (message.id == messageId) {
        existing = message;
        break;
      }
    }
    if (existing != null && existing.isE2e) {
      final peerId = _dmPeerProfileId();
      final localId = _activeProfileId();
      if (peerId != null && localId != null) {
        try {
          outbound = await _ref.read(e2eMessageServiceProvider).encryptOutgoing(
            localProfileId: localId,
            peerProfileId: peerId,
            plaintext: outbound,
            authorization: auth,
            chatId: chatId,
          );
        } on E2eEncryptException catch (e) {
          return e.message;
        }
      }
    }

    final result = await _ref
        .read(voiceMessagesClientProvider)
        .editMessage(
          authorization: auth,
          messageId: messageId,
          content: outbound,
        );
    if (!mounted) return null;
    return switch (result) {
      MessagesApiOk(:final data) => _replaceMessage(data),
      MessagesApiFailure(:final message) => message,
    };
  }

  Future<String?> toggleReaction(
    String messageId,
    String emoji, {
    required bool currentlyReacted,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    _applyReactionDelta(
      messageId: messageId,
      emoji: emoji,
      add: !currentlyReacted,
      reactedByMe: !currentlyReacted,
    );
    final client = _ref.read(voiceMessagesClientProvider);
    final result = currentlyReacted
        ? await client.removeReaction(
            authorization: auth,
            messageId: messageId,
            emoji: emoji,
          )
        : await client.addReaction(
            authorization: auth,
            messageId: messageId,
            emoji: emoji,
          );
    if (!mounted) return null;
    switch (result) {
      case MessagesApiOk<void>():
        return null;
      case MessagesApiFailure(:final message):
        unawaited(loadInitial());
        state = state.copyWith(errorMessage: message);
        return message;
    }
  }

  Future<String?> addReaction(String messageId, String emoji) async {
    final existing = state.messages
        .where((m) => m.id == messageId)
        .map((m) => m.reactions.where((r) => r.emoji == emoji).firstOrNull)
        .firstOrNull;
    return toggleReaction(
      messageId,
      emoji,
      currentlyReacted: existing?.reactedByMe ?? false,
    );
  }

  Future<String?> togglePin(String messageId, {required bool currentlyPinned}) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    _applyPinDelta(messageId: messageId, pinned: !currentlyPinned);
    final client = _ref.read(voiceMessagesClientProvider);
    final result = currentlyPinned
        ? await client.unpinMessage(
            authorization: auth,
            messageId: messageId,
            chatId: chatId,
          )
        : await client.pinMessage(
            authorization: auth,
            messageId: messageId,
            chatId: chatId,
          );
    if (!mounted) return null;
    switch (result) {
      case MessagesApiOk<void>():
        unawaited(_refreshPinnedMessages(auth));
        return null;
      case MessagesApiFailure(:final message):
        unawaited(loadInitial());
        state = state.copyWith(errorMessage: message);
        return message;
    }
  }

  void _applyPinDelta({required String messageId, required bool pinned}) {
    final updatedMessages = state.messages
        .map(
          (m) => m.id == messageId ? m.copyWith(isPinned: pinned) : m,
        )
        .toList(growable: false);
    final pinnedList = [...state.pinnedMessages];
    if (pinned) {
      final msg = updatedMessages.where((m) => m.id == messageId).firstOrNull;
      if (msg != null && !pinnedList.any((p) => p.id == messageId)) {
        pinnedList.insert(0, msg);
      }
    } else {
      pinnedList.removeWhere((p) => p.id == messageId);
    }
    state = state.copyWith(
      messages: updatedMessages,
      pinnedMessages: pinnedList,
    );
  }

  void _applyReactionDelta({
    required String messageId,
    required String emoji,
    required bool add,
    required bool reactedByMe,
  }) {
    state = state.copyWith(
      messages: state.messages.map((message) {
        if (message.id != messageId) return message;
        final reactions = [...message.reactions];
        final index = reactions.indexWhere((r) => r.emoji == emoji);
        if (add) {
          if (index >= 0) {
            final current = reactions[index];
            reactions[index] = MessageReaction(
              emoji: emoji,
              count: current.count + 1,
              reactedByMe: current.reactedByMe || reactedByMe,
            );
          } else {
            reactions.add(
              MessageReaction(
                emoji: emoji,
                count: 1,
                reactedByMe: reactedByMe,
              ),
            );
          }
        } else if (index >= 0) {
          final current = reactions[index];
          final nextCount = current.count - 1;
          if (nextCount <= 0) {
            reactions.removeAt(index);
          } else {
            reactions[index] = MessageReaction(
              emoji: emoji,
              count: nextCount,
              reactedByMe: reactedByMe ? false : current.reactedByMe,
            );
          }
        }
        return message.copyWith(reactions: reactions);
      }).toList(),
    );
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
        final messages =
            state.messages.where((m) => m.id != messageId).toList();
        state = state.copyWith(messages: messages, clearError: true);
        unawaited(_writeCache(messages));
        _invalidateChatLists(_ref);
        return null;
      case MessagesApiFailure(:final message):
        state = state.copyWith(errorMessage: message);
        return message;
    }
  }

  String? _replaceMessage(VoiceMessage message) {
    final messages = state.messages
        .map((m) => m.id == message.id ? message : m)
        .toList();
    state = state.copyWith(messages: messages, clearError: true);
    unawaited(_writeCache(messages));
    _invalidateChatLists(_ref);
    return null;
  }

  /// Merges an outbound message (e.g. forward) when the target room is open.
  void ingestOutboundMessage(VoiceMessage message) {
    if (!mounted) return;
    final merged = [...state.messages];
    if (!merged.any((m) => m.id == message.id)) {
      merged.add(message);
    }
    final sorted = _sortMessages(merged);
    state = state.copyWith(messages: sorted, clearError: true);
    unawaited(_writeCache(sorted));
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
    _syncSpaceForChat(chatId);
    _ref.read(selectedChatIdProvider.notifier).state = chatId;
    _ref.read(realtimeHubProvider).ensureSubscribed(chatId);
    _rememberDmPeerForChat(chatId);
  }

  void rememberDmPeerForChat(String chatId) => _rememberDmPeerForChat(chatId);

  void _syncSpaceForChat(String chatId) {
    final items = _ref.read(chatListControllerProvider).items;
    for (final item in items) {
      if (item.chatId != chatId) continue;
      final spaceId = item.chat.spaceId;
      if (spaceId != null && spaceId.isNotEmpty) {
        _ref.read(selectedSpaceIdProvider.notifier).state = spaceId;
      } else {
        _ref.read(selectedSpaceIdProvider.notifier).state = null;
      }
      return;
    }
  }

  /// Forwards a message with attribution into another chat the user belongs to.
  Future<String?> forwardMessage({
    required String sourceMessageId,
    required String targetChatId,
    String? commentary,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final trimmedCommentary = commentary?.trim();
    final result = await _ref.read(voiceMessagesClientProvider).forwardMessage(
      authorization: auth,
      sourceMessageId: sourceMessageId,
      targetChatId: targetChatId,
      commentary: trimmedCommentary == null || trimmedCommentary.isEmpty
          ? null
          : trimmedCommentary,
    );
    return switch (result) {
      MessagesApiOk(:final data) => () {
        _invalidateChatLists(_ref);
        if (_ref.read(selectedChatIdProvider) == targetChatId) {
          _ref
              .read(chatRoomControllerProvider(targetChatId).notifier)
              .ingestOutboundMessage(data);
        }
        return null;
      }(),
      MessagesApiFailure(:final message) => message,
    };
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

  Future<String?> transferGroupOwnership(
    String chatId,
    String newOwnerProfileId,
  ) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .transferGroupOwnership(
          authorization: auth,
          chatId: chatId,
          newOwnerProfileId: newOwnerProfileId,
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
