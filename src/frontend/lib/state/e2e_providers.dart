import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/e2e_client.dart';
import '../backend/chats_client.dart';
import '../e2e/e2e_crypto_adapter.dart';
import '../e2e/e2e_message_service.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

final e2eCryptoAdapterProvider = Provider<E2eCryptoAdapter>((ref) {
  return E2eCryptoAdapter();
});

final e2eMessageServiceProvider = Provider<E2eMessageService>((ref) {
  return E2eMessageService(adapter: ref.watch(e2eCryptoAdapterProvider));
});

final voiceE2eClientProvider = Provider<VoiceE2eClient>((ref) {
  return VoiceE2eClient(
    gateway: ref.watch(gatewayHttpClientProvider),
    adapter: ref.watch(e2eCryptoAdapterProvider),
  );
});

/// Whether the open DM chat has E2E enabled (from chat list metadata).
final chatE2eEnabledProvider = Provider.family<bool, String>((ref, chatId) {
  final list = ref.watch(chatListControllerProvider).items;
  for (final item in list) {
    if (item.chatId == chatId) {
      return item.chat.e2eEnabled;
    }
  }
  return false;
});

VoiceChat? chatMetadataForId(Iterable<ChatListItem> items, String chatId) {
  for (final item in items) {
    if (item.chatId == chatId) return item.chat;
  }
  return null;
}
