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
  }) {
    return ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        messageCacheStoreProvider.overrideWithValue(cache),
        isDeviceOfflineProvider.overrideWith((ref) => true),
        realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
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

    final input = tester.widget<TextField>(find.byKey(ChatRoomPanel.inputKey));
    expect(input.readOnly, isTrue);

    final attach = tester.widget<IconButton>(
      find.byKey(ChatRoomPanel.attachKey),
    );
    expect(attach.onPressed, isNull);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {}
}
