import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/composer_send_menu.dart';

import 'support/voice_test_theme.dart';

/// Fidelity for the reviewed v3 `Panel / Chat / SendMenu` mockup (§3.6c,
/// H+V): DM shows all three send actions, a group chat hides the
/// when-online action, and the mobile group bottom sheet explains why
/// with a dm-only notice.
void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  for (final width in [1280.0, 390.0]) {
    for (final isDm in [true, false]) {
      testWidgets(
        'v3 send menu $width isDm=$isDm: rows, dm-only notice, selection',
        (tester) async {
          tester.view.physicalSize = Size(width, width > 600 ? 800.0 : 844.0);
          tester.view.devicePixelRatio = 1.0;
          addTearDown(tester.view.resetPhysicalSize);
          addTearDown(tester.view.resetDevicePixelRatio);
          final mobile = width < 600;
          ComposerSendAction? selected;
          final anchor = GlobalKey();
          await tester.pumpWidget(
            ProviderScope(
              child: MaterialApp(
                theme: voiceTestTheme(),
                locale: const Locale('ru'),
                localizationsDelegates: AppLocalizations.localizationsDelegates,
                supportedLocales: AppLocalizations.supportedLocales,
                home: Builder(
                  builder: (context) => Scaffold(
                    body: Align(
                      alignment: Alignment.bottomRight,
                      child: Padding(
                        padding: const EdgeInsets.all(16),
                        child: TextButton(
                          key: anchor,
                          onPressed: () async {
                            selected = await showComposerSendMenu(
                              context,
                              anchorContext: anchor.currentContext,
                              isDm: isDm,
                              scheduledMessagesEnabled: true,
                            );
                          },
                          child: const Text('options'),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          );
          await tester.tap(find.text('options'));
          await tester.pumpAndSettle();
          expect(tester.takeException(), isNull);
          final menu = find.byKey(ComposerSendMenu.menuKey);
          expect(menu, findsOneWidget);
          expect(
            find.byType(BottomSheet),
            mobile ? findsOneWidget : findsNothing,
          );
          expect(find.byKey(const Key('composer_send_silent')), findsOneWidget);
          expect(
            find.byKey(const Key('composer_send_schedule')),
            findsOneWidget,
          );
          expect(
            find.byKey(const Key('composer_send_online')),
            isDm ? findsOneWidget : findsNothing,
          );
          expect(
            find.byKey(const Key('composer_send_online_dm_only_notice')),
            mobile && !isDm ? findsOneWidget : findsNothing,
          );
          if (!mobile) expect(tester.getSize(menu).width, 260);
          // Mock rows are 40px; the fallback test text theme uses 24px body
          // lines, so the tile may only grow to 24 + 2x10 padding. It must
          // never fall back to the 56px ListTile default.
          expect(
            tester
                .getSize(find.byKey(const Key('composer_send_silent')))
                .height,
            lessThanOrEqualTo(
              ComposerSendMenu.rowHeight + ComposerSendMenu.rowVerticalPadding,
            ),
          );

          final actionKey = isDm
              ? const Key('composer_send_online')
              : const Key('composer_send_schedule');
          await tester.tap(find.byKey(actionKey));
          await tester.pumpAndSettle();
          expect(
            selected,
            isDm ? ComposerSendAction.online : ComposerSendAction.schedule,
          );
          expect(tester.takeException(), isNull);
          expect(find.byKey(ComposerSendMenu.menuKey), findsNothing);
        },
      );
    }
  }
}
