import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  test('applyDeepLinkNavigation selects chat and message', () async {
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => throw UnimplementedError()),
      ),
    );
    addTearDown(container.dispose);

    container.read(chatListControllerProvider);
    await pumpEventQueue();

    await container.read(deepLinkNavigatorProvider).apply(
      parseDeepLinkUrl('https://voice.gg/ch/chat-1/m/msg-1'),
    );
    await pumpEventQueue();

    expect(container.read(selectedChatIdProvider), 'chat-1');
    expect(container.read(pendingChatMessageScrollProvider('chat-1')), 'msg-1');
    expect(container.read(pendingChatMessageHighlightProvider('chat-1')), 'msg-1');
    expect(container.read(navigationSectionProvider), NavigationSection.chats);
  });

  test('applyDeepLinkNavigation selects space chat', () async {
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => throw UnimplementedError()),
      ),
    );
    addTearDown(container.dispose);

    container.read(chatListControllerProvider);
    await pumpEventQueue();

    await container.read(deepLinkNavigatorProvider).apply(
      parseDeepLinkUrl('https://voice.gg/s/space-1/c/chat-2'),
    );
    await pumpEventQueue();

    expect(container.read(selectedSpaceIdProvider), 'space-1');
    expect(container.read(selectedChatIdProvider), 'chat-2');
  });
}
