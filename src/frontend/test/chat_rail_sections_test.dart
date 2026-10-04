import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/ui/shell/chat_rail_sections.dart';

import 'support/fake_voice_api_clients.dart';

void main() {
  testWidgets('ChatRailQuickAccessSection renders items and opens chat', (
    tester,
  ) async {
    const qaData = QuickAccessListData(
      items: [
        VoiceQuickAccessItem(
          chatId: 'chat-qa-1',
          chat: VoiceChat(
            id: 'chat-qa-1',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'profile-a',
            name: 'Favorite DM',
          ),
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [quickAccessListProvider.overrideWith((_) async => qaData)],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SizedBox(width: 56, child: ChatRailQuickAccessSection()),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRailQuickAccessSection.sectionKey), findsOneWidget);
    expect(
      find.byKey(ChatRailQuickAccessSection.itemKey('chat-qa-1')),
      findsOneWidget,
    );
  });

  testWidgets(
    'ChatRailQuickAccessSection persists a reordered canonical list',
    (tester) async {
      final chats = _RecordingQuickAccessClient();
      const qaData = QuickAccessListData(
        items: [
          VoiceQuickAccessItem(chatId: 'chat-qa-1'),
          VoiceQuickAccessItem(chatId: 'chat-qa-2'),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authorizationHeaderProvider.overrideWithValue('Bearer test'),
            voiceChatsClientProvider.overrideWithValue(chats),
            quickAccessListProvider.overrideWith((_) async => qaData),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(
              body: SizedBox(width: 96, child: ChatRailQuickAccessSection()),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final list = tester.widget<ReorderableListView>(
        find.byKey(ChatRailQuickAccessSection.reorderListKey),
      );
      list.onReorder(0, 2);
      await tester.pumpAndSettle();

      expect(chats.reorderedChatIds, ['chat-qa-2', 'chat-qa-1']);
    },
  );

  testWidgets('ChatRailQuickAccessSection hides action failure details', (
    tester,
  ) async {
    final chats = _RecordingQuickAccessClient()
      ..reorderError = 'private_quick_access_reorder_detail'
      ..removeError = 'private_quick_access_remove_detail';
    const qaData = QuickAccessListData(
      items: [
        VoiceQuickAccessItem(chatId: 'chat-qa-1'),
        VoiceQuickAccessItem(chatId: 'chat-qa-2'),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authorizationHeaderProvider.overrideWithValue('Bearer test'),
          voiceChatsClientProvider.overrideWithValue(chats),
          quickAccessListProvider.overrideWith((_) async => qaData),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SizedBox(width: 96, child: ChatRailQuickAccessSection()),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final list = tester.widget<ReorderableListView>(
      find.byKey(ChatRailQuickAccessSection.reorderListKey),
    );
    list.onReorder(0, 2);
    await tester.pumpAndSettle();
    expect(chats.reorderedChatIds, ['chat-qa-2', 'chat-qa-1']);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text('private_quick_access_reorder_detail'), findsNothing);
    await tester.tap(find.text('Could not complete this action.'));
    await tester.pumpAndSettle();
    await tester.pump(const Duration(seconds: 5));

    await tester.longPress(
      find.byKey(ChatRailQuickAccessSection.itemKey('chat-qa-1')),
    );
    await tester.pumpAndSettle();
    expect(chats.removedChatIds, ['chat-qa-1']);
    final l10n = AppLocalizations.of(
      tester.element(
        find.byKey(ChatRailQuickAccessSection.itemKey('chat-qa-1')),
      ),
    )!;
    expect(find.text(l10n.backendUnavailable), findsOneWidget);
    expect(find.text('private_quick_access_remove_detail'), findsNothing);
  });
}

class _RecordingQuickAccessClient extends FakeVoiceChatsClient {
  List<String>? reorderedChatIds;
  String? reorderError;
  String? removeError;
  final removedChatIds = <String>[];

  @override
  Future<ChatsApiResult<void>> reorderQuickAccess({
    required String authorization,
    required List<String> chatIds,
  }) async {
    reorderedChatIds = List.of(chatIds);
    if (reorderError case final message?) {
      return ChatsApiFailure(message: message);
    }
    return const ChatsApiOk<void>(null);
  }

  @override
  Future<ChatsApiResult<void>> removeQuickAccess({
    required String authorization,
    required String chatId,
  }) async {
    removedChatIds.add(chatId);
    if (removeError case final message?) {
      return ChatsApiFailure(message: message, statusCode: 503);
    }
    return const ChatsApiOk<void>(null);
  }
}
