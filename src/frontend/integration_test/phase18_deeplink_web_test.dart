import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';

import '../test/support/auth_test_overrides.dart';

/// Phase 18: Chrome/web integration — deep link navigates to conversation.
void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('deep link navigation opens conversation region', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

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
        child: Consumer(
          builder: (context, ref, _) {
            WidgetsBinding.instance.addPostFrameCallback((_) async {
              final target = parseDeepLinkUrl(
                'https://voice.gg/ch/integration-chat/m/integration-msg',
              );
              await ref.read(deepLinkNavigatorProvider).apply(target);
            });
            return const VoiceApp(locale: Locale('en'));
          },
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(container.read(selectedChatIdProvider), 'integration-chat');
    expect(
      container.read(pendingChatMessageScrollProvider('integration-chat')),
      'integration-msg',
    );
    expect(find.bySemanticsLabel('Conversation'), findsOneWidget);
    expect(find.byKey(ThreeColumnShell.navOpenChat), findsOneWidget);
  });
}
