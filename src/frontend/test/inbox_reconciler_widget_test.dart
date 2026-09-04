import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/inbox_reconciler_fakes.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'keeps a cached row visible while offline reconciliation exposes error and retry',
    (tester) async {
      final chats = InboxReconcilerChatsFake()
        ..enqueue(
          InboxChatPageScript(
            inbox: 'main',
            cursor: null,
            result: ChatsApiOk(
              ChatListData(items: [inboxChatItem('cached-row')]),
            ),
          ),
        );
      await tester.pumpWidget(
        _chatListApp(
          chats: chats,
          offline: true,
          initial: ChatListState(
            items: [inboxChatItem('cached-row', preview: 'saved locally')],
            errorMessage: 'offline',
            errorStatusCode: 503,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(ChatListBody.tileKey('cached-row')), findsOneWidget);
      expect(find.text('Could not load chats'), findsOneWidget);
      expect(find.text('Try again'), findsOneWidget);

      await tester.tap(find.text('Try again'));
      await tester.pumpAndSettle();
      expect(chats.calls.last.cursor, isNull);
      expect(chats.calls.last.inbox, 'main');
    },
  );

  testWidgets(
    'renders the first cached page during progressive reconciliation',
    (tester) async {
      await tester.pumpWidget(
        _chatListApp(
          chats: InboxReconcilerChatsFake(),
          initial: ChatListState(
            items: [inboxChatItem('first-page')],
            nextCursor: 'opaque-next',
            isLoading: true,
          ),
        ),
      );
      await tester.pump();

      expect(find.byKey(ChatListBody.tileKey('first-page')), findsOneWidget);
      expect(find.byKey(ChatListBody.listKey), findsOneWidget);
      expect(find.byType(VoiceListSkeleton), findsNothing);
    },
  );

  testWidgets('initial failed reconciliation shows the existing retry state', (
    tester,
  ) async {
    await tester.pumpWidget(
      _chatListApp(
        chats: InboxReconcilerChatsFake(),
        initial: const ChatListState(
          isLoading: false,
          errorMessage: 'gateway unavailable',
          errorStatusCode: 503,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatListBody.unavailableKey), findsOneWidget);
    expect(find.text('Could not load chats'), findsWidgets);
    expect(find.text('Try again'), findsOneWidget);
  });
}

Widget _chatListApp({
  required InboxReconcilerChatsFake chats,
  required ChatListState initial,
  bool offline = false,
}) {
  return ProviderScope(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceChatsClientProvider.overrideWithValue(chats),
      isDeviceOfflineProvider.overrideWith((ref) => offline),
      chatListControllerProvider.overrideWith(
        (ref) => ChatListController(ref)..state = initial,
      ),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const Scaffold(body: ChatListBody(showHeader: false)),
    ),
  );
}
