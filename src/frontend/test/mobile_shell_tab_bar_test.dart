import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/shell/mobile_shell_tab_bar.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('mobile tabs reveal their destinations from an open chat', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final container = ProviderContainer();
    addTearDown(container.dispose);
    container.read(selectedChatIdProvider.notifier).state = 'chat-abc';

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Consumer(
              builder: (context, ref, _) {
                final chatId = ref.watch(selectedChatIdProvider);
                final section = ref.watch(navigationSectionProvider);
                return ThreeColumnShell(
                  navigationChild: Center(
                    child: Text(
                      section == NavigationSection.social
                          ? 'social panel'
                          : 'chat list',
                    ),
                  ),
                  mainChild: chatId == null ? null : const Text('chat room'),
                  showMainOnlyOnNarrow: chatId != null,
                );
              },
            ),
            bottomNavigationBar: const MobileShellTabBar(),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MobileShellTabBar.barKey), findsOneWidget);
    expect(find.text('chat room'), findsOneWidget);
    expect(find.text('chat list'), findsNothing);

    await tester.tap(find.text('Friends'));
    await tester.pumpAndSettle();

    expect(container.read(selectedChatIdProvider), isNull);
    expect(
      container.read(navigationSectionProvider),
      NavigationSection.social,
    );
    expect(find.text('social panel'), findsOneWidget);
    expect(find.text('chat room'), findsNothing);

    await tester.tap(find.byIcon(Icons.chat_bubble_outline));
    await tester.pumpAndSettle();

    expect(container.read(navigationSectionProvider), NavigationSection.chats);
    expect(find.text('chat list'), findsOneWidget);
    expect(find.text('social panel'), findsNothing);
  });
}
