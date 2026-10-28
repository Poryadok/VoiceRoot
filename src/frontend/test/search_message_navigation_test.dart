import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/search/global_search_panel.dart';
import 'package:voice_frontend/ui/search/in_chat_search.dart';

import 'support/auth_test_overrides.dart';
import 'support/deep_link_message_fixture.dart';
import 'support/fake_voice_api_clients.dart';
import 'support/voice_test_theme.dart';

void main() {
  Future<(ProviderContainer, _History)> mount(
    WidgetTester tester, {
    bool inChat = false,
    bool missingChatId = false,
  }) async {
    await bindDesktopTestViewport(tester);
    final history = _History();
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((request) async {
            if (request.url.path.contains('/search/')) {
              return http.Response(
                jsonEncode({
                  inChat ? 'search_results' : 'global_search_results': {
                    'hits': [
                      {
                        'message_id': 'historic-target',
                        'snippet': 'needle result',
                        'score': 1,
                      },
                    ],
                    'messages': [
                      {
                        'message_id': 'historic-target',
                        'chat_id': missingChatId ? '' : 'search-chat',
                        'snippet': 'needle result',
                        'score': 1,
                      },
                    ],
                    'matched_chats': [
                      {'id': 'wrong-chat'},
                    ],
                  },
                }),
                200,
              );
            }
            return http.Response('{}', 200);
          }),
        ),
        voiceMessagesClientProvider.overrideWithValue(history),
      ],
    );
    addTearDown(container.dispose);
    if (inChat) {
      container.read(selectedChatIdProvider.notifier).state = 'search-chat';
    }
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Row(
              children: [
                if (!inChat)
                  const SizedBox(width: 320, child: GlobalSearchPanel()),
                Expanded(
                  child: Consumer(
                    builder: (context, ref, _) {
                      final chatId = ref.watch(selectedChatIdProvider);
                      return chatId == null
                          ? const SizedBox()
                          : ChatRoomPanel(
                              key: ValueKey(chatId),
                              chatId: chatId,
                            );
                    },
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return (container, history);
  }

  testWidgets(
    'global message hit loads its chat cursor and exact historic row',
    (tester) async {
      final (container, history) = await mount(tester);
      await tester.enterText(
        find.byKey(GlobalSearchPanel.searchFieldKey),
        'needle',
      );
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();
      await tester.tap(find.text('historic-target'));
      await tester.pumpAndSettle();
      expect(container.read(selectedChatIdProvider), 'search-chat');
      expect(history.olderRequests, [('search-chat', 'older-search-chat')]);
      await expectDeepLinkMessageVisible(
        tester,
        container: container,
        chatId: 'search-chat',
        messageId: 'historic-target',
      );
      await tester.pump(const Duration(seconds: 2));
    },
  );

  testWidgets('global hit without chat identity cannot guess a matched chat', (
    tester,
  ) async {
    final (container, _) = await mount(tester, missingChatId: true);
    await tester.enterText(
      find.byKey(GlobalSearchPanel.searchFieldKey),
      'needle',
    );
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();
    await tester.tap(find.text('historic-target'));
    await tester.pumpAndSettle();
    expect(container.read(selectedChatIdProvider), isNull);
  });

  testWidgets(
    'in-chat result selection loads exact historic row and flashes it',
    (tester) async {
      final (container, history) = await mount(tester, inChat: true);
      await tester.tap(find.byKey(ChatRoomPanel.inChatSearchKey));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'needle');
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();
      expect(
        find.byKey(InChatSearch.activeHitKey('historic-target')),
        findsOneWidget,
      );
      await tester.tap(
        find.byKey(InChatSearch.activeHitKey('historic-target')),
      );
      await tester.pumpAndSettle();
      expect(history.olderRequests, [('search-chat', 'older-search-chat')]);
      await expectDeepLinkMessageVisible(
        tester,
        container: container,
        chatId: 'search-chat',
        messageId: 'historic-target',
      );
      final text = find.text('Historic needle message', findRichText: true);
      final highlight = find
          .ancestor(of: text, matching: find.byType(AnimatedContainer))
          .first;
      expect(tester.widget<AnimatedContainer>(highlight).decoration, isNotNull);
      expect(
        container.read(pendingChatMessageHighlightProvider('search-chat')),
        isNull,
      );
      await tester.pump(const Duration(seconds: 2));
      await tester.pumpAndSettle();
      expect(tester.widget<AnimatedContainer>(highlight).decoration, isNull);
    },
  );
}

class _History extends FakeVoiceMessagesClient {
  final olderRequests = <(String, String?)>[];

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) async {
    if (cursor != null) {
      olderRequests.add((chatId, cursor));
      return MessagesApiOk(
        MessageListData(
          messages: [
            VoiceMessage(
              id: 'historic-target',
              chatId: chatId,
              senderProfileId: 'peer',
              content: 'Historic needle message',
              createdAt: DateTime.utc(2024),
            ),
          ],
        ),
      );
    }
    return MessagesApiOk(
      MessageListData(
        messages: List.generate(
          25,
          (index) => VoiceMessage(
            id: 'recent-$index',
            chatId: chatId,
            senderProfileId: 'peer',
            content: 'Recent message $index',
            createdAt: DateTime.utc(2024, 1, 2, 0, index),
          ),
        ),
        nextCursor: 'older-search-chat',
        hasMore: true,
      ),
    );
  }
}
