import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/ui/core/voice_badge.dart';
import 'package:voice_frontend/ui/shell/mobile_chat_strip.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

const _stripItems = [
  ChatListItem(
    chat: VoiceChat(
      id: 'chat-a',
      type: 'CHAT_TYPE_DM',
      creatorProfileId: 'peer-1',
      name: 'Alice',
    ),
    unreadCount: 3,
  ),
  ChatListItem(
    chat: VoiceChat(
      id: 'chat-b',
      type: 'CHAT_TYPE_GROUP',
      creatorProfileId: 'prof-test',
      name: 'Squad',
    ),
  ),
];

class _PresetChatListController extends ChatListController {
  _PresetChatListController(super.ref, {List<ChatListItem>? items})
    : super() {
    state = ChatListState(items: items ?? _stripItems);
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _EmptyChatListController extends _PresetChatListController {
  _EmptyChatListController(super.ref) : super(items: const []);
}

class _FakeRealtimeHub extends RealtimeHub {
  _FakeRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

List<Override> _stripOverrides() => [
  ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
  chatListControllerProvider.overrideWith(_PresetChatListController.new),
  realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
];

void main() {
  testWidgets('strip shows empty placeholder when no chats', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
        chatListControllerProvider.overrideWith(_EmptyChatListController.new),
        realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: MobileChatStrip()),
        ),
      ),
    );
    await tester.pump();

    expect(find.byKey(MobileChatStrip.stripKey), findsOneWidget);
    expect(find.byType(VoiceBadge), findsNothing);
  });

  testWidgets('strip renders chat icons with unread badges', (tester) async {
    final container = ProviderContainer(overrides: _stripOverrides());
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: MobileChatStrip()),
        ),
      ),
    );
    await tester.pump();

    expect(find.byKey(MobileChatStrip.stripKey), findsOneWidget);
    expect(find.byKey(MobileChatStrip.tileKey('chat-a')), findsOneWidget);
    expect(find.byKey(MobileChatStrip.tileKey('chat-b')), findsOneWidget);
    expect(find.byType(VoiceBadge), findsOneWidget);
    expect(find.text('3'), findsOneWidget);
  });

  testWidgets('strip chat tiles expose button semantics', (tester) async {
    final container = ProviderContainer(overrides: _stripOverrides());
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: MobileChatStrip()),
        ),
      ),
    );
    await tester.pump();

    expect(find.bySemanticsLabel(RegExp('Alice')), findsOneWidget);
    expect(find.bySemanticsLabel(RegExp('3 unread')), findsOneWidget);
  });

  testWidgets('strip chat tiles meet minimum touch target', (tester) async {
    final container = ProviderContainer(overrides: _stripOverrides());
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: MobileChatStrip()),
        ),
      ),
    );
    await tester.pump();

    final tile = tester.getSize(find.byKey(MobileChatStrip.tileKey('chat-a')));
    expect(tile.width, greaterThanOrEqualTo(44));
    expect(tile.height, greaterThanOrEqualTo(44));
  });

  testWidgets('tapping strip icon selects chat', (tester) async {
    final container = ProviderContainer(overrides: _stripOverrides());
    addTearDown(container.dispose);

    container.read(selectedChatIdProvider.notifier).state = 'chat-a';

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: MobileChatStrip()),
        ),
      ),
    );
    await tester.pump();

    await tester.tap(find.byKey(MobileChatStrip.tileKey('chat-b')));
    await tester.pump();

    expect(container.read(selectedChatIdProvider), 'chat-b');
  });
}
