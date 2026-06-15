import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/state/bot_providers.dart';

void main() {
  test('deferred outcome sets deferredBotInteractionProvider', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    const chatId = 'chat-1';
    final notifier = container.read(deferredBotInteractionProvider(chatId).notifier);

    // Simulate _handleOutcome deferred branch via direct notifier (executor needs HTTP).
    notifier.setDeferred(botName: 'PingBot', interactionToken: 'tok-1');
    expect(container.read(deferredBotInteractionProvider(chatId))?.botName, 'PingBot');

    notifier.clear();
    expect(container.read(deferredBotInteractionProvider(chatId)), isNull);
  });

  test('SlashInteractionOutcome parses deferred flag', () {
    const outcome = SlashInteractionOutcome(
      interactionToken: 'tok',
      deferred: true,
    );
    expect(outcome.deferred, isTrue);
  });
}
