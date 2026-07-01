import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

/// Phase 18: Chrome/web widget test — deep link navigates to conversation.
///
/// CI job `flutter-web-integration` runs this with `-d chrome` (integration_test
/// package cannot target web via `flutter test`; see integration_test/README.md).
void main() {
  testWidgets('deep link navigation opens conversation region', (tester) async {
    await bindDesktopTestViewport(tester);

    late ProviderContainer container;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container = ProviderContainer(
          overrides: [
            ...guestShellTestOverrides(
              client: MockClient((_) async => throw UnimplementedError()),
            ),
          ],
        ),
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await _pumpShellReady(tester);

    await container.read(deepLinkNavigatorProvider).apply(
      parseDeepLinkUrl(
        'https://voice.gg/ch/integration-chat/m/integration-msg',
      ),
    );
    await _pumpShellReady(tester);

    expect(container.read(selectedChatIdProvider), 'integration-chat');
    expect(
      container.read(pendingChatMessageScrollProvider('integration-chat')),
      'integration-msg',
    );

    expect(find.byKey(ThreeColumnShell.navOpenChat), findsOneWidget);
    expect(find.bySemanticsLabel('Conversation'), findsOneWidget);
    expect(
      find.ancestor(
        of: find.byKey(ThreeColumnShell.navOpenChat),
        matching: find.bySemanticsLabel('Conversation'),
      ),
      findsOneWidget,
    );
  });
}

Future<void> _pumpShellReady(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump(const Duration(milliseconds: 400));
}
