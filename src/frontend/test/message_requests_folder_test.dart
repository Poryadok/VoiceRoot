import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/message_requests_providers.dart';
import 'package:voice_frontend/ui/shell/message_requests_folder.dart';

void main() {
  testWidgets('MessageRequestsFolderRailButton hidden when no pending requests', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          messageRequestsSummaryProvider.overrideWith(
            (_) async => const MessageRequestsSummary(
              pendingCount: 0,
              unreadCount: 0,
            ),
          ),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: MessageRequestsFolderRailButton(
              selected: false,
              onPressed: _noop,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MessageRequestsFolderRailButton.keyId), findsNothing);
  });

  testWidgets('MessageRequestsFolderRailButton visible with unread badge', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          messageRequestsSummaryProvider.overrideWith(
            (_) async => const MessageRequestsSummary(
              pendingCount: 2,
              unreadCount: 3,
            ),
          ),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: MessageRequestsFolderRailButton(
              selected: true,
              onPressed: _noop,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MessageRequestsFolderRailButton.keyId), findsOneWidget);
    expect(find.text('3'), findsOneWidget);
  });
}

void _noop() {}
