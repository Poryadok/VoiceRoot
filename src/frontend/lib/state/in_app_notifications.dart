import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/realtime_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'matchmaking_match_controller.dart';

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
  void onPushNotificationData(Map<String, dynamic>? data) => _onNotification(data);

  void _onFrame(RealtimeFrame frame) {
    switch (frame.op) {
      case 'notification':
        _onNotification(frame.data);
      case 'match_found':
        _ref.read(matchmakingMatchControllerProvider.notifier).onPushNotificationData(frame.data);
      case 'mark_read':
        _onMarkRead(frame.data);
      default:
        break;
    }
  }

  void _onNotification(Map<String, dynamic>? data) {
    if (data == null) return;
    final type = data['type'] as String?;
    if (type == 'match_found') {
      _ref.read(matchmakingMatchControllerProvider.notifier).onPushNotificationData(data);
      return;
    }
    final chatId = data['chat_id'] as String?;
    if (chatId == null || chatId.isEmpty) return;

    switch (type) {
      case 'new_message':
        _handleIncomingActivity(
          chatId: chatId,
          actorProfileId: data['sender_profile_id'] as String?,
          playNewMessageSound: true,
        );
      case 'reaction':
        _handleIncomingActivity(
          chatId: chatId,
          actorProfileId: data['reactor_profile_id'] as String?,
          playNewMessageSound: false,
          playReactionSound: true,
        );
      case 'mention':
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
    unawaited(_ref.read(chatListControllerProvider.notifier).loadInitial());
  }

  void _handleIncomingActivity({
    required String chatId,
    required String? actorProfileId,
    bool playNewMessageSound = false,
    bool playReactionSound = false,
    bool playMentionSound = false,
  }) {
    final activeProfile = _ref.read(authControllerProvider).activeProfileId;
    if (actorProfileId != null &&
        activeProfile != null &&
        actorProfileId == activeProfile) {
      return;
    }

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
}

final inAppNotificationControllerProvider =
    Provider<InAppNotificationController?>((ref) {
      final auth = ref.watch(authControllerProvider);
      if (!auth.isAuthenticated) return null;
      final controller = InAppNotificationController(ref);
      ref.onDispose(controller.dispose);
      return controller;
    });
