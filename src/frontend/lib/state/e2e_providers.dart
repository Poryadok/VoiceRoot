import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/chats_client.dart';
import '../backend/e2e_client.dart';
import '../e2e/e2e_bootstrap.dart';
import '../e2e/e2e_crypto_adapter.dart';
import '../e2e/e2e_message_service.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

final e2eCryptoAdapterProvider = Provider<E2eCryptoAdapter>((ref) {
  return E2eCryptoAdapter();
});

final voiceE2eClientProvider = Provider<VoiceE2eClient>((ref) {
  return VoiceE2eClient(
    gateway: ref.watch(gatewayHttpClientProvider),
    adapter: ref.watch(e2eCryptoAdapterProvider),
  );
});

final e2eMessageServiceProvider = Provider<E2eMessageService>((ref) {
  return E2eMessageService(
    adapter: ref.watch(e2eCryptoAdapterProvider),
    e2eClient: ref.watch(voiceE2eClientProvider),
  );
});

final e2eBootstrapServiceProvider = Provider<E2eBootstrapService>((ref) {
  return E2eBootstrapService(e2eClient: ref.watch(voiceE2eClientProvider));
});

/// Best-effort pre-key upload after login/register/session restore.
final e2eBootstrapLifecycleProvider = Provider<void>((ref) {
  ref.listen<AuthState>(authControllerProvider, (previous, next) {
    if (!next.isAuthenticated || next.session == null) return;
    if (previous?.session?.accessToken == next.session!.accessToken &&
        previous?.isAuthenticated == true) {
      return;
    }
    unawaited(
      ref
          .read(e2eBootstrapServiceProvider)
          .ensurePreKeysUploaded(
            authorization: next.session!.authorizationHeader,
          )
          .catchError((_) {}),
    );
  });
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

String? dmPeerProfileIdForChat(WidgetRef ref, String chatId) {
  final known = ref.read(dmPeerProfileByChatIdProvider)[chatId];
  if (known != null && known.isNotEmpty) return known;
  for (final item in ref.read(chatListControllerProvider).items) {
    if (item.chatId == chatId) return item.dmPeerProfileId;
  }
  return null;
}
