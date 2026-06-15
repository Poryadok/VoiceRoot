import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/e2e_chat_settings.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

/// Phase 15 red tests: E2E opt-in/opt-out confirmation dialogs (docs/features/encryption.md).
void main() {
  Widget e2eDialogTestApp({required Widget child}) {
    return ProviderScope(
      overrides: voiceThemeTestOverrides(),
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: child),
      ),
    );
  }

  testWidgets('E2eEnableConfirmDialog shows search limitation warning', (
    tester,
  ) async {
    await tester.pumpWidget(
      e2eDialogTestApp(
        child: E2eEnableConfirmDialog(
          chatId: 'dm-chat-1',
          onConfirmed: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(E2eEnableConfirmDialog.dialogKey), findsOneWidget);
    expect(
      find.textContaining('end-to-end encryption', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.textContaining('Global search', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.textContaining('server', findRichText: true),
      findsOneWidget,
    );
    expect(find.byKey(E2eEnableConfirmDialog.confirmButtonKey), findsOneWidget);
    expect(find.byKey(E2eEnableConfirmDialog.cancelButtonKey), findsOneWidget);
  });

  testWidgets('E2eDisableConfirmDialog warns opt-out reverts to plaintext', (
    tester,
  ) async {
    await tester.pumpWidget(
      e2eDialogTestApp(
        child: E2eDisableConfirmDialog(
          chatId: 'dm-chat-1',
          onConfirmed: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(E2eDisableConfirmDialog.dialogKey), findsOneWidget);
    expect(
      find.textContaining('disable', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.textContaining('plain', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.textContaining('search', findRichText: true),
      findsOneWidget,
    );
    expect(
      find.byKey(E2eDisableConfirmDialog.confirmButtonKey),
      findsOneWidget,
    );
  });

  testWidgets('confirming enable dialog invokes onConfirmed callback', (
    tester,
  ) async {
    var confirmed = false;
    await tester.pumpWidget(
      e2eDialogTestApp(
        child: E2eEnableConfirmDialog(
          chatId: 'dm-chat-1',
          onConfirmed: () => confirmed = true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(E2eEnableConfirmDialog.confirmButtonKey));
    await tester.pumpAndSettle();

    expect(confirmed, isTrue);
  });

  testWidgets('E2eKeyBackupSheet shows password fields and save action', (
    tester,
  ) async {
    await tester.pumpWidget(
      e2eDialogTestApp(
        child: E2eKeyBackupSheet(
          onSave: (_, _) async {},
          onRestore: (_) async {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(E2eKeyBackupSheet.sheetKey), findsOneWidget);
    expect(find.byKey(E2eKeyBackupSheet.passwordFieldKey), findsOneWidget);
    expect(find.byKey(E2eKeyBackupSheet.saveButtonKey), findsOneWidget);
    expect(find.byKey(E2eKeyBackupSheet.restoreButtonKey), findsOneWidget);
  });
}
