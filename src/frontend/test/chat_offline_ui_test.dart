import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/markdown_test_helpers.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget offlineChatApp({
    required InMemoryMessageCacheStore cache,
    required Widget home,
    bool isOffline = true,
    RealtimeLinkStatus realtimeStatus = RealtimeLinkStatus.disconnected,
    bool canRetryCurrentSession = false,
    VoidCallback? onRetryCurrentSession,
  }) {
    return ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        messageCacheStoreProvider.overrideWithValue(cache),
        isDeviceOfflineProvider.overrideWith((ref) => isOffline),
        realtimeLinkStatusProvider.overrideWith((ref) => realtimeStatus),
        realtimeHubProvider.overrideWith(
          (ref) => _NoopRealtimeHub(
            ref,
            canRetryCurrentSession: canRetryCurrentSession,
            onRetryCurrentSession: onRetryCurrentSession,
          ),
        ),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: home),
      ),
    );
  }

  testWidgets('shows cached messages and offline banner when offline', (
    tester,
  ) async {
    final cache = InMemoryMessageCacheStore();
    await cache.replaceChatMessages(
      profileId: 'prof-test',
      chatId: 'chat-offline',
      messages: [
        VoiceMessage(
          id: 'msg-offline',
          chatId: 'chat-offline',
          senderProfileId: 'peer-1',
          content: 'cached body',
          createdAt: DateTime.parse('2024-01-01T00:00:00Z'),
        ),
      ],
    );

    await tester.pumpWidget(
      offlineChatApp(
        cache: cache,
        home: const ChatRoomPanel(chatId: 'chat-offline'),
      ),
    );
    await tester.pumpAndSettle();

    expectMessagePlainText(tester, 'cached body');
    expect(find.byKey(ChatRoomPanel.offlineBannerKey), findsOneWidget);
    expect(
      find.text("You're offline. Showing saved messages."),
      findsOneWidget,
    );
  });

  testWidgets('blocks composer controls while offline', (tester) async {
    final cache = InMemoryMessageCacheStore();
    await cache.replaceChatMessages(
      profileId: 'prof-test',
      chatId: 'chat-offline',
      messages: [
        VoiceMessage(
          id: 'msg-offline',
          chatId: 'chat-offline',
          senderProfileId: 'peer-1',
          content: 'cached body',
          createdAt: DateTime.parse('2024-01-01T00:00:00Z'),
        ),
      ],
    );

    await tester.pumpWidget(
      offlineChatApp(
        cache: cache,
        home: const ChatRoomPanel(chatId: 'chat-offline'),
      ),
    );
    await tester.pumpAndSettle();

    final input = tester.widget<TextField>(
      find.descendant(
        of: find.byKey(ChatRoomPanel.inputKey),
        matching: find.byType(TextField),
      ),
    );
    expect(input.readOnly, isTrue);

    final attach = tester.widget<IconButton>(
      find.byKey(ChatRoomPanel.attachKey),
    );
    expect(attach.onPressed, isNull);
  });

  testWidgets('offline chat banner explains the blocked send state', (
    tester,
  ) async {
    await tester.pumpWidget(
      offlineChatApp(
        cache: InMemoryMessageCacheStore(),
        home: const ChatRoomPanel(chatId: 'chat-offline'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRoomPanel.offlineBannerKey), findsOneWidget);
    expect(find.text("Can't send messages while offline."), findsOneWidget);
  });

  testWidgets(
    'offline chat banner dismisses while reconnect monitoring remains active',
    (tester) async {
      await tester.pumpWidget(
        offlineChatApp(
          cache: InMemoryMessageCacheStore(),
          home: const ChatRoomPanel(chatId: 'chat-offline'),
        ),
      );
      await tester.pumpAndSettle();

      final offlineBanner = find.byKey(ChatRoomPanel.offlineBannerKey);
      expect(offlineBanner, findsOneWidget);
      final dismiss = find.descendant(
        of: offlineBanner,
        matching: find.byTooltip('Close'),
      );
      expect(dismiss, findsOneWidget);
      await tester.tap(dismiss);
      await tester.pump();
      expect(offlineBanner, findsNothing);

      final container = ProviderScope.containerOf(
        tester.element(find.byType(ChatRoomPanel)),
      );
      container.read(isDeviceOfflineProvider.notifier).state = false;
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.reconnecting;
      await tester.pump(reconnectBannerShowDelay);
      expect(find.byKey(ChatRoomPanel.reconnectBannerKey), findsOneWidget);
    },
  );

  testWidgets(
    'online chat reconnect banner retry invokes the current session',
    (tester) async {
      var retryCalls = 0;
      await tester.pumpWidget(
        offlineChatApp(
          cache: InMemoryMessageCacheStore(),
          home: const ChatRoomPanel(chatId: 'chat-offline'),
          isOffline: false,
          realtimeStatus: RealtimeLinkStatus.reconnecting,
          canRetryCurrentSession: true,
          onRetryCurrentSession: () => retryCalls++,
        ),
      );
      await tester.pump(reconnectBannerShowDelay);

      final reconnectBanner = find.byKey(ChatRoomPanel.reconnectBannerKey);
      expect(reconnectBanner, findsOneWidget);
      final retry = find.descendant(
        of: reconnectBanner,
        matching: find.text('Try again'),
      );
      expect(retry, findsOneWidget);
      await tester.tap(retry);
      await tester.pump();
      expect(retryCalls, 1);
    },
  );
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(
    super.ref, {
    this.canRetryCurrentSession = false,
    this.onRetryCurrentSession,
  });

  @override
  final bool canRetryCurrentSession;

  final VoidCallback? onRetryCurrentSession;

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  Future<void> retryCurrentSession() async {
    onRetryCurrentSession?.call();
  }

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {}
}
