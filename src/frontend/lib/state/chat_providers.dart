import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/api_errors.dart';
import '../backend/chats_client.dart';
import '../backend/messages_client.dart';
import '../backend/realtime_client.dart';
import 'auth_providers.dart';
import 'gateway_providers.dart';

final voiceChatsClientProvider = Provider<VoiceChatsClient>((ref) {
  return VoiceChatsClient(
    httpClient: ref.watch(httpClientProvider),
    config: ref.watch(gatewayConfigProvider),
  );
});

final voiceMessagesClientProvider = Provider<VoiceMessagesClient>((ref) {
  return VoiceMessagesClient(
    httpClient: ref.watch(httpClientProvider),
    config: ref.watch(gatewayConfigProvider),
  );
});

/// Active DM chat id in the main column, or null.
final selectedChatIdProvider = StateProvider<String?>((ref) => null);

/// Peer profile id per DM chat id (filled when opening DM from profile).
final dmPeerProfileByChatIdProvider =
    StateProvider<Map<String, String>>((ref) => {});

final chatListProvider = FutureProvider<ChatListData>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result =
      await ref.watch(voiceChatsClientProvider).listChats(authorization: auth);
  return switch (result) {
    ChatsApiOk(:final data) => data,
    ChatsApiFailure(:final statusCode) when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    ChatsApiFailure(:final message) => throw Exception(message),
  };
});

class ChatRoomState {
  const ChatRoomState({
    this.messages = const [],
    this.isLoading = false,
    this.isSending = false,
    this.errorMessage,
    this.realtimeStatus = RealtimeLinkStatus.disconnected,
  });

  final List<VoiceMessage> messages;
  final bool isLoading;
  final bool isSending;
  final String? errorMessage;
  final RealtimeLinkStatus realtimeStatus;

  String? get lastMessageId =>
      messages.isEmpty ? null : messages.last.id;

  ChatRoomState copyWith({
    List<VoiceMessage>? messages,
    bool? isLoading,
    bool? isSending,
    String? errorMessage,
    bool clearError = false,
    RealtimeLinkStatus? realtimeStatus,
  }) {
    return ChatRoomState(
      messages: messages ?? this.messages,
      isLoading: isLoading ?? this.isLoading,
      isSending: isSending ?? this.isSending,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      realtimeStatus: realtimeStatus ?? this.realtimeStatus,
    );
  }
}

enum RealtimeLinkStatus { disconnected, connecting, connected, reconnecting }

class ChatRoomController extends StateNotifier<ChatRoomState> {
  ChatRoomController(this._ref, this.chatId) : super(const ChatRoomState()) {
    _realtimeSub = _ref.listen<RealtimeLinkStatus>(
      realtimeLinkStatusProvider,
      (_, next) {
        final prev = state.realtimeStatus;
        state = state.copyWith(realtimeStatus: next);
        if (prev == RealtimeLinkStatus.reconnecting &&
            next == RealtimeLinkStatus.connected) {
          unawaited(_catchUpAfterReconnect());
        }
      },
    );
    _eventSub = _ref.listen<AsyncValue<RealtimeFrame>>(
      realtimeEventProvider,
      (_, next) {
        next.whenData((frame) {
          if (frame.op == 'message_create') {
            final chatId = frame.data?['chat_id'] as String?;
            if (chatId == this.chatId) {
              unawaited(_catchUpAfterEvent());
            }
          }
        });
      },
    );
    unawaited(loadInitial());
    _ref.read(realtimeHubProvider).ensureSubscribed(chatId);
  }

  final Ref _ref;
  final String chatId;
  ProviderSubscription<RealtimeLinkStatus>? _realtimeSub;
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _eventSub;

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
    final result = await _ref.read(voiceMessagesClientProvider).getMessages(
          authorization: auth,
          chatId: chatId,
        );
    switch (result) {
      case MessagesApiOk(:final data):
        state = state.copyWith(
          messages: data.messages,
          isLoading: false,
          clearError: true,
        );
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
    final result = await _ref.read(voiceMessagesClientProvider).getMessages(
          authorization: auth,
          chatId: chatId,
          afterMessageId: afterMessageId,
          lastMessageId: lastMessageId,
        );
    if (result case MessagesApiOk(:final data)) {
      if (data.messages.isEmpty) return;
      final merged = [...state.messages];
      for (final m in data.messages) {
        if (!merged.any((x) => x.id == m.id)) {
          merged.add(m);
        }
      }
      merged.sort((a, b) {
        final at = a.createdAt ?? DateTime.fromMillisecondsSinceEpoch(0);
        final bt = b.createdAt ?? DateTime.fromMillisecondsSinceEpoch(0);
        return at.compareTo(bt);
      });
      state = state.copyWith(messages: merged, clearError: true);
      _ref.invalidate(chatListProvider);
    }
  }

  Future<String?> sendMessage(String content) async {
    final trimmed = content.trim();
    if (trimmed.isEmpty) return null;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    state = state.copyWith(isSending: true, clearError: true);
    final result = await _ref.read(voiceMessagesClientProvider).sendMessage(
          authorization: auth,
          chatId: chatId,
          content: trimmed,
        );
    switch (result) {
      case MessagesApiOk(:final data):
        final merged = [...state.messages];
        if (!merged.any((m) => m.id == data.id)) {
          merged.add(data);
        }
        state = state.copyWith(
          messages: merged,
          isSending: false,
          clearError: true,
        );
        _ref.invalidate(chatListProvider);
        return null;
      case MessagesApiFailure(:final message):
        state = state.copyWith(isSending: false, errorMessage: message);
        return message;
    }
  }
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

  RealtimeLinkStatus get status => _status;
  Stream<RealtimeFrame> get events => _eventController.stream;

  Future<void> ensureConnected() async {
    if (_connection != null) return;
    final auth = _ref.read(authControllerProvider).session;
    final config = _ref.read(gatewayConfigProvider);
    if (auth == null || !config.hasBaseUrl) return;

    _setStatus(RealtimeLinkStatus.connecting);
    final uri = gatewayWebSocketUri(config.baseUrl);
    final headers = {
      'Authorization': auth.authorizationHeader,
      'X-Profile-Id': auth.activeProfileId,
      'X-Voice-Profile-Id': auth.activeProfileId,
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
      _scheduleReconnect();
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

  void _onFrame(RealtimeFrame frame) {
    if (!_eventController.isClosed) {
      _eventController.add(frame);
    }
    if (frame.op == 'hello') {
      _setStatus(RealtimeLinkStatus.connected);
      _connection?.sendResume();
    }
  }

  void _scheduleReconnect() {
    if (_ref.read(authControllerProvider).session == null) return;
    _setStatus(RealtimeLinkStatus.reconnecting);
    _reconnectTimer?.cancel();
    final delay = Duration(
      seconds: [1, 2, 4, 8, 16, 30][_reconnectAttempt.clamp(0, 5)],
    );
    _reconnectAttempt++;
    _reconnectTimer = Timer(delay, () async {
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

  void _setStatus(RealtimeLinkStatus next) {
    _status = next;
    _ref.read(realtimeLinkStatusProvider.notifier).state = next;
  }

  Future<void> dispose() async {
    _reconnectTimer?.cancel();
    await _tearDownConnection();
    await _eventController.close();
    _setStatus(RealtimeLinkStatus.disconnected);
  }
}

final realtimeHubProvider = Provider<RealtimeHub>((ref) {
  final hub = RealtimeHub(ref);
  ref.onDispose(hub.dispose);
  ref.listen(authControllerProvider, (prev, next) {
    if (next.isAuthenticated && !(prev?.isAuthenticated ?? false)) {
      unawaited(hub.ensureConnected());
    }
    if (!next.isAuthenticated && (prev?.isAuthenticated ?? false)) {
      unawaited(hub.dispose());
    }
  });
  if (ref.read(authControllerProvider).isAuthenticated) {
    unawaited(hub.ensureConnected());
  }
  return hub;
});

final realtimeLinkStatusProvider =
    StateProvider<RealtimeLinkStatus>((ref) => RealtimeLinkStatus.disconnected);

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
    final result = await _ref.read(voiceChatsClientProvider).createDm(
          authorization: auth,
          otherProfileId: otherProfileId,
        );
    return switch (result) {
      ChatsApiOk(:final data) => _selectDmChat(data.id, otherProfileId),
      ChatsApiFailure(:final message) => message,
    };
  }

  String? _selectDmChat(String chatId, String peerProfileId) {
    final peers = Map<String, String>.from(
      _ref.read(dmPeerProfileByChatIdProvider),
    );
    peers[chatId] = peerProfileId;
    _ref.read(dmPeerProfileByChatIdProvider.notifier).state = peers;
    _ref.read(selectedChatIdProvider.notifier).state = chatId;
    _ref.invalidate(chatListProvider);
    return null;
  }

  void selectChat(String chatId) {
    _ref.read(selectedChatIdProvider.notifier).state = chatId;
    _ref.read(realtimeHubProvider).ensureSubscribed(chatId);
  }
}

final chatActionsProvider = Provider<ChatActions>((ref) {
  return ChatActions(ref);
});
