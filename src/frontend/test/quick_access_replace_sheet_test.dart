import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/shell/quick_access_replace_sheet.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('QuickAccessReplaceSheet lists slots and returns selection', (
    tester,
  ) async {
    const items = [
      VoiceQuickAccessItem(
        chatId: 'qa-1',
        chat: VoiceChat(
          id: 'qa-1',
          type: 'CHAT_TYPE_DM',
          creatorProfileId: 'p1',
          name: 'Slot One',
        ),
      ),
      VoiceQuickAccessItem(
        chatId: 'qa-2',
        chat: VoiceChat(
          id: 'qa-2',
          type: 'CHAT_TYPE_DM',
          creatorProfileId: 'p1',
          name: 'Slot Two',
        ),
      ),
    ];

    String? picked;
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) {
            return Scaffold(
              body: TextButton(
                onPressed: () async {
                  picked = await QuickAccessReplaceSheet.show(
                    context,
                    items: items,
                  );
                },
                child: const Text('open'),
              ),
            );
          },
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.byKey(QuickAccessReplaceSheet.sheetKey), findsOneWidget);
    expect(find.text('Choose what to replace'), findsOneWidget);
    expect(find.text('Slot One'), findsOneWidget);

    await tester.tap(find.text('Slot Two'));
    await tester.pumpAndSettle();

    expect(picked, 'qa-2');
  });
}
