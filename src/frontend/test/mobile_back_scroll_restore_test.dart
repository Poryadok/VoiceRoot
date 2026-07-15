import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  test('backToChatList clears chat and requests scroll restore', () {
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
    );
    addTearDown(container.dispose);

    container.read(chatListScrollOffsetProvider.notifier).state = 128;
    container.read(selectedChatIdProvider.notifier).state = 'chat-1';
    container.read(shellSidePanelProvider.notifier).state =
        ShellSidePanel.members;

    container.read(shellNavigationProvider).backToChatList();

    expect(container.read(selectedChatIdProvider), isNull);
    expect(container.read(shellSidePanelProvider), ShellSidePanel.none);
    expect(container.read(chatListScrollRestoreProvider), isTrue);
    expect(container.read(chatListScrollOffsetProvider), 128);
  });
}
