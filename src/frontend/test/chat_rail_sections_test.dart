import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/ui/shell/chat_rail_sections.dart';

void main() {
  testWidgets('ChatRailQuickAccessSection renders items and opens chat', (tester) async {
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
        overrides: [
          quickAccessListProvider.overrideWith((_) async => qaData),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SizedBox(
              width: 56,
              child: ChatRailQuickAccessSection(),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRailQuickAccessSection.sectionKey), findsOneWidget);
    expect(find.byKey(ChatRailQuickAccessSection.itemKey('chat-qa-1')), findsOneWidget);
  });
}
