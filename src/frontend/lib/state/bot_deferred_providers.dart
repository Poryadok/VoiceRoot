import 'package:flutter_riverpod/flutter_riverpod.dart';

class DeferredBotInteraction {
  const DeferredBotInteraction({
    required this.botName,
    required this.interactionToken,
  });

  final String botName;
  final String interactionToken;
}

class DeferredInteractionNotifier extends StateNotifier<DeferredBotInteraction?> {
  DeferredInteractionNotifier() : super(null);

  void setDeferred({required String botName, required String interactionToken}) {
    state = DeferredBotInteraction(
      botName: botName,
      interactionToken: interactionToken,
    );
  }

  void clear() => state = null;
}

final deferredBotInteractionProvider = StateNotifierProvider.autoDispose
    .family<DeferredInteractionNotifier, DeferredBotInteraction?, String>(
      (ref, chatId) => DeferredInteractionNotifier(),
    );
