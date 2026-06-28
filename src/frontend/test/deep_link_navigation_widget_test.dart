import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  testWidgets('applyDeepLinkNavigation selects chat and message', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        child: const _DeepLinkHarness(
          url: 'https://voice.gg/ch/chat-1/m/msg-1',
        ),
      ),
    );
    await tester.pump();

    final container = ProviderScope.containerOf(
      tester.element(find.byType(_DeepLinkHarness)),
    );

    expect(container.read(selectedChatIdProvider), 'chat-1');
    expect(container.read(pendingChatMessageScrollProvider('chat-1')), 'msg-1');
    expect(container.read(pendingChatMessageHighlightProvider('chat-1')), 'msg-1');
    expect(container.read(navigationSectionProvider), NavigationSection.chats);
  });

  testWidgets('applyDeepLinkNavigation selects space chat', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        child: const _DeepLinkHarness(
          url: 'https://voice.gg/s/space-1/c/chat-2',
        ),
      ),
    );
    await tester.pump();

    final container = ProviderScope.containerOf(
      tester.element(find.byType(_DeepLinkHarness)),
    );

    expect(container.read(selectedSpaceIdProvider), 'space-1');
    expect(container.read(selectedChatIdProvider), 'chat-2');
  });
}

class _DeepLinkHarness extends ConsumerWidget {
  const _DeepLinkHarness({required this.url});

  final String url;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final target = parseDeepLinkUrl(url);
      unawaited(ref.read(deepLinkNavigatorProvider).apply(target));
    });
    return const SizedBox();
  }
}
