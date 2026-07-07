import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/search_client.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';

import 'support/live_gateway_harness.dart';

/// app stack5 live E2E edit: ciphertext update and search exclusion for edited body.
void main() {
  test(
    'E2E DM edit updates ciphertext; search excludes edited plaintext',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('e2e-edit-a');
      final sessionB = await ctx.registerUser('e2e-edit-b');

      final gateway = GatewayHttpClient(
        httpClient: ctx.httpClient,
        config: ctx.config,
      );
      final chats = VoiceChatsClient(gateway: gateway);
      final adapterA = E2eCryptoAdapter.inMemoryForTest();
      final e2eA = VoiceE2eClient(gateway: gateway, adapter: adapterA);
      final messages = VoiceMessagesClient(gateway: gateway);
      final search = VoiceSearchClient(gateway: gateway);

      final dmResult = await chats.createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      expect(dmResult, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (dmResult as ChatsApiOk<VoiceChat>).data.id;

      expect(await e2eA.uploadPreKeyBundle(authorization: sessionA.authorizationHeader), isA<E2eApiOk<void>>());
      final e2eB = VoiceE2eClient(
        gateway: gateway,
        adapter: E2eCryptoAdapter.inMemoryForTest(),
      );
      expect(await e2eB.uploadPreKeyBundle(authorization: sessionB.authorizationHeader), isA<E2eApiOk<void>>());
      expect(await e2eA.enableChatE2e(authorization: sessionA.authorizationHeader, chatId: chatId), isA<E2eApiOk<void>>());
      expect(await e2eB.enableChatE2e(authorization: sessionB.authorizationHeader, chatId: chatId), isA<E2eApiOk<void>>());

      final secretV1 = 'phase15-edit-v1-${DateTime.now().microsecondsSinceEpoch}';
      final ciphertextV1 = await e2eA.encryptForChat(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        peerProfileId: sessionB.activeProfileId,
        plaintext: secretV1,
      );
      final sendResult = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: ciphertextV1,
        isE2e: true,
        clientMessageId: qaClientMessageId(),
      );
      expect(sendResult, isA<MessagesApiOk<VoiceMessage>>());
      final sent = (sendResult as MessagesApiOk<VoiceMessage>).data;

      final secretV2 = 'phase15-edit-v2-${DateTime.now().microsecondsSinceEpoch}';
      final ciphertextV2 = await e2eA.encryptForChat(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        peerProfileId: sessionB.activeProfileId,
        plaintext: secretV2,
      );
      final editResult = await messages.editMessage(
        authorization: sessionA.authorizationHeader,
        messageId: sent.id,
        content: ciphertextV2,
      );
      expect(editResult, isA<MessagesApiOk<VoiceMessage>>());
      expect((editResult as MessagesApiOk<VoiceMessage>).data.content, isNot(equals(secretV2)));

      void expectSearchOkExcludes(SearchApiResult<dynamic> result, String phrase) {
        expect(result, isA<SearchApiOk<dynamic>>());
        final data = (result as SearchApiOk<dynamic>).data;
        final hits = switch (data) {
          GlobalSearchData(:final messages) => messages,
          InChatSearchData(:final hits) => hits,
          _ => const <SearchHit>[],
        };
        expect(hits.any((h) => h.snippet.contains(phrase)), isFalse);
      }

      expectSearchOkExcludes(
        await search.searchGlobal(
          authorization: sessionA.authorizationHeader,
          query: secretV2,
        ),
        secretV2,
      );
      expectSearchOkExcludes(
        await search.searchInChat(
          authorization: sessionA.authorizationHeader,
          chatId: chatId,
          query: secretV2,
        ),
        secretV2,
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
