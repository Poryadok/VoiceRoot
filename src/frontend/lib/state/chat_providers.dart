import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:meta/meta.dart';

import '../backend/api_errors.dart';
import '../backend/auth_session.dart';
import '../backend/chats_client.dart';
import '../backend/files_client.dart';
import '../backend/gateway_request_id.dart';
import '../backend/messaging_read_sync.dart';
import '../backend/messages_client.dart';
import '../backend/message_cache/message_cache_store.dart';
import '../backend/realtime_client.dart';
import '../e2e/e2e_exceptions.dart';
import '../e2e/e2e_file_crypto.dart';
import '../e2e/e2e_image_thumb.dart';
import '../gen/voice/messaging/v1/messaging.pb.dart' as messaging_pb;
import 'auth_providers.dart';
import 'inbox_reconciler.dart';
import 'bot_deferred_providers.dart';
import 'connectivity_providers.dart';
import 'gateway_providers.dart';
import 'message_cache_providers.dart';
import 'e2e_providers.dart';
import 'message_requests_providers.dart';
import 'mobile_opened_chat_strip.dart';
import 'space_providers.dart';
import 'shell_providers.dart';

/// Returned by [ChatRoomController.sendMessage] when offline send is blocked.
const String kChatOfflineBlockedError = 'offline_blocked';
const String kChatActionStaleContext = 'stale_context';

final voiceChatsClientProvider = Provider<VoiceChatsClient>((ref) {
  return VoiceChatsClient(gateway: ref.watch(gatewayHttpClientProvider));
});

/// Optimistic mute deadlines keyed by chat id (server source of truth via MuteChat).
final chatMutedUntilProvider = StateProvider<Map<String, DateTime>>(
  (ref) => <String, DateTime>{},
);

final voiceMessagesClientProvider = Provider<VoiceMessagesClient>((ref) {
  return VoiceMessagesClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final voiceFilesClientProvider = Provider<VoiceFilesClient>((ref) {
  return VoiceFilesClient(gateway: ref.watch(gatewayHttpClientProvider));
});

/// Resolves File-authoritative lifecycle metadata for a message attachment.
final fileAttachmentMetadataProvider =
    FutureProvider.family<FileMetadataData?, String>((ref, fileId) async {
      if (fileId.isEmpty) return null;
      final auth = ref.watch(authorizationHeaderProvider);
      if (auth == null) return null;
      final result = await ref
          .read(voiceFilesClientProvider)
          .getFileMetadata(authorization: auth, fileId: fileId);
      return switch (result) {
        FilesApiOk(:final data) => data,
        FilesApiFailure() => null,
      };
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

/// Resolves the File-authorized thumbnail URL for an image attachment.
///
/// Metadata and message payloads never provide a storage key or a thumbnail
/// URL. The File URL surface performs the same ACL check for this variant.
final fileAttachmentThumbnailUrlProvider =
    FutureProvider.family<String?, String>((ref, fileId) async {
      if (fileId.isEmpty) return null;
      final auth = ref.watch(authorizationHeaderProvider);
      if (auth == null) return null;
      final result = await ref
          .read(voiceFilesClientProvider)
          .getFileUrl(
            authorization: auth,
            fileId: fileId,
            variant: FileUrlVariant.thumbnail,
          );
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

/// Client-side thumbnail for decrypted E2E image attachments (cached per [fileId]).
final e2eDecryptedAttachmentThumbProvider =
    FutureProvider.family<Uint8List?, E2eAttachmentDecryptRequest>((
      ref,
      request,
    ) async {
      if (request.fileId.isEmpty) return null;
      final fullBytes = await ref.watch(
        e2eDecryptedAttachmentBytesProvider(request).future,
      );
      if (fullBytes == null) return null;
      return resizeImageBytesForThumb(fullBytes);
    });

/// Active DM chat id in the main column, or null.
final selectedChatIdProvider = StateProvider<String?>((ref) => null);

/// Changes when a successful block/unblock may change the visible DM history.
final socialBlockVisibilityRevisionProvider = StateProvider<int>((ref) => 0);

class SocialBlockVisibilityBarrier {
  const SocialBlockVisibilityBarrier({
    required this.revision,
    this.hiddenProfileIds = const {},
    this.releasingProfileIds = const {},
    this.clearAllUntilServerRefresh = false,
  });

  final int revision;
  final Set<String> hiddenProfileIds;
  final Set<String> releasingProfileIds;
  final bool clearAllUntilServerRefresh;
}

/// Global fail-closed barrier. A block is account-wide, so a room-local
/// profile filter cannot safely protect sibling profiles or other rooms.
final socialBlockVisibilityBarrierProvider =
    StateProvider<SocialBlockVisibilityBarrier>(
      (ref) => const SocialBlockVisibilityBarrier(revision: 0),
    );
final socialBlockMutationPendingProvider = StateProvider<bool>((ref) => false);
final socialBlockCacheMutationEpochProvider = StateProvider<int>((ref) => 0);

String _socialBlockCacheKey(String profileId, String chatId) =>
    '$profileId\u0000$chatId';

/// Rooms whose full server history was refreshed under the latest barrier.
/// Cached history is unavailable for every other room until that refresh.
final socialBlockFilteredHistoryRevisionByChatProvider =
    StateProvider<Map<String, int>>((ref) => const {});

class MessageCacheMutationQueue {
  Future<void> _tail = Future<void>.value();

  Future<void> enqueue(Future<void> Function() mutation) {
    final operation = _tail.then((_) => mutation());
    _tail = operation.catchError((Object _) {});
    return operation;
  }
}

final messageCacheMutationQueueProvider = Provider<MessageCacheMutationQueue>(
  (ref) => MessageCacheMutationQueue(),
);

Future<void> prepareSocialBlockVisibilityChange(Ref ref) async {
  ref.read(socialBlockCacheMutationEpochProvider.notifier).state++;
  // Blocks are account-scoped while offline history is keyed by profile and
  // chat. Purge every profile's old cache so switching profiles or restarting
  // cannot restore a pre-block snapshot. The request must wait for this purge.
  final store = ref.read(messageCacheStoreProvider);
  try {
    await ref.read(messageCacheMutationQueueProvider).enqueue(store.clearAll);
  } catch (_) {
    rethrow;
  }
}

void recordSocialBlockVisibilityChange(
  Ref ref, {
  required bool blocked,
  String? profileId,
  bool refreshSelectedRoom = true,
}) {
  final previous = ref.read(socialBlockVisibilityBarrierProvider);
  final hiddenProfileIds = {...previous.hiddenProfileIds};
  final releasingProfileIds = {...previous.releasingProfileIds};
  final knownProfileId = profileId?.trim();
  if (knownProfileId != null && knownProfileId.isNotEmpty) {
    if (blocked) {
      hiddenProfileIds.add(knownProfileId);
      releasingProfileIds.remove(knownProfileId);
    } else {
      hiddenProfileIds.remove(knownProfileId);
      releasingProfileIds.add(knownProfileId);
    }
  }
  ref.read(socialBlockFilteredHistoryRevisionByChatProvider.notifier).state =
      const {};
  ref
      .read(socialBlockVisibilityBarrierProvider.notifier)
      .state = SocialBlockVisibilityBarrier(
    revision: previous.revision + 1,
    hiddenProfileIds: hiddenProfileIds,
    releasingProfileIds: releasingProfileIds,
    // Account blocks cover sibling profiles; clear each room until its
    // full server history has been reloaded under this revision.
    clearAllUntilServerRefresh: true,
  );
  if (refreshSelectedRoom) {
    ref.read(socialBlockVisibilityRevisionProvider.notifier).state++;
  }
}

bool isDefinitiveSocialMutationRejection(int? statusCode) =>
    statusCode != null &&
    statusCode >= 400 &&
    statusCode < 500 &&
    statusCode != 408 &&
    statusCode != 425 &&
    statusCode != 429;

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
final chatReplyTargetProvider = StateProvider.family<VoiceMessage?, String>(
  (ref, chatId) => null,
);

/// Open thread panel parent message id per chat.
final chatActiveThreadProvider = StateProvider.family<String?, String>(
  (ref, chatId) => null,
);

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
    this.profileId,
  });

  final List<ChatListItem> items;
  final String? nextCursor;
  final bool isLoading;
  final bool isLoadingMore;
  final String? errorMessage;
  final int? errorStatusCode;
  final String? profileId;

  bool get hasMore => nextCursor != null && nextCursor!.isNotEmpty;

  ChatListState copyWith({
    List<ChatListItem>? items,
    String? nextCursor,
    bool clearNextCursor = false,
    bool? isLoading,
    bool? isLoadingMore,
    String? errorMessage,
    int? errorStatusCode,
    String? profileId,
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
      profileId: profileId ?? this.profileId,
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
  int _loadGeneration = 0;

  @override
  void dispose() {
    _authSub?.close();
    super.dispose();
  }

  void _onAuthStateChanged(AuthState? previous, AuthState next) {
    if (previous?.session?.activeProfileId != next.session?.activeProfileId ||
        previous?.session?.accessToken != next.session?.accessToken) {
      _loadGeneration++;
    }
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
    await _loadInitial();
  }

  /// Reloads the active chat list and reports whether the response was applied.
  Future<bool> reloadInitial() => _loadInitial();

  Future<bool> _loadInitial() async {
    final generation = ++_loadGeneration;
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return false;
    final auth = session.authorizationHeader;
    final profileId = session.activeProfileId;
    final inbox = _ref.read(chatInboxProvider);
    final folderId = inbox == 'requests'
        ? null
        : _ref.read(selectedChatFolderIdProvider);
    state = state.profileId == null || state.profileId == profileId
        ? state.copyWith(
            isLoading: true,
            clearError: true,
            profileId: profileId,
          )
        : ChatListState(isLoading: true, profileId: profileId);
    final result = await _ref
        .read(voiceChatsClientProvider)
        .listChats(authorization: auth, inbox: inbox, folderId: folderId);
    if (!mounted) return false;
    if (generation != _loadGeneration ||
        !_matchesSession(profileId, auth) ||
        _ref.read(chatInboxProvider) != inbox ||
        (inbox != 'requests' &&
            _ref.read(selectedChatFolderIdProvider) != folderId)) {
      return false;
    }
    switch (result) {
      case ChatsApiOk(:final data):
        _syncDmPeersFromList(data.items);
        state = ChatListState(
          items: data.items,
          nextCursor: data.nextCursor,
          profileId: profileId,
        );
        return true;
      case ChatsApiFailure(:final message, :final statusCode):
        state = state.copyWith(
          isLoading: false,
          errorMessage: message,
          errorStatusCode: statusCode,
          clearNextCursor: true,
        );
        return false;
    }
  }

  Future<void> loadMore() async {
    final cursor = state.nextCursor;
    if (cursor == null || cursor.isEmpty || state.isLoadingMore) return;
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return;
    final generation = _loadGeneration;
    final auth = session.authorizationHeader;
    final profileId = session.activeProfileId;
    final inbox = _ref.read(chatInboxProvider);
    final folderId = inbox == 'requests'
        ? null
        : _ref.read(selectedChatFolderIdProvider);
    state = state.copyWith(isLoadingMore: true, clearError: true);
    final result = await _ref
        .read(voiceChatsClientProvider)
        .listChats(
          authorization: auth,
          cursor: cursor,
          inbox: inbox,
          folderId: folderId,
        );
    if (!mounted) return;
    if (generation != _loadGeneration ||
        !_matchesSession(profileId, auth) ||
        _ref.read(chatInboxProvider) != inbox ||
        (inbox != 'requests' &&
            _ref.read(selectedChatFolderIdProvider) != folderId) ||
        state.profileId != profileId ||
        state.nextCursor != cursor) {
      return;
    }
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
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return 'not_authenticated';
    final auth = session.authorizationHeader;
    final profileId = session.activeProfileId;
    final result = await _ref
        .read(voiceChatsClientProvider)
        .acceptDmRequest(authorization: auth, chatId: chatId);
    if (!mounted) return kChatActionStaleContext;
    switch (result) {
      case ChatsApiOk<void>():
        final reconciler = _ref.read(inboxReconcilerProvider.notifier);
        final error = _afterRequestAction(chatId, profileId, auth);
        if (error == null) unawaited(reconciler.reconcile());
        return error;
      case ChatsApiFailure(:final message):
        return message;
    }
  }

  Future<String?> declineRequest(String chatId) async {
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return 'not_authenticated';
    final auth = session.authorizationHeader;
    final profileId = session.activeProfileId;
    final result = await _ref
        .read(voiceChatsClientProvider)
        .declineDmRequest(authorization: auth, chatId: chatId);
    if (!mounted) return kChatActionStaleContext;
    return switch (result) {
      ChatsApiOk<void>() => _afterRequestAction(chatId, profileId, auth),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> archiveChat(
    String chatId, {
    required bool archived,
    ChatListItem? sourceItem,
  }) async {
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return 'not_authenticated';
    final auth = session.authorizationHeader;
    final profileId = session.activeProfileId;
    ChatListItem? archivedItem = sourceItem;
    if (archived) {
      if (archivedItem == null) {
        for (final item in state.items) {
          if (item.chatId == chatId) {
            archivedItem = item;
            break;
          }
        }
      }
      final snapshot = _ref
          .read(inboxReconcilerProvider)
          .profileSnapshots[profileId]?[InboxScope.main];
      if (archivedItem == null && snapshot != null) {
        for (final item in snapshot.items) {
          if (item.chatId == chatId) {
            archivedItem = item;
            break;
          }
        }
      }
    }
    final result = await _ref
        .read(voiceChatsClientProvider)
        .archiveChat(authorization: auth, chatId: chatId, archived: archived);
    if (!mounted) return kChatActionStaleContext;
    return switch (result) {
      ChatsApiOk<void>() => () {
        if (!archived) return _afterRequestAction(chatId, profileId, auth);
        if (!_matchesSession(profileId, auth)) {
          return kChatActionStaleContext;
        }
        state = state.copyWith(
          items: state.items.where((item) => item.chatId != chatId).toList(),
        );
        if (archivedItem != null) {
          _ref
              .read(inboxReconcilerProvider.notifier)
              .archiveChat(
                archivedItem,
                expectedProfileId: profileId,
                expectedAuthorization: auth,
              );
        }
        return null;
      }(),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> muteChat(String chatId, {DateTime? mutedUntil}) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .muteChat(authorization: auth, chatId: chatId, mutedUntil: mutedUntil);
    return switch (result) {
      ChatsApiOk<void>() => null,
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> setChatPinnedInFolder({
    required String folderId,
    required String chatId,
    required bool pinned,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final client = _ref.read(voiceChatsClientProvider);
    final result = pinned
        ? await client.pinChatInFolder(
            authorization: auth,
            folderId: folderId,
            chatId: chatId,
          )
        : await client.unpinChatInFolder(
            authorization: auth,
            folderId: folderId,
            chatId: chatId,
          );
    return switch (result) {
      ChatsApiOk<void>() => await _afterFolderPinAction(),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> reorderFolderChats(List<String> chatIds) async {
    final folderId = _ref.read(selectedChatFolderIdProvider);
    if (folderId == null) return 'no_folder';
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final previous = state.items;
    state = state.copyWith(
      items: [
        for (final id in chatIds)
          previous.firstWhere((item) => item.chatId == id),
      ],
    );
    final result = await _ref
        .read(voiceChatsClientProvider)
        .reorderFolderChats(
          authorization: auth,
          folderId: folderId,
          chatIds: chatIds,
        );
    return switch (result) {
      ChatsApiOk<void>() => null,
      ChatsApiFailure(:final message) => () {
        state = state.copyWith(items: previous);
        return message;
      }(),
    };
  }

  Future<String?> _afterFolderPinAction() async {
    await loadInitial();
    return null;
  }

  String? _afterRequestAction(
    String chatId,
    String? expectedProfileId,
    String expectedAuthorization,
  ) {
    if (!_matchesSession(expectedProfileId, expectedAuthorization)) {
      return kChatActionStaleContext;
    }
    state = state.copyWith(
      items: state.items.where((item) => item.chatId != chatId).toList(),
    );
    final isRequestsSelection =
        _ref.read(chatInboxProvider) == 'requests' &&
        isMessageRequestsFolderSelected(
          _ref.read(selectedChatFolderIdProvider),
        );
    final navigationGeneration = _ref.read(
      messageRequestsNavigationGenerationProvider,
    );
    final container = _ref.container;
    final Future<MessageRequestsSummary>? summaryFuture;
    if (isRequestsSelection) {
      summaryFuture = container.refresh(messageRequestsSummaryProvider.future);
    } else {
      summaryFuture = null;
      container.invalidate(messageRequestsSummaryProvider);
    }
    _invalidateChatLists(_ref);
    if (summaryFuture != null) {
      unawaited(
        restorePreviousChatFolderAfterFinalRequest(
          container,
          summaryFuture: summaryFuture,
          expectedGeneration: navigationGeneration,
          expectedAuthorization: expectedAuthorization,
          expectedProfileId: expectedProfileId,
        ),
      );
    }
    return null;
  }

  bool _matchesSession(String? profileId, String authorization) {
    final session = _ref.read(authControllerProvider).session;
    return session?.activeProfileId == profileId &&
        session?.authorizationHeader == authorization;
  }

  /// Optimistic unread bump when an in-app notification arrives for a background chat.
  void bumpUnread(String chatId, {int delta = 1}) {
    if (delta <= 0) return;
    final index = state.items.indexWhere((item) => item.chatId == chatId);
    if (index < 0) return;
    final item = state.items[index];
    final updated = item.copyWith(unreadCount: item.unreadCount + delta);
    final items = [...state.items];
    items[index] = updated;
    state = state.copyWith(items: items);
  }

  bool markChatRead(String chatId) {
    final index = state.items.indexWhere((item) => item.chatId == chatId);
    if (index < 0) {
      _loadGeneration++;
      state = state.copyWith(isLoading: false, isLoadingMore: false);
      return false;
    }
    // A list response that began before the persisted read position may still
    // contain the old unread count. Do not let it overwrite the local read.
    _loadGeneration++;
    final items = [...state.items];
    items[index] = items[index].copyWith(unreadCount: 0);
    state = state.copyWith(
      items: items,
      isLoading: false,
      isLoadingMore: false,
    );
    return true;
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

class ChatArchiveListController extends StateNotifier<ChatListState> {
  ChatArchiveListController(this._ref) : super(const ChatListState()) {
    _authSub = _ref.listen<AuthState>(authControllerProvider, (_, _) {
      _loadGeneration++;
    });
  }

  final Ref _ref;
  ProviderSubscription<AuthState>? _authSub;
  int _loadGeneration = 0;

  @override
  void dispose() {
    _authSub?.close();
    super.dispose();
  }

  Future<void> loadInitial() async {
    final generation = ++_loadGeneration;
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return;
    final auth = session.authorizationHeader;
    final profileId = session.activeProfileId;
    state = state.profileId == null || state.profileId == profileId
        ? state.copyWith(
            isLoading: true,
            clearError: true,
            profileId: profileId,
          )
        : ChatListState(isLoading: true, profileId: profileId);
    final result = await _ref
        .read(voiceChatsClientProvider)
        .listChats(authorization: auth, inbox: 'archive');
    if (!mounted) return;
    if (generation != _loadGeneration || !_matchesSession(profileId, auth)) {
      return;
    }
    switch (result) {
      case ChatsApiOk(:final data):
        state = ChatListState(
          items: data.items,
          nextCursor: data.nextCursor,
          profileId: profileId,
        );
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
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return;
    final generation = _loadGeneration;
    final auth = session.authorizationHeader;
    final profileId = session.activeProfileId;
    state = state.copyWith(isLoadingMore: true, clearError: true);
    final result = await _ref
        .read(voiceChatsClientProvider)
        .listChats(authorization: auth, cursor: cursor, inbox: 'archive');
    if (!mounted) return;
    if (generation != _loadGeneration ||
        !_matchesSession(profileId, auth) ||
        state.profileId != profileId ||
        state.nextCursor != cursor) {
      return;
    }
    switch (result) {
      case ChatsApiOk(:final data):
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

  Future<String?> unarchiveChat(String chatId) async {
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return 'not_authenticated';
    final auth = session.authorizationHeader;
    final profileId = session.activeProfileId;
    final result = await _ref
        .read(voiceChatsClientProvider)
        .archiveChat(authorization: auth, chatId: chatId, archived: false);
    if (!mounted) return kChatActionStaleContext;
    switch (result) {
      case ChatsApiOk<void>():
        if (!_matchesSession(profileId, auth)) {
          return kChatActionStaleContext;
        }
        state = state.copyWith(
          items: state.items.where((item) => item.chatId != chatId).toList(),
        );
        _ref
            .read(inboxReconcilerProvider.notifier)
            .unarchiveChat(
              chatId,
              expectedProfileId: profileId,
              expectedAuthorization: auth,
            );
        return null;
      case ChatsApiFailure(:final message):
        return message;
    }
  }

  /// Quiet activity for an archived chat: update its badge only. The Realtime
  /// `archive_activity` frame intentionally has no notification-center or
  /// sound semantics (text-chat.md §Архивирование).
  void bumpUnread(String chatId, {int delta = 1}) {
    if (delta <= 0) return;
    final index = state.items.indexWhere((item) => item.chatId == chatId);
    if (index < 0) return;
    final item = state.items[index];
    final items = [...state.items];
    items[index] = item.copyWith(unreadCount: item.unreadCount + delta);
    state = state.copyWith(items: items);
  }

  bool _matchesSession(String? profileId, String authorization) {
    final session = _ref.read(authControllerProvider).session;
    return session?.activeProfileId == profileId &&
        session?.authorizationHeader == authorization;
  }
}

final chatArchiveListControllerProvider =
    StateNotifierProvider.autoDispose<ChatArchiveListController, ChatListState>(
      (ref) => ChatArchiveListController(ref),
    );

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
    ChatsApiFailure(:final statusCode) when isBackendUnavailable(statusCode) =>
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
    this.pinnedMessagesStatus = PinnedMessagesLoadStatus.idle,
    this.pinnedMessagesErrorStatusCode,
    this.isOfflineCache = false,
    this.isDmPeerDeleted = false,
    this.historyProfileId,
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
  final PinnedMessagesLoadStatus pinnedMessagesStatus;
  final int? pinnedMessagesErrorStatusCode;
  final bool isOfflineCache;
  final bool isDmPeerDeleted;

  /// The profile whose successful history result owns [messages] and cursor.
  /// This remains intact across a profile switch so controller lifecycle code
  /// can reject stale events without discarding a useful snapshot.
  final String? historyProfileId;

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
    PinnedMessagesLoadStatus? pinnedMessagesStatus,
    int? pinnedMessagesErrorStatusCode,
    bool clearPinnedMessagesErrorStatusCode = false,
    bool? isOfflineCache,
    bool? isDmPeerDeleted,
    String? historyProfileId,
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
      pinnedMessagesStatus: pinnedMessagesStatus ?? this.pinnedMessagesStatus,
      pinnedMessagesErrorStatusCode: clearPinnedMessagesErrorStatusCode
          ? null
          : (pinnedMessagesErrorStatusCode ??
                this.pinnedMessagesErrorStatusCode),
      isOfflineCache: isOfflineCache ?? this.isOfflineCache,
      isDmPeerDeleted: isDmPeerDeleted ?? this.isDmPeerDeleted,
      historyProfileId: historyProfileId ?? this.historyProfileId,
    );
  }
}

enum PinnedMessagesLoadStatus { idle, loading, loaded, failed }

class PinMutationResult {
  const PinMutationResult.success()
    : succeeded = true,
      stale = false,
      message = null,
      errorCode = null,
      statusCode = null;

  const PinMutationResult.failure({
    required this.message,
    this.errorCode,
    this.statusCode,
  }) : succeeded = false,
       stale = false;

  const PinMutationResult.stale()
    : succeeded = false,
      stale = true,
      message = null,
      errorCode = null,
      statusCode = null;

  final bool succeeded;
  final bool stale;
  final String? message;
  final String? errorCode;
  final int? statusCode;

  bool get permissionDenied =>
      statusCode == 403 || errorCode == 'permission_denied';

  bool get pinLimitReached =>
      statusCode == 429 && errorCode == 'resource_exhausted';
}

class PendingPinnedMessageJump {
  PendingPinnedMessageJump(this.messageId);

  final String messageId;
  final Completer<bool> _completion = Completer<bool>();
  bool cancelled = false;

  Future<bool> get result => _completion.future;

  void complete(bool opened) {
    if (!_completion.isCompleted) _completion.complete(opened);
  }
}

final pendingPinnedMessageJumpProvider = StateProvider.autoDispose
    .family<PendingPinnedMessageJump?, String>((ref, chatId) => null);

enum RealtimeLinkStatus { disconnected, connecting, connected, reconnecting }

class ChatRoomController extends StateNotifier<ChatRoomState> {
  ChatRoomController(this._ref, this.chatId) : super(const ChatRoomState()) {
    _authSub = _ref.listen<AuthState>(authControllerProvider, (previous, next) {
      final previousProfileId = previous?.activeProfileId;
      final nextProfileId = next.activeProfileId;
      final profileChanged = previousProfileId != nextProfileId;
      final sessionChanged =
          previous?.session?.accessToken != next.session?.accessToken ||
          previous?.session?.refreshToken != next.session?.refreshToken;
      if (!profileChanged && !sessionChanged) {
        return;
      }
      _loadGeneration++;
      if (!profileChanged && nextProfileId != null) {
        _pinnedMessagesGeneration++;
        if (mounted) {
          state = state.copyWith(
            isLoadingOlder: false,
            pinnedMessagesStatus: state.pinnedMessages.isEmpty
                ? PinnedMessagesLoadStatus.idle
                : PinnedMessagesLoadStatus.loaded,
            clearPinnedMessagesErrorStatusCode: true,
          );
        }
        return;
      }
      _historyGeneration++;
      _loadedHistoryProfileId = null;
      _pinnedMessagesGeneration++;
      if (mounted) {
        // Keep the snapshot/cursor for lifecycle fencing while panel
        // presentation waits for the new profile's bound history.
        state = state.copyWith(
          isLoading: true,
          isOfflineCache: false,
          clearError: true,
          isDmPeerDeleted: false,
          pinnedMessages: const [],
          pinnedMessagesStatus: PinnedMessagesLoadStatus.idle,
          clearPinnedMessagesErrorStatusCode: true,
        );
      }
    });
    _realtimeSub = _ref.listen<RealtimeLinkStatus>(realtimeLinkStatusProvider, (
      _,
      next,
    ) {
      final prev = state.realtimeStatus;
      state = state.copyWith(realtimeStatus: next);
      if (prev == RealtimeLinkStatus.reconnecting &&
          next == RealtimeLinkStatus.connected &&
          _isSelectedForAutomaticHistory()) {
        unawaited(_catchUpAfterReconnect());
      }
    }, fireImmediately: true);
    _eventSub = _ref.listen<AsyncValue<RealtimeFrame>>(realtimeEventProvider, (
      _,
      next,
    ) {
      next.whenData((frame) {
        if (frame.op == 'message_create') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId == this.chatId) {
            if (_ref.read(deferredBotInteractionProvider(this.chatId)) !=
                null) {
              _ref
                  .read(deferredBotInteractionProvider(this.chatId).notifier)
                  .clear();
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
            if (_isSelectedForAutomaticHistory()) {
              unawaited(_catchUpAfterEvent());
            }
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
        } else if (frame.op == 'message_read_revoked') {
          final chatId = frame.data?['chat_id'] as String?;
          final messageId = frame.data?['message_id'] as String?;
          final recipientProfileId =
              frame.data?['recipient_profile_id'] as String?;
          final activeProfileId = _ref
              .read(authControllerProvider)
              .activeProfileId;
          if (chatId == this.chatId &&
              messageId != null &&
              (recipientProfileId == null ||
                  recipientProfileId.isEmpty ||
                  recipientProfileId == activeProfileId)) {
            final read = {...state.readMessageIds}..remove(messageId);
            state = state.copyWith(readMessageIds: read);
            _ref.invalidate(chatListProvider);
          }
        } else if (frame.op == 'message_update' ||
            frame.op == 'message_delete') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId == this.chatId) {
            if (_isSelectedForAutomaticHistory()) {
              unawaited(loadInitial());
            }
          }
        } else if (frame.op == 'mention') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId == this.chatId) {
            if (_isSelectedForAutomaticHistory()) {
              unawaited(_catchUpAfterEvent());
            }
          }
        } else if (frame.op == 'reaction_add' ||
            frame.op == 'reaction_remove') {
          final chatId = frame.data?['chat_id'] as String?;
          if (chatId != this.chatId) return;
          final profileId = frame.data?['profile_id'] as String?;
          final activeProfile = _ref
              .read(authControllerProvider)
              .activeProfileId;
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
          final activeProfile = _ref
              .read(authControllerProvider)
              .activeProfileId;
          if (actor == activeProfile) return;
          _applyPinDelta(
            messageId: messageId,
            pinned: frame.op == 'message_pinned',
          );
        } else if (frame.op == 'dm_peer_deleted') {
          _handleDmPeerDeleted(frame);
        }
      });
    });
    _selectionSub = _ref.listen<String?>(selectedChatIdProvider, (
      previous,
      next,
    ) {
      if (next == chatId && previous != chatId) {
        _scheduleAutomaticActivation();
      }
    });
    _socialBlockVisibilitySub = _ref.listen<int>(
      socialBlockVisibilityRevisionProvider,
      (_, _) {
        final barrier = _ref.read(socialBlockVisibilityBarrierProvider);
        if (barrier.revision > _visibilityBarrierRevision) {
          _applySocialBlockVisibilityBarrier(barrier);
        } else if (_ref.read(selectedChatIdProvider) == chatId) {
          unawaited(loadInitial());
        }
      },
    );
    _socialBlockBarrierSub = _ref.listen<SocialBlockVisibilityBarrier>(
      socialBlockVisibilityBarrierProvider,
      (_, barrier) =>
          _applySocialBlockVisibilityBarrier(barrier, reload: false),
    );
    _socialBlockCacheEpochSub = _ref.listen<int>(
      socialBlockCacheMutationEpochProvider,
      (_, _) {
        _loadGeneration++;
        _historyGeneration++;
        _loadedHistoryProfileId = null;
      },
    );
    final existingBarrier = _ref.read(socialBlockVisibilityBarrierProvider);
    if (existingBarrier.revision > 0) {
      _applySocialBlockVisibilityBarrier(existingBarrier, reload: false);
    }
    if (_isSelectedForAutomaticHistory()) {
      _scheduleAutomaticActivation();
    }
  }

  final Ref _ref;
  final String chatId;
  ProviderSubscription<AuthState>? _authSub;
  ProviderSubscription<RealtimeLinkStatus>? _realtimeSub;
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _eventSub;
  ProviderSubscription<String?>? _selectionSub;
  ProviderSubscription<int>? _socialBlockVisibilitySub;
  ProviderSubscription<SocialBlockVisibilityBarrier>? _socialBlockBarrierSub;
  ProviderSubscription<int>? _socialBlockCacheEpochSub;
  String? _lastMarkedReadMessageId;
  bool _automaticActivationStarted = false;
  final Set<String> _hiddenMessageSenderProfileIds = <String>{};
  var _clearAllCachedHistoryUntilServerRefresh = false;
  var _visibilityBarrierRevision = 0;

  void _applySocialBlockVisibilityBarrier(
    SocialBlockVisibilityBarrier barrier, {
    bool reload = true,
  }) {
    if (barrier.revision <= _visibilityBarrierRevision) return;
    final profileId = _activeProfileId();
    final cacheKey = profileId == null
        ? null
        : _socialBlockCacheKey(profileId, chatId);
    final filteredRevision = cacheKey == null
        ? null
        : _ref.read(socialBlockFilteredHistoryRevisionByChatProvider)[cacheKey];
    _visibilityBarrierRevision = barrier.revision;
    _hiddenMessageSenderProfileIds
      ..clear()
      ..addAll(barrier.hiddenProfileIds);
    _clearAllCachedHistoryUntilServerRefresh =
        filteredRevision != barrier.revision;
    if (!_clearAllCachedHistoryUntilServerRefresh) return;
    _loadGeneration++;
    _historyGeneration++;
    _loadedHistoryProfileId = null;
    final activeProfileId = _activeProfileId();
    state = state.copyWith(
      messages: activeProfileId == null
          ? const []
          : state.messages
                .where((message) => message.senderProfileId == activeProfileId)
                .toList(),
      pinnedMessages: activeProfileId == null
          ? const []
          : state.pinnedMessages
                .where((message) => message.senderProfileId == activeProfileId)
                .toList(),
      isLoading: reload,
      isOfflineCache: false,
      hasMore: false,
      clearNextCursor: true,
      clearError: true,
    );
    unawaited(_rewriteCachedMessagesForCurrentBlockPolicy());
    if (reload && _ref.read(selectedChatIdProvider) == chatId) {
      unawaited(loadInitial());
    }
  }

  List<VoiceMessage> _visibleForCurrentBlockPolicy(
    Iterable<VoiceMessage> messages,
  ) {
    return _visibleForBlockPolicy(
      messages,
      _hiddenMessageSenderProfileIds,
      _clearAllCachedHistoryUntilServerRefresh,
    );
  }

  List<VoiceMessage> _visibleForBlockPolicy(
    Iterable<VoiceMessage> messages,
    Set<String> hiddenProfileIds,
    bool clearAll,
  ) {
    if (clearAll) return const [];
    return messages
        .where((message) => !hiddenProfileIds.contains(message.senderProfileId))
        .toList();
  }

  bool _isSelectedForAutomaticHistory() {
    final selected = _ref.read(selectedChatIdProvider);
    // A null selection preserves existing controller-level/test callers. Once
    // navigation has an explicit selection, mounted background rooms stay
    // passive and cannot start REST history catch-up.
    return selected == null || selected == chatId;
  }

  void _scheduleAutomaticActivation() {
    if (_automaticActivationStarted) return;
    _automaticActivationStarted = true;
    Future<void>(() async {
      if (!mounted || !_isSelectedForAutomaticHistory()) {
        _automaticActivationStarted = false;
        return;
      }
      unawaited(loadInitial());
      _ref.read(realtimeHubProvider).ensureSubscribed(chatId);
    });
  }

  String? _loadedHistoryProfileId;
  var _historyGeneration = 0;
  var _loadGeneration = 0;
  var _pinnedMessagesGeneration = 0;
  var _olderPageGeneration = 0;

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

  Future<List<VoiceMessage>> _finalizeMessages(
    List<VoiceMessage> messages,
  ) async {
    if (!_isE2eChat()) return messages;
    final localId = _activeProfileId();
    final peerId = _dmPeerProfileId();
    if (localId == null || peerId == null) return messages;
    final auth = _ref.read(authorizationHeaderProvider);
    return _ref
        .read(e2eMessageServiceProvider)
        .decryptAllForDisplay(
          messages: messages,
          localProfileId: localId,
          peerProfileId: peerId,
          authorization: auth,
        );
  }

  @override
  void dispose() {
    _authSub?.close();
    _realtimeSub?.close();
    _eventSub?.close();
    _selectionSub?.close();
    _socialBlockVisibilitySub?.close();
    _socialBlockBarrierSub?.close();
    _socialBlockCacheEpochSub?.close();
    super.dispose();
  }

  Future<void> loadInitial() async {
    final auth = _ref.read(authorizationHeaderProvider);
    final profileId = _activeProfileId();
    if (auth == null || profileId == null) return;
    final loadGeneration = ++_loadGeneration;
    final previousHistoryProfileId = _loadedHistoryProfileId;
    final hadLoadedHistory =
        previousHistoryProfileId == profileId && state.messages.isNotEmpty;
    final historyGeneration = ++_historyGeneration;
    _loadedHistoryProfileId = null;
    if (previousHistoryProfileId != profileId) {
      state = state.copyWith(isDmPeerDeleted: false);
    }
    state = state.copyWith(
      isLoading: true,
      clearError: true,
      isOfflineCache: false,
    );
    if (_isDeviceOffline()) {
      final served = await _serveCachedMessages(
        profileId: profileId,
        authorization: auth,
        historyGeneration: historyGeneration,
      );
      if (served) return;
    }
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .getMessages(authorization: auth, chatId: chatId);
    if (!_isCurrentInitialLoad(
      profileId: profileId,
      authorization: auth,
      generation: loadGeneration,
    )) {
      return;
    }
    switch (result) {
      case MessagesApiOk(:final data):
        final finalized = await _finalizeMessages(_sortMessages(data.messages));
        if (!_isCurrentInitialLoad(
          profileId: profileId,
          authorization: auth,
          generation: loadGeneration,
        )) {
          return;
        }
        _markFilteredServerHistoryLoaded(profileId);
        final sorted = _visibleForCurrentBlockPolicy(finalized);
        final isDeleted =
            data.dmPeerState == messaging_pb.DmPeerState.DM_PEER_STATE_DELETED;
        if (isDeleted && hadLoadedHistory) {
          _loadedHistoryProfileId = profileId;
          _markDmPeerDeleted();
          state = state.copyWith(isLoading: false, clearError: true);
          return;
        }
        if (data.messages.isNotEmpty) {
          _applyDmPeerState(data.dmPeerState);
        }
        if (data.messages.isNotEmpty) {
          _loadedHistoryProfileId = profileId;
        }
        state = state.copyWith(
          messages: sorted,
          isLoading: false,
          isOfflineCache: false,
          nextCursor: data.nextCursor,
          clearNextCursor: data.nextCursor == null,
          hasMore: data.hasMore && data.nextCursor != null,
          historyProfileId: profileId,
          clearError: true,
        );
        await _writeCache(sorted, profileId: profileId);
        unawaited(_markLatestRead());
        unawaited(
          _refreshPinnedMessages(
            auth,
            profileId: profileId,
            generation: loadGeneration,
          ),
        );
      case MessagesApiFailure(:final message, :final errorCode):
        if (errorCode == 'network_error') {
          final served = await _serveCachedMessages(
            profileId: profileId,
            authorization: auth,
            historyGeneration: historyGeneration,
          );
          if (served) return;
        }
        state = state.copyWith(
          isLoading: false,
          errorMessage: message,
          isOfflineCache: false,
        );
    }
  }

  Future<void> _refreshPinnedMessages(
    String auth, {
    required String profileId,
    required int generation,
  }) async {
    final requestGeneration = ++_pinnedMessagesGeneration;
    state = state.copyWith(
      pinnedMessagesStatus: PinnedMessagesLoadStatus.loading,
      clearPinnedMessagesErrorStatusCode: true,
    );
    final pinned = await _ref
        .read(voiceMessagesClientProvider)
        .getPinnedMessages(authorization: auth, chatId: chatId);
    if (!_isCurrentMutation(
          profileId: profileId,
          authorization: auth,
          generation: generation,
        ) ||
        requestGeneration != _pinnedMessagesGeneration) {
      return;
    }
    switch (pinned) {
      case MessagesApiOk(:final data):
        state = state.copyWith(
          pinnedMessages: _visibleForCurrentBlockPolicy(data.messages),
          pinnedMessagesStatus: PinnedMessagesLoadStatus.loaded,
          clearPinnedMessagesErrorStatusCode: true,
        );
      case MessagesApiFailure(:final statusCode):
        state = state.copyWith(
          pinnedMessagesStatus: PinnedMessagesLoadStatus.failed,
          pinnedMessagesErrorStatusCode: statusCode,
        );
    }
  }

  Future<void> retryPinnedMessages() async {
    final auth = _ref.read(authorizationHeaderProvider);
    final profileId = _activeProfileId();
    if (auth == null || profileId == null) return;
    await _refreshPinnedMessages(
      auth,
      profileId: profileId,
      generation: _loadGeneration,
    );
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
    final profileId = _activeProfileId();
    final historyGeneration = _historyGeneration;
    if (auth == null || profileId == null) return;
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .getMessages(
          authorization: auth,
          chatId: chatId,
          afterMessageId: afterMessageId,
          lastMessageId: lastMessageId,
        );
    if (result case MessagesApiOk(:final data)) {
      if (!_isCurrentHistory(
        profileId: profileId,
        authorization: auth,
        generation: historyGeneration,
      )) {
        return;
      }
      _applyDmPeerState(data.dmPeerState);
      if (data.dmPeerState == messaging_pb.DmPeerState.DM_PEER_STATE_DELETED) {
        return;
      }
      if (data.messages.isEmpty) return;
      final merged = [...state.messages];
      for (final m in data.messages) {
        if (!merged.any((x) => x.id == m.id)) {
          merged.add(m);
        }
      }
      final sorted = await _finalizeMessages(
        _sortMessages(_visibleForCurrentBlockPolicy(merged)),
      );
      if (!_isCurrentHistory(
        profileId: profileId,
        authorization: auth,
        generation: historyGeneration,
      )) {
        return;
      }
      state = state.copyWith(
        messages: sorted,
        clearError: true,
        isOfflineCache: false,
      );
      unawaited(_writeCache(sorted, profileId: profileId));
      unawaited(_markLatestRead());
      _invalidateChatLists(_ref);
    }
  }

  Future<bool> loadOlderMessages() async {
    if (_isDeviceOffline()) return false;
    final cursor = state.nextCursor;
    if (cursor == null || cursor.isEmpty || state.isLoadingOlder) return false;
    final auth = _ref.read(authorizationHeaderProvider);
    final profileId = _activeProfileId();
    final historyGeneration = _historyGeneration;
    final pageGeneration = ++_olderPageGeneration;
    if (auth == null || profileId == null) return false;
    state = state.copyWith(isLoadingOlder: true, clearError: true);
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .getMessages(authorization: auth, chatId: chatId, cursor: cursor);
    if (!_isCurrentHistory(
      profileId: profileId,
      authorization: auth,
      generation: historyGeneration,
    )) {
      if (mounted &&
          pageGeneration == _olderPageGeneration &&
          state.isLoadingOlder) {
        state = state.copyWith(isLoadingOlder: false);
      }
      return false;
    }
    switch (result) {
      case MessagesApiOk(:final data):
        _applyDmPeerState(data.dmPeerState);
        if (data.dmPeerState ==
            messaging_pb.DmPeerState.DM_PEER_STATE_DELETED) {
          state = state.copyWith(isLoadingOlder: false);
          return false;
        }
        final merged = [...state.messages];
        for (final m in data.messages) {
          if (!merged.any((x) => x.id == m.id)) {
            merged.add(m);
          }
        }
        final sorted = await _finalizeMessages(
          _sortMessages(_visibleForCurrentBlockPolicy(merged)),
        );
        if (!_isCurrentHistory(
          profileId: profileId,
          authorization: auth,
          generation: historyGeneration,
        )) {
          if (mounted &&
              pageGeneration == _olderPageGeneration &&
              state.isLoadingOlder) {
            state = state.copyWith(isLoadingOlder: false);
          }
          return false;
        }
        state = state.copyWith(
          messages: sorted,
          isLoadingOlder: false,
          isOfflineCache: false,
          nextCursor: data.nextCursor,
          clearNextCursor: data.nextCursor == null,
          hasMore: data.hasMore && data.nextCursor != null,
          clearError: true,
        );
        unawaited(_writeCache(sorted, profileId: profileId));
        return true;
      case MessagesApiFailure(:final message):
        state = state.copyWith(isLoadingOlder: false, errorMessage: message);
        return false;
    }
  }

  /// Loads existing cursor pages until [messageId] is present or the current
  /// history reaches an explicit end/failure. It never repeats a cursor.
  Future<bool> loadHistoryUntilMessage(
    String messageId, {
    required bool Function() isCancelled,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    final profileId = _activeProfileId();
    final historyGeneration = _historyGeneration;
    if (auth == null || profileId == null) return false;
    final visitedCursors = <String>{};
    while (true) {
      if (isCancelled() ||
          !_isSelectedForAutomaticHistory() ||
          !_isCurrentHistory(
            profileId: profileId,
            authorization: auth,
            generation: historyGeneration,
          )) {
        return false;
      }
      if (state.messages.any((message) => message.id == messageId)) return true;
      final cursor = state.nextCursor;
      if (!state.hasMore || cursor == null || cursor.isEmpty) return false;
      if (!visitedCursors.add(cursor) || state.isLoadingOlder) return false;
      final loaded = await loadOlderMessages();
      if (!loaded || isCancelled() || !_isSelectedForAutomaticHistory()) {
        return false;
      }
      if (!_isCurrentHistory(
        profileId: profileId,
        authorization: auth,
        generation: historyGeneration,
      )) {
        return false;
      }
      if (state.messages.any((message) => message.id == messageId)) return true;
      if (state.nextCursor == cursor) return false;
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
    final profileId = _activeProfileId();
    final generation = _loadGeneration;
    if (auth == null || profileId == null) return 'not_authenticated';
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
          outbound = await _ref
              .read(e2eMessageServiceProvider)
              .encryptOutgoing(
                localProfileId: localId,
                peerProfileId: peerId,
                plaintext: outbound,
                authorization: auth,
                chatId: chatId,
              );
          if (!_isCurrentMutation(
            profileId: profileId,
            authorization: auth,
            generation: generation,
          )) {
            return null;
          }
          isE2e = true;
        } on E2eEncryptException catch (e) {
          if (!_isCurrentMutation(
            profileId: profileId,
            authorization: auth,
            generation: generation,
          )) {
            return null;
          }
          state = state.copyWith(isSending: false, errorMessage: e.message);
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
    if (!_isCurrentMutation(
      profileId: profileId,
      authorization: auth,
      generation: generation,
    )) {
      return null;
    }
    switch (result) {
      case MessagesApiOk(:final data):
        final merged = [...state.messages];
        if (!merged.any((m) => m.id == data.id)) {
          merged.add(data);
        }
        final sorted = await _finalizeMessages(
          _sortMessages(_visibleForCurrentBlockPolicy(merged)),
        );
        if (!_isCurrentMutation(
          profileId: profileId,
          authorization: auth,
          generation: generation,
        )) {
          return null;
        }
        state = state.copyWith(
          messages: sorted,
          isSending: false,
          isOfflineCache: false,
          clearError: true,
        );
        unawaited(_writeCache(sorted, profileId: profileId));
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

  bool _isCurrentInitialLoad({
    required String profileId,
    required String authorization,
    required int generation,
  }) {
    return mounted &&
        _loadGeneration == generation &&
        _activeProfileId() == profileId &&
        _ref.read(authorizationHeaderProvider) == authorization;
  }

  bool _isCurrentMutation({
    required String profileId,
    required String authorization,
    required int generation,
  }) {
    return mounted &&
        _loadGeneration == generation &&
        _activeProfileId() == profileId &&
        _ref.read(authorizationHeaderProvider) == authorization;
  }

  bool _isCurrentHistory({
    required String profileId,
    required String authorization,
    required int generation,
  }) {
    return mounted &&
        _loadedHistoryProfileId == profileId &&
        _historyGeneration == generation &&
        _activeProfileId() == profileId &&
        _ref.read(authorizationHeaderProvider) == authorization;
  }

  void _markDmPeerDeleted() {
    if (!state.isDmPeerDeleted) {
      state = state.copyWith(isDmPeerDeleted: true);
    }
  }

  void _applyDmPeerState(messaging_pb.DmPeerState? peerState) {
    if (peerState == messaging_pb.DmPeerState.DM_PEER_STATE_DELETED) {
      _markDmPeerDeleted();
    }
  }

  void _handleDmPeerDeleted(RealtimeFrame frame) {
    final data = frame.data;
    final recipientProfileId = data?['recipient_profile_id'] as String?;
    final activeProfileId = _activeProfileId();
    if (data?['chat_id'] != chatId ||
        recipientProfileId == null ||
        recipientProfileId != activeProfileId ||
        _loadedHistoryProfileId != activeProfileId) {
      return;
    }
    _markDmPeerDeleted();
  }

  Future<bool> _serveCachedMessages({
    required String profileId,
    required String authorization,
    required int historyGeneration,
  }) async {
    final barrier = _ref.read(socialBlockVisibilityBarrierProvider);
    if (_ref.read(socialBlockMutationPendingProvider)) return false;
    final filteredRevision = _ref.read(
      socialBlockFilteredHistoryRevisionByChatProvider,
    )[_socialBlockCacheKey(profileId, chatId)];
    if (barrier.revision > 0 && filteredRevision != barrier.revision) {
      return false;
    }
    final cached = await _ref
        .read(messageCacheStoreProvider)
        .getMessages(profileId: profileId, chatId: chatId);
    if (!_isCurrentInitialLoad(
          profileId: profileId,
          authorization: authorization,
          generation: _loadGeneration,
        ) ||
        _historyGeneration != historyGeneration) {
      return false;
    }
    final visibleCached = _visibleForCurrentBlockPolicy(cached);
    if (visibleCached.length != cached.length) {
      await _writeCache(visibleCached, profileId: profileId);
    }
    if (visibleCached.isEmpty) return false;
    final finalized = await _finalizeMessages(_sortMessages(visibleCached));
    if (!_isCurrentInitialLoad(
          profileId: profileId,
          authorization: authorization,
          generation: _loadGeneration,
        ) ||
        _historyGeneration != historyGeneration) {
      return false;
    }
    final sorted = _visibleForCurrentBlockPolicy(finalized);
    _loadedHistoryProfileId = profileId;
    state = state.copyWith(
      messages: sorted,
      isLoading: false,
      isOfflineCache: true,
      clearError: true,
      hasMore: false,
      clearNextCursor: true,
      historyProfileId: profileId,
    );
    return true;
  }

  void _markFilteredServerHistoryLoaded(String profileId) {
    final barrier = _ref.read(socialBlockVisibilityBarrierProvider);
    if (barrier.revision == 0) return;
    _hiddenMessageSenderProfileIds
      ..clear()
      ..addAll(barrier.hiddenProfileIds);
    _clearAllCachedHistoryUntilServerRefresh = false;
    _visibilityBarrierRevision = barrier.revision;
    final revisions = {
      ..._ref.read(socialBlockFilteredHistoryRevisionByChatProvider),
      _socialBlockCacheKey(profileId, chatId): barrier.revision,
    };
    _ref.read(socialBlockFilteredHistoryRevisionByChatProvider.notifier).state =
        revisions;
  }

  Future<void> _writeCache(
    List<VoiceMessage> messages, {
    String? profileId,
  }) async {
    profileId ??= _activeProfileId();
    if (profileId == null) return;
    final targetProfileId = profileId;
    final sourceVisibilityRevision = _ref
        .read(socialBlockVisibilityBarrierProvider)
        .revision;
    final sourceCacheEpoch = _ref.read(socialBlockCacheMutationEpochProvider);
    return _enqueueCacheMutation((store) async {
      final barrier = _ref.read(socialBlockVisibilityBarrierProvider);
      final cacheEpoch = _ref.read(socialBlockCacheMutationEpochProvider);
      final filteredRevision = _ref.read(
        socialBlockFilteredHistoryRevisionByChatProvider,
      )[_socialBlockCacheKey(targetProfileId, chatId)];
      final isStaleHistory =
          _ref.read(socialBlockMutationPendingProvider) ||
          cacheEpoch != sourceCacheEpoch ||
          barrier.revision != sourceVisibilityRevision ||
          (barrier.revision > 0 &&
              filteredRevision != sourceVisibilityRevision);
      await store.replaceChatMessages(
        profileId: targetProfileId,
        chatId: chatId,
        messages: isStaleHistory
            ? const []
            : _visibleForCurrentBlockPolicy(messages),
      );
    });
  }

  Future<void> _rewriteCachedMessagesForCurrentBlockPolicy() async {
    final profileId = _activeProfileId();
    if (profileId == null) return;
    final hiddenProfileIds = {..._hiddenMessageSenderProfileIds};
    final clearAll = _clearAllCachedHistoryUntilServerRefresh;
    return _enqueueCacheMutation((store) async {
      final cached = await store.getMessages(
        profileId: profileId,
        chatId: chatId,
      );
      await store.replaceChatMessages(
        profileId: profileId,
        chatId: chatId,
        messages: _visibleForBlockPolicy(cached, hiddenProfileIds, clearAll),
      );
    });
  }

  Future<void> _enqueueCacheMutation(
    Future<void> Function(MessageCacheStore store) mutation,
  ) {
    final store = _ref.read(messageCacheStoreProvider);
    return _ref
        .read(messageCacheMutationQueueProvider)
        .enqueue(() => mutation(store));
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
      // The visible inbox is owned by InboxReconciler, not the legacy
      // ChatListController cache updated below. Re-read Chat's durable row so
      // the badge and preview converge after a successful REST MarkRead.
      _ref.read(inboxReconcilerProvider.notifier).reconcileAfterMutation();
      if (state.lastMessageId == lastId) {
        final wasListed = _ref
            .read(chatListControllerProvider.notifier)
            .markChatRead(chatId);
        if (!wasListed) _invalidateChatLists(_ref);
      } else {
        _invalidateChatLists(_ref);
      }
    }
  }

  Future<String?> editMessage(String messageId, String content) async {
    final trimmed = content.trim();
    if (trimmed.isEmpty) return null;
    final auth = _ref.read(authorizationHeaderProvider);
    final profileId = _activeProfileId();
    final generation = _loadGeneration;
    if (auth == null || profileId == null) return 'not_authenticated';

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
          outbound = await _ref
              .read(e2eMessageServiceProvider)
              .encryptOutgoing(
                localProfileId: localId,
                peerProfileId: peerId,
                plaintext: outbound,
                authorization: auth,
                chatId: chatId,
              );
          if (!_isCurrentMutation(
            profileId: profileId,
            authorization: auth,
            generation: generation,
          )) {
            return null;
          }
        } on E2eEncryptException catch (e) {
          if (!_isCurrentMutation(
            profileId: profileId,
            authorization: auth,
            generation: generation,
          )) {
            return null;
          }
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
    if (!_isCurrentMutation(
      profileId: profileId,
      authorization: auth,
      generation: generation,
    )) {
      return null;
    }
    return switch (result) {
      MessagesApiOk(:final data) => _replaceMessage(data, profileId: profileId),
      MessagesApiFailure(:final message) => message,
    };
  }

  Future<String?> toggleReaction(
    String messageId,
    String emoji, {
    required bool currentlyReacted,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    final profileId = _activeProfileId();
    final generation = _loadGeneration;
    if (auth == null || profileId == null) return 'not_authenticated';
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
    if (!_isCurrentMutation(
      profileId: profileId,
      authorization: auth,
      generation: generation,
    )) {
      return null;
    }
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

  Future<String?> togglePin(
    String messageId, {
    required bool currentlyPinned,
  }) async {
    final result = await togglePinWithResult(
      messageId,
      currentlyPinned: currentlyPinned,
    );
    return result.message;
  }

  Future<PinMutationResult> togglePinWithResult(
    String messageId, {
    required bool currentlyPinned,
  }) async {
    final initialSession = _ref.read(authControllerProvider).session;
    final auth = _ref.read(authorizationHeaderProvider);
    final profileId = _activeProfileId();
    final generation = _loadGeneration;
    final refreshToken = initialSession?.refreshToken;
    if (auth == null || profileId == null) {
      return const PinMutationResult.failure(message: 'not_authenticated');
    }
    _applyPinDelta(messageId: messageId, pinned: !currentlyPinned);
    final optimisticPinnedMessages = state.pinnedMessages;
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
    if (!_isCurrentMutation(
      profileId: profileId,
      authorization: auth,
      generation: generation,
    )) {
      final currentSession = _ref.read(authControllerProvider).session;
      final currentProfileId = _activeProfileId();
      final currentAuth = _ref.read(authorizationHeaderProvider);
      final sameProfileRefreshed =
          mounted &&
          currentProfileId == profileId &&
          currentAuth != null &&
          (currentAuth != auth || currentSession?.refreshToken != refreshToken);
      if (sameProfileRefreshed) {
        // The old mutation result is no longer authoritative after token
        // rotation. Roll back only our own optimistic snapshot; if a newer
        // same-viewer pin update already replaced it, preserve that update
        // while a post-mutation read reconciles the current server state.
        if (identical(state.pinnedMessages, optimisticPinnedMessages)) {
          _applyPinDelta(messageId: messageId, pinned: currentlyPinned);
        }
        final refreshedAuth = currentAuth;
        final refreshedProfileId = currentProfileId!;
        final currentGeneration = _loadGeneration;
        await _refreshPinnedMessages(
          refreshedAuth,
          profileId: refreshedProfileId,
          generation: currentGeneration,
        );
        if (!_isCurrentMutation(
          profileId: refreshedProfileId,
          authorization: refreshedAuth,
          generation: currentGeneration,
        )) {
          return const PinMutationResult.stale();
        }
        if (state.pinnedMessagesStatus == PinnedMessagesLoadStatus.loaded) {
          final authoritativePinned = state.pinnedMessages.any(
            (message) => message.id == messageId,
          );
          _applyPinDelta(messageId: messageId, pinned: authoritativePinned);
          if (authoritativePinned == !currentlyPinned) {
            return const PinMutationResult.success();
          }
          return const PinMutationResult.failure(message: 'unknown_error');
        }
      }
      return const PinMutationResult.stale();
    }
    switch (result) {
      case MessagesApiOk<void>():
        unawaited(
          _refreshPinnedMessages(
            auth,
            profileId: profileId,
            generation: generation,
          ),
        );
        return const PinMutationResult.success();
      case MessagesApiFailure(
        :final message,
        :final errorCode,
        :final statusCode,
      ):
        unawaited(loadInitial());
        state = state.copyWith(errorMessage: message);
        return PinMutationResult.failure(
          message: message,
          errorCode: errorCode,
          statusCode: statusCode,
        );
    }
  }

  void _applyPinDelta({required String messageId, required bool pinned}) {
    final updatedMessages = state.messages
        .map((m) => m.id == messageId ? m.copyWith(isPinned: pinned) : m)
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
              MessageReaction(emoji: emoji, count: 1, reactedByMe: reactedByMe),
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
    final profileId = _activeProfileId();
    final generation = _loadGeneration;
    if (auth == null || profileId == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .deleteMessage(
          authorization: auth,
          messageId: messageId,
          scope: forMe ? 'me' : 'everyone',
        );
    if (!_isCurrentMutation(
      profileId: profileId,
      authorization: auth,
      generation: generation,
    )) {
      return null;
    }
    switch (result) {
      case MessagesApiOk<void>():
        final messages = state.messages
            .where((m) => m.id != messageId)
            .toList();
        state = state.copyWith(messages: messages, clearError: true);
        unawaited(_writeCache(messages, profileId: profileId));
        _invalidateChatLists(_ref);
        return null;
      case MessagesApiFailure(:final message):
        state = state.copyWith(errorMessage: message);
        return message;
    }
  }

  String? _replaceMessage(VoiceMessage message, {required String profileId}) {
    final messages = state.messages
        .map((m) => m.id == message.id ? message : m)
        .toList();
    final visible = _visibleForCurrentBlockPolicy(messages);
    state = state.copyWith(messages: visible, clearError: true);
    unawaited(_writeCache(visible, profileId: profileId));
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
    final sorted = _visibleForCurrentBlockPolicy(_sortMessages(merged));
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
abstract interface class RealtimeTransportFactory {
  Future<VoiceRealtimeConnection> open({
    required Uri uri,
    required AuthSession session,
  });
}

class _GatewayRealtimeTransportFactory implements RealtimeTransportFactory {
  _GatewayRealtimeTransportFactory(this._ref);

  final Ref _ref;

  @override
  Future<VoiceRealtimeConnection> open({
    required Uri uri,
    required AuthSession session,
  }) async {
    String? wsTicket;
    if (kIsWeb) {
      final gatewayClient = _ref.read(voiceGatewayClientProvider);
      final issued = await gatewayClient.requestWsTicket(
        session.authorizationHeader,
      );
      if (issued == null) {
        throw StateError('realtime_ticket_unavailable');
      }
      wsTicket = issued.ticket;
    }
    return VoiceRealtimeConnection(
      uri: uri,
      headers: <String, String>{
        if (!kIsWeb) 'Authorization': session.authorizationHeader,
        if (!kIsWeb) 'X-Voice-Profile-Id': session.activeProfileId,
        if (!kIsWeb) 'X-Request-Id': newGatewayRequestId(),
      },
      wsTicket: wsTicket,
    );
  }
}

final realtimeTransportFactoryProvider = Provider<RealtimeTransportFactory>((
  ref,
) {
  return _GatewayRealtimeTransportFactory(ref);
});

class _RealtimeHubBinding {
  const _RealtimeHubBinding({required this.generation, required this.session});

  final int generation;
  final AuthSession session;
}

/// Immutable proof that one current Realtime transport accepted its `hello`.
class RealtimeHelloBinding {
  const RealtimeHelloBinding({
    required this.generation,
    required this.bindingGeneration,
    required this.profileId,
    required this.authorization,
  });

  final int generation;
  final int bindingGeneration;
  final String profileId;
  final String authorization;
}

/// A Realtime frame tied to the immutable binding that accepted its transport.
class ProfileBoundRealtimeFrame {
  const ProfileBoundRealtimeFrame({required this.frame, required this.binding});

  final RealtimeFrame frame;
  final RealtimeHelloBinding binding;
}

class RealtimeHub {
  RealtimeHub(this._ref, {RealtimeTransportFactory? transportFactory})
    : _transportFactory =
          transportFactory ?? _GatewayRealtimeTransportFactory(_ref);

  final Ref _ref;
  final RealtimeTransportFactory _transportFactory;
  RealtimeTransport? _connection;
  StreamSubscription<RealtimeFrame>? _frameSub;
  final _eventController = StreamController<RealtimeFrame>.broadcast();
  final _profileBoundEventController =
      StreamController<ProfileBoundRealtimeFrame>.broadcast();
  var _status = RealtimeLinkStatus.disconnected;
  final _subscribedChats = <String>{};
  Timer? _reconnectTimer;
  var _reconnectAttempt = 0;
  var _disposed = false;
  var _nextGeneration = 0;
  var _nextHelloGeneration = 0;
  _RealtimeHubBinding? _binding;
  _RealtimeHubBinding? _connectingBinding;
  RealtimeTransport? _helloAcceptedConnection;
  _RealtimeHubBinding? _helloAcceptedBinding;
  RealtimeHelloBinding? _helloBinding;
  int? _lastSequence;
  int? _resumeLastSequence;
  Future<void>? _manualRetry;

  RealtimeLinkStatus get status => _status;
  Stream<RealtimeFrame> get events => _eventController.stream;
  Stream<ProfileBoundRealtimeFrame> get profileBoundEvents =>
      _profileBoundEventController.stream;

  Future<void> ensureConnected() async {
    if (_disposed) return;
    if (_eventController.isClosed ||
        _connection != null ||
        _connectingBinding != null) {
      return;
    }
    final auth = _ref.read(authControllerProvider).session;
    final config = _ref.read(gatewayConfigProvider);
    if (auth == null || !config.hasBaseUrl) return;
    final binding = _activateBinding(auth);
    await _connect(binding, config.baseUrl);
  }

  bool get canRetryCurrentSession =>
      !_disposed &&
      _manualRetry == null &&
      _connectingBinding == null &&
      (_connection == null || identical(_helloAcceptedConnection, _connection));

  /// Immediately retries the currently authenticated session without using
  /// the profile-switch path or resetting automatic backoff state.
  Future<void> retryCurrentSession() {
    final pending = _manualRetry;
    if (pending != null) return pending;
    if (!canRetryCurrentSession) return Future<void>.value();
    final session = _ref.read(authControllerProvider).session;
    final config = _ref.read(gatewayConfigProvider);
    if (session == null || !config.hasBaseUrl) return Future<void>.value();
    if (_status == RealtimeLinkStatus.connected) return Future<void>.value();

    final currentBinding = _binding;
    final binding = currentBinding != null && currentBinding.session == session
        ? currentBinding
        : _activateBinding(session);
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    final operation = () async {
      await _tearDownConnection(preserveLastSequence: true);
      if (!_isCurrent(binding) ||
          _ref.read(authControllerProvider).session != session) {
        return;
      }
      final latestConfig = _ref.read(gatewayConfigProvider);
      if (!latestConfig.hasBaseUrl) return;
      await _connect(binding, latestConfig.baseUrl, isReconnectAttempt: true);
    }();
    _manualRetry = operation;
    return operation.whenComplete(() {
      if (identical(_manualRetry, operation)) _manualRetry = null;
    });
  }

  _RealtimeHubBinding _activateBinding(AuthSession session, {int? generation}) {
    final binding = _RealtimeHubBinding(
      generation: generation ?? ++_nextGeneration,
      session: session,
    );
    _nextGeneration = _nextGeneration < binding.generation
        ? binding.generation
        : _nextGeneration;
    _binding = binding;
    return binding;
  }

  bool _isCurrent(_RealtimeHubBinding binding) =>
      !_disposed && identical(_binding, binding);

  bool _isActive(_RealtimeHubBinding binding, RealtimeTransport connection) =>
      _isCurrent(binding) && identical(_connection, connection);

  Future<void> _connect(
    _RealtimeHubBinding binding,
    String baseUrl, {
    bool isReconnectAttempt = false,
  }) async {
    if (!_isCurrent(binding)) return;
    _connectingBinding = binding;
    _setStatus(
      isReconnectAttempt
          ? RealtimeLinkStatus.reconnecting
          : RealtimeLinkStatus.connecting,
      binding: binding,
    );
    final uri = gatewayWebSocketUri(baseUrl);
    VoiceRealtimeConnection? connection;
    StreamSubscription<RealtimeFrame>? frameSub;
    try {
      connection = await _transportFactory.open(
        uri: uri,
        session: binding.session,
      );
      if (!_isCurrent(binding)) {
        await connection.dispose();
        return;
      }
      frameSub = connection.events.listen(
        (frame) => _onFrame(binding, connection!, frame),
        onError: (_) => _scheduleReconnect(binding, connection!),
        onDone: () => _scheduleReconnect(binding, connection!),
      );
      await connection.connect();
      if (!_isCurrent(binding)) {
        await frameSub.cancel();
        await connection.dispose();
        return;
      }
      _connection = connection;
      _frameSub = frameSub;
      _reconnectAttempt = 0;
      for (final chatId in _subscribedChats) {
        if (!_isActive(binding, connection)) return;
        connection.sendSubscribe(chatId);
      }
    } catch (_) {
      await frameSub?.cancel();
      await connection?.dispose();
      if (_isCurrent(binding)) {
        _scheduleReconnect(
          binding,
          connection,
          requiresActiveConnection: false,
        );
      }
      return;
    } finally {
      if (identical(_connectingBinding, binding)) {
        _connectingBinding = null;
      }
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

  void _onFrame(
    _RealtimeHubBinding binding,
    RealtimeTransport connection,
    RealtimeFrame frame,
  ) {
    if (!_isActive(binding, connection)) return;
    _lastSequence = RealtimeProtocol.trackSequence(
      _lastSequence,
      frame.sequence,
    );
    if (!_eventController.isClosed) {
      _eventController.add(frame);
    }
    if (frame.op == 'hello') {
      if (identical(_helloAcceptedConnection, connection)) return;
      _helloAcceptedConnection = connection;
      _helloAcceptedBinding = binding;
      _setStatus(RealtimeLinkStatus.connected, binding: binding);
      final helloBinding = RealtimeHelloBinding(
        generation: ++_nextHelloGeneration,
        bindingGeneration: binding.generation,
        profileId: binding.session.activeProfileId,
        authorization: binding.session.authorizationHeader,
      );
      _helloBinding = helloBinding;
      _ref.read(realtimeHelloBindingProvider.notifier).state = helloBinding;
      final resumeLastSequence = _resumeLastSequence;
      _resumeLastSequence = null;
      if (resumeLastSequence != null) {
        connection.sendResume(lastSequence: resumeLastSequence);
      }
      // Message catch-up after reconnect is REST-only (see ARCHITECTURE_REQUIREMENTS).
      return;
    }
    final helloBinding = _helloBinding;
    if (helloBinding != null &&
        identical(_helloAcceptedConnection, connection) &&
        identical(_helloAcceptedBinding, binding) &&
        helloBinding.bindingGeneration == binding.generation &&
        !_profileBoundEventController.isClosed) {
      _profileBoundEventController.add(
        ProfileBoundRealtimeFrame(frame: frame, binding: helloBinding),
      );
    }
  }

  void _scheduleReconnect(
    _RealtimeHubBinding binding,
    RealtimeTransport? connection, {
    bool requiresActiveConnection = true,
  }) {
    if (!_isCurrent(binding)) return;
    if (requiresActiveConnection &&
        (connection == null || !_isActive(binding, connection))) {
      return;
    }
    if (_ref.read(authControllerProvider).session == null) return;
    _setStatus(RealtimeLinkStatus.reconnecting, binding: binding);
    _reconnectTimer?.cancel();
    final delay = Duration(
      seconds: [1, 2, 4, 8, 16, 30][_reconnectAttempt.clamp(0, 5)],
    );
    _reconnectAttempt++;
    _reconnectTimer = Timer(delay, () async {
      if (!_isCurrent(binding)) return;
      if (requiresActiveConnection) {
        if (connection == null || !_isActive(binding, connection)) return;
        await _tearDownConnection(
          expected: connection,
          preserveLastSequence: true,
        );
      } else if (_connection != null) {
        return;
      }
      if (!_isCurrent(binding)) return;
      final config = _ref.read(gatewayConfigProvider);
      if (!config.hasBaseUrl) return;
      await _connect(binding, config.baseUrl, isReconnectAttempt: true);
    });
  }

  Future<void> _tearDownConnection({
    RealtimeTransport? expected,
    bool preserveLastSequence = false,
  }) async {
    if (expected != null && !identical(_connection, expected)) return;
    final connection = _connection;
    final frameSub = _frameSub;
    if (preserveLastSequence &&
        connection != null &&
        identical(_helloAcceptedConnection, connection)) {
      _resumeLastSequence = _lastSequence;
    }
    _lastSequence = null;
    if (identical(_connection, connection)) _connection = null;
    if (identical(_frameSub, frameSub)) _frameSub = null;
    if (identical(_helloAcceptedConnection, connection)) {
      final acceptedBinding = _helloAcceptedBinding;
      _helloAcceptedConnection = null;
      _helloAcceptedBinding = null;
      _helloBinding = null;
      if (acceptedBinding != null &&
          _ref.read(realtimeHelloBindingProvider)?.bindingGeneration ==
              acceptedBinding.generation) {
        _ref.read(realtimeHelloBindingProvider.notifier).state = null;
      }
    }
    await frameSub?.cancel();
    await connection?.dispose();
  }

  Future<void> disconnect() async {
    _binding = null;
    _connectingBinding = null;
    _helloAcceptedConnection = null;
    _helloAcceptedBinding = null;
    _helloBinding = null;
    _lastSequence = null;
    _resumeLastSequence = null;
    if (!_disposed) {
      _ref.read(realtimeHelloBindingProvider.notifier).state = null;
    }
    _reconnectTimer?.cancel();
    _reconnectAttempt = 0;
    _subscribedChats.clear();
    await _tearDownConnection();
    _setStatus(RealtimeLinkStatus.disconnected);
  }

  Set<String> get subscribedChatIds => Set.unmodifiable(_subscribedChats);

  Future<void> retireAndReconnect({
    required int generation,
    required AuthSession session,
    required Set<String> retiredSubscriptionIds,
  }) async {
    _subscribedChats.removeAll(retiredSubscriptionIds);
    final binding = _activateBinding(session, generation: generation);
    await _reconnect(binding);
  }

  /// Reconnect WebSocket after profile switch (new JWT, same subscriptions).
  Future<void> reconnectWithNewSession() async {
    final auth = _ref.read(authControllerProvider).session;
    if (auth == null) return;
    await _reconnect(_activateBinding(auth), isReconnectAttempt: true);
  }

  Future<void> _reconnect(
    _RealtimeHubBinding binding, {
    bool isReconnectAttempt = false,
  }) async {
    if (_disposed) return;
    _reconnectTimer?.cancel();
    _reconnectAttempt = 0;
    _lastSequence = null;
    _resumeLastSequence = null;
    await _tearDownConnection();
    if (!_isCurrent(binding)) return;
    final config = _ref.read(gatewayConfigProvider);
    if (!config.hasBaseUrl) return;
    await _connect(
      binding,
      config.baseUrl,
      isReconnectAttempt: isReconnectAttempt,
    );
  }

  void _setStatus(RealtimeLinkStatus next, {_RealtimeHubBinding? binding}) {
    if (_disposed) return;
    if (binding != null && !_isCurrent(binding)) return;
    _status = next;
    _ref.read(realtimeLinkStatusProvider.notifier).state = next;
  }

  Future<void> dispose() async {
    _disposed = true;
    await disconnect();
    await _eventController.close();
    await _profileBoundEventController.close();
  }
}

/// When false, [RealtimeHub] does not open WebSocket (widget tests).
final realtimeAutoConnectProvider = Provider<bool>((ref) => true);

final realtimeHubProvider = Provider<RealtimeHub>((ref) {
  final hub = RealtimeHub(
    ref,
    transportFactory: ref.watch(realtimeTransportFactoryProvider),
  );
  final autoConnect = ref.watch(realtimeAutoConnectProvider);
  ref.onDispose(hub.dispose);
  ref.listen<AuthState>(authControllerProvider, (prev, next) {
    if (!autoConnect) return;
    if (next.isAuthenticated && !(prev?.isAuthenticated ?? false)) {
      unawaited(hub.ensureConnected());
    } else if (next.isAuthenticated && (prev?.isAuthenticated ?? false)) {
      final previousSession = prev?.session;
      final nextSession = next.session;
      if (previousSession != null &&
          nextSession != null &&
          previousSession.accountId == nextSession.accountId &&
          previousSession.activeProfileId == nextSession.activeProfileId &&
          previousSession.accessToken != nextSession.accessToken) {
        unawaited(hub.reconnectWithNewSession());
      }
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

final realtimeHelloBindingProvider = StateProvider<RealtimeHelloBinding?>(
  (ref) => null,
);

/// Frames emitted only after an active Realtime transport accepts `hello`.
final profileBoundRealtimeEventProvider =
    StreamProvider<ProfileBoundRealtimeFrame>((ref) {
      return ref.watch(realtimeHubProvider).profileBoundEvents;
    });

/// Debounced reconnect banner per [ARCHITECTURE_REQUIREMENTS.md]:
/// show 2s after disconnect, hide 1s after successful reconnect.
const reconnectBannerShowDelay = Duration(seconds: 2);
const reconnectBannerHideDelay = Duration(seconds: 1);

class NetworkStatusBannerState {
  const NetworkStatusBannerState({
    this.visible = false,
    this.dismissed = false,
    this.retrying = false,
  });

  final bool visible;
  final bool dismissed;
  final bool retrying;
}

class ReconnectBannerController extends Notifier<NetworkStatusBannerState> {
  Timer? _showTimer;
  Timer? _hideTimer;
  var _wasConnected = false;
  var _disposed = false;

  bool _isOffline = false;

  @override
  NetworkStatusBannerState build() {
    ref.onDispose(() {
      _disposed = true;
      _cancelTimers();
    });
    final initialStatus = ref.read(realtimeLinkStatusProvider);
    _isOffline = ref.read(isDeviceOfflineProvider);
    if (initialStatus == RealtimeLinkStatus.connected) {
      _wasConnected = true;
    } else if (_isUnhealthy(initialStatus)) {
      _scheduleShow();
    }

    ref.listen<RealtimeLinkStatus>(realtimeLinkStatusProvider, (prev, next) {
      _onLinkStatusChanged(prev, next);
    });
    ref.listen<AuthState>(authControllerProvider, (previous, next) {
      if (previous?.session == next.session) return;
      _cancelTimers();
      _wasConnected =
          ref.read(realtimeLinkStatusProvider) == RealtimeLinkStatus.connected;
      state = NetworkStatusBannerState(
        visible:
            _isOffline || _isUnhealthy(ref.read(realtimeLinkStatusProvider)),
      );
    });
    ref.listen<bool>(isDeviceOfflineProvider, (_, offline) {
      _isOffline = offline;
      if (offline) {
        _showTimer?.cancel();
        _showTimer = null;
        _hideTimer?.cancel();
        _hideTimer = null;
        state = NetworkStatusBannerState(
          visible: !state.dismissed,
          dismissed: state.dismissed,
          retrying: state.retrying,
        );
      } else if (_isUnhealthy(ref.read(realtimeLinkStatusProvider))) {
        state = NetworkStatusBannerState(
          visible: !state.dismissed,
          dismissed: state.dismissed,
          retrying: state.retrying,
        );
      } else if (!_isUnhealthy(ref.read(realtimeLinkStatusProvider))) {
        state = NetworkStatusBannerState(
          visible: false,
          dismissed: false,
          retrying: state.retrying,
        );
      }
    });
    return NetworkStatusBannerState(visible: _isOffline);
  }

  void dismiss() {
    _showTimer?.cancel();
    _showTimer = null;
    _hideTimer?.cancel();
    _hideTimer = null;
    state = NetworkStatusBannerState(dismissed: true, retrying: state.retrying);
  }

  Future<void> retry() async {
    if (_isOffline || state.retrying) return;
    final auth = ref.read(authControllerProvider).session;
    final hub = ref.read(realtimeHubProvider);
    if (auth == null || !hub.canRetryCurrentSession) return;
    state = NetworkStatusBannerState(
      visible: state.visible,
      dismissed: state.dismissed,
      retrying: true,
    );
    try {
      await hub.retryCurrentSession();
    } finally {
      if (!_disposed && ref.read(authControllerProvider).session == auth) {
        state = NetworkStatusBannerState(
          visible: state.visible,
          dismissed: state.dismissed,
          retrying: false,
        );
      }
    }
  }

  void _setVisible(bool visible, {bool? dismissed}) {
    state = NetworkStatusBannerState(
      visible: visible,
      dismissed: dismissed ?? state.dismissed,
      retrying: state.retrying,
    );
  }

  void _cancelTimers() {
    _showTimer?.cancel();
    _showTimer = null;
    _hideTimer?.cancel();
    _hideTimer = null;
  }

  bool _isUnhealthy(RealtimeLinkStatus status) {
    return status == RealtimeLinkStatus.reconnecting ||
        (status == RealtimeLinkStatus.connecting && _wasConnected);
  }

  void _scheduleShow() {
    _showTimer?.cancel();
    _showTimer = Timer(reconnectBannerShowDelay, () {
      _showTimer = null;
      if (_disposed) return;
      if (_isUnhealthy(ref.read(realtimeLinkStatusProvider))) {
        if (!state.dismissed && !_isOffline) _setVisible(true);
      }
    });
  }

  void _onLinkStatusChanged(RealtimeLinkStatus? prev, RealtimeLinkStatus next) {
    if (next == RealtimeLinkStatus.connected) {
      _wasConnected = true;
      _showTimer?.cancel();
      _showTimer = null;
      if (state.visible || state.dismissed) {
        _hideTimer?.cancel();
        _hideTimer = Timer(reconnectBannerHideDelay, () {
          _hideTimer = null;
          if (_disposed || _isOffline) return;
          _setVisible(false, dismissed: false);
        });
      } else {
        _hideTimer?.cancel();
        _hideTimer = null;
      }
      return;
    }

    if (next == RealtimeLinkStatus.disconnected) {
      _cancelTimers();
      _wasConnected = false;
      _setVisible(_isOffline, dismissed: false);
      return;
    }

    if (next == RealtimeLinkStatus.connecting && !_wasConnected) {
      return;
    }

    final lostConnection = prev == RealtimeLinkStatus.connected;
    if (_isUnhealthy(next) &&
        (lostConnection || (!state.visible && _showTimer == null))) {
      _hideTimer?.cancel();
      _hideTimer = null;
      _scheduleShow();
    }
  }
}

final reconnectBannerVisibleProvider =
    NotifierProvider<ReconnectBannerController, NetworkStatusBannerState>(
      ReconnectBannerController.new,
    );

final realtimeEventProvider = StreamProvider<RealtimeFrame>((ref) {
  final hub = ref.watch(realtimeHubProvider);
  return hub.events;
});

class ChatActions {
  ChatActions(this._ref);

  final Ref _ref;

  Future<String?> openDmWithProfile(String otherProfileId) async {
    final session = _ref.read(authControllerProvider).session;
    if (session == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .createDm(
          authorization: session.authorizationHeader,
          otherProfileId: otherProfileId,
        );
    final current = _ref.read(authControllerProvider).session;
    if (current?.activeProfileId != session.activeProfileId ||
        current?.authorizationHeader != session.authorizationHeader) {
      return kChatActionStaleContext;
    }
    return switch (result) {
      ChatsApiOk(:final data) => () {
        final reconciler = _ref.read(inboxReconcilerProvider.notifier);
        final error = _selectDmChat(data.id, otherProfileId);
        if (error == null) unawaited(reconciler.reconcile());
        return error;
      }(),
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
    final inviteResult = await _ref
        .read(voiceChatsClientProvider)
        .addGroupMembers(
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
    _ref.read(mobileOpenedChatStripProvider.notifier).openChat(chatId);
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

  /// Forwards a message into another chat. Set [withoutAttribution] for FW-03
  /// copy-as-new (regular message, no Forwarded-from).
  Future<String?> forwardMessage({
    required String sourceMessageId,
    required String targetChatId,
    String? commentary,
    bool withoutAttribution = false,
    String? expectedProfileId,
    String? expectedAuthorization,
  }) async {
    final session = _ref.read(authControllerProvider).session;
    if (expectedProfileId != null || expectedAuthorization != null) {
      if (session?.activeProfileId != expectedProfileId ||
          session?.authorizationHeader != expectedAuthorization) {
        return kChatActionStaleContext;
      }
    }
    final auth =
        expectedAuthorization ?? _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final trimmedCommentary = commentary?.trim();
    final result = await _ref
        .read(voiceMessagesClientProvider)
        .forwardMessage(
          authorization: auth,
          sourceMessageId: sourceMessageId,
          targetChatId: targetChatId,
          commentary: trimmedCommentary == null || trimmedCommentary.isEmpty
              ? null
              : trimmedCommentary,
          withoutAttribution: withoutAttribution,
        );
    if (expectedProfileId != null || expectedAuthorization != null) {
      final current = _ref.read(authControllerProvider).session;
      if (current?.activeProfileId != expectedProfileId ||
          current?.authorizationHeader != expectedAuthorization) {
        return kChatActionStaleContext;
      }
    }
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

  /// FW-05: forward several messages into one target. Commentary is attached
  /// only to the first RPC (optional note before the batch).
  Future<String?> forwardMessages({
    required List<String> sourceMessageIds,
    required String targetChatId,
    String? commentary,
    bool withoutAttribution = false,
    String? expectedProfileId,
    String? expectedAuthorization,
  }) async {
    if (sourceMessageIds.isEmpty) return null;
    for (var i = 0; i < sourceMessageIds.length; i++) {
      final err = await forwardMessage(
        sourceMessageId: sourceMessageIds[i],
        targetChatId: targetChatId,
        commentary: i == 0 ? commentary : null,
        withoutAttribution: withoutAttribution,
        expectedProfileId: expectedProfileId,
        expectedAuthorization: expectedAuthorization,
      );
      if (err != null) return err;
    }
    return null;
  }

  Future<String?> removeGroupMember({
    required String chatId,
    required String profileId,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .removeGroupMember(
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
    final result = await _ref
        .read(voiceChatsClientProvider)
        .leaveGroup(authorization: auth, chatId: chatId);
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
    ChatsApiFailure(:final statusCode) when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    ChatsApiFailure(:final message) => throw Exception(message),
  };
});
