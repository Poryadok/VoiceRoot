import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/realtime_client.dart';
import 'deep_link_navigation.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'inbox_reconciler.dart';
import 'matchmaking_match_controller.dart';
import 'matchmaking_search_controller.dart';
import 'push_notification_handler.dart';

/// Plays short in-app notification sounds (no FCM).
abstract class NotificationSoundPlayer {
  void playNewMessage();
  void playReaction();

  void playMention();
}

/// Default production player — no external audio dependency; override in tests.
class NoOpNotificationSoundPlayer implements NotificationSoundPlayer {
  const NoOpNotificationSoundPlayer();

  @override
  void playNewMessage() {}

  @override
  void playReaction() {}

  @override
  void playMention() {}
}

final inAppNotificationsSoundEnabledProvider = Provider<bool>((ref) => true);

final notificationSoundPlayerProvider = Provider<NotificationSoundPlayer>(
  (ref) => const NoOpNotificationSoundPlayer(),
);

/// Client-local notification-center row. Notifications exposes no list/read
/// endpoint, so this state is derived only from delivered client events.
class InAppNotificationEntry {
  const InAppNotificationEntry({
    required this.key,
    required this.type,
    required this.chatId,
    required this.messageId,
    required this.isRead,
  });

  final String key;
  final String type;
  final String chatId;
  final String? messageId;
  final bool isRead;

  InAppNotificationEntry markRead() => InAppNotificationEntry(
    key: key,
    type: type,
    chatId: chatId,
    messageId: messageId,
    isRead: true,
  );
}

class InAppNotificationCenterState {
  const InAppNotificationCenterState({this.items = const []});

  final List<InAppNotificationEntry> items;

  int get unreadCount => items.where((item) => !item.isRead).length;
}

class InAppNotificationCenterController
    extends StateNotifier<InAppNotificationCenterState> {
  InAppNotificationCenterController()
    : super(const InAppNotificationCenterState());

  /// Returns false when this event already has a local center row.
  bool add({
    required String type,
    required String chatId,
    required String? messageId,
  }) {
    final key = '$type:$chatId:${messageId ?? ''}';
    if (state.items.any((item) => item.key == key)) return false;
    state = InAppNotificationCenterState(
      items: [
        InAppNotificationEntry(
          key: key,
          type: type,
          chatId: chatId,
          messageId: messageId,
          isRead: false,
        ),
        ...state.items,
      ],
    );
    return true;
  }

  /// `mark_read` is chat-scoped, so local rows converge with its chat badge.
  void markChatRead(String chatId) {
    state = InAppNotificationCenterState(
      items: state.items
          .map((item) => item.chatId == chatId ? item.markRead() : item)
          .toList(growable: false),
    );
  }

  void clear() => state = const InAppNotificationCenterState();
}

final inAppNotificationCenterProvider =
    StateNotifierProvider<
      InAppNotificationCenterController,
      InAppNotificationCenterState
    >((ref) {
      // Notification rows are account-local and must not survive a session change.
      ref.watch(authControllerProvider);
      return InAppNotificationCenterController();
    });

/// Listens to realtime events and updates unread badges + optional sounds.
class InAppNotificationController {
  InAppNotificationController(this._ref) {
    _eventSub = _ref.listen<AsyncValue<RealtimeFrame>>(
      realtimeEventProvider,
      (_, next) => next.whenData(_onFrame),
    );
  }

  final Ref _ref;
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _eventSub;

  void dispose() => _eventSub?.close();

  /// Applies a push notification payload using the same path as WS `notification`.
  void onPushNotificationData(
    Map<String, dynamic>? data, {
    bool navigateToChat = false,
  }) => _onNotification(data, navigateToChat: navigateToChat);

  void _onFrame(RealtimeFrame frame) {
    switch (frame.op) {
      case 'notification':
        _onNotification(frame.data);
      case 'match_found':
        _ref
            .read(matchmakingMatchControllerProvider.notifier)
            .onPushNotificationData(frame.data);
      case 'mark_read':
        _onMarkRead(frame.data);
      case 'archive_activity':
        _onArchiveActivity(frame.data);
      default:
        break;
    }
  }

  void _onNotification(
    Map<String, dynamic>? data, {
    bool navigateToChat = false,
  }) {
    if (data == null) return;
    final type = data['type'] as String?;
    if (type == 'match_found') {
      _ref
          .read(matchmakingMatchControllerProvider.notifier)
          .onPushNotificationData(data);
      return;
    }
    if (type == 'search_nudge' || type == 'search_timeout') {
      _ref
          .read(matchmakingSearchControllerProvider.notifier)
          .onPushNotificationData(data);
      return;
    }

    if (navigateToChat) {
      final normalized = data.map(
        (key, value) => MapEntry(key, value?.toString() ?? ''),
      );
      final deepLink = pushDataToDeepLinkTarget(normalized);
      if (deepLink != null) {
        unawaited(_ref.read(deepLinkNavigatorProvider).apply(deepLink));
      }
    }

    final chatId = data['chat_id'] as String?;
    if (chatId == null || chatId.isEmpty) return;

    switch (type) {
      case 'new_message':
        if (_isOwnActivity(data['sender_profile_id'] as String?)) return;
        if (!_recordCenterRow(
          type: 'new_message',
          chatId: chatId,
          data: data,
        )) {
          return;
        }
        _handleIncomingActivity(
          chatId: chatId,
          actorProfileId: data['sender_profile_id'] as String?,
          playNewMessageSound: true,
        );
      case 'reaction':
        if (_isOwnActivity(data['reactor_profile_id'] as String?)) return;
        if (!_recordCenterRow(type: 'reaction', chatId: chatId, data: data)) {
          return;
        }
        _handleIncomingActivity(
          chatId: chatId,
          actorProfileId: data['reactor_profile_id'] as String?,
          playNewMessageSound: false,
          playReactionSound: true,
        );
      case 'mention':
        if (_isOwnActivity(data['sender_profile_id'] as String?)) return;
        if (!_recordCenterRow(type: 'mention', chatId: chatId, data: data)) {
          return;
        }
        _handleIncomingActivity(
          chatId: chatId,
          actorProfileId: data['sender_profile_id'] as String?,
          playNewMessageSound: false,
          playMentionSound: true,
        );
      default:
        break;
    }
  }

  void _onMarkRead(Map<String, dynamic>? data) {
    final chatId = data?['chat_id'] as String?;
    if (chatId == null || chatId.isEmpty) return;
    _ref.read(inAppNotificationCenterProvider.notifier).markChatRead(chatId);
    unawaited(_ref.read(chatListControllerProvider.notifier).loadInitial());
  }

  bool _recordCenterRow({
    required String type,
    required String chatId,
    required Map<String, dynamic> data,
  }) => _ref
      .read(inAppNotificationCenterProvider.notifier)
      .add(
        type: type,
        chatId: chatId,
        messageId: data['message_id'] as String?,
      );

  void _onArchiveActivity(Map<String, dynamic>? data) {
    final chatId = data?['chat_id'] as String?;
    if (chatId == null || chatId.isEmpty) return;
    _ref.read(chatArchiveListControllerProvider.notifier).bumpUnread(chatId);
    _ref.read(inboxReconcilerProvider.notifier).bumpArchiveUnread(chatId);
  }

  void _handleIncomingActivity({
    required String chatId,
    required String? actorProfileId,
    bool playNewMessageSound = false,
    bool playReactionSound = false,
    bool playMentionSound = false,
  }) {
    if (_isOwnActivity(actorProfileId)) return;

    final selectedChatId = _ref.read(selectedChatIdProvider);
    if (selectedChatId == chatId) return;

    _ref.read(chatListControllerProvider.notifier).bumpUnread(chatId);

    if (!_ref.read(inAppNotificationsSoundEnabledProvider)) return;

    final player = _ref.read(notificationSoundPlayerProvider);
    if (playNewMessageSound) {
      player.playNewMessage();
    } else if (playReactionSound) {
      player.playReaction();
    } else if (playMentionSound) {
      player.playMention();
    }
  }

  bool _isOwnActivity(String? actorProfileId) {
    final activeProfile = _ref.read(authControllerProvider).activeProfileId;
    return actorProfileId != null &&
        activeProfile != null &&
        actorProfileId == activeProfile;
  }
}

final inAppNotificationControllerProvider =
    Provider<InAppNotificationController?>((ref) {
      final auth = ref.watch(authControllerProvider);
      if (!auth.isAuthenticated) return null;
      final controller = InAppNotificationController(ref);
      ref.onDispose(controller.dispose);
      return controller;
    });
