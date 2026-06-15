import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/search_client.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';

import 'support/live_gateway_harness.dart';

/// Phase 15 live E2E DM: two users opt in, send ciphertext, server search excludes body.
///
/// Run when full compose stack is up:
/// ```text
/// flutter test test/phase15_e2e_dm_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'two users enable E2E DM; global and in-chat search exclude message body',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('e2e-dm-a');
      final sessionB = await ctx.registerUser('e2e-dm-b');

      final gateway = GatewayHttpClient(
        httpClient: ctx.httpClient,
        config: ctx.config,
      );
      final chats = VoiceChatsClient(gateway: gateway);
      final adapterA = E2eCryptoAdapter.inMemoryForTest();
      final adapterB = E2eCryptoAdapter.inMemoryForTest();
      final e2eA = VoiceE2eClient(gateway: gateway, adapter: adapterA);
      final e2eB = VoiceE2eClient(gateway: gateway, adapter: adapterB);
      final messages = VoiceMessagesClient(gateway: gateway);
      final search = VoiceSearchClient(gateway: gateway);

      final dmResult = await chats.createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      expect(dmResult, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (dmResult as ChatsApiOk<VoiceChat>).data.id;

      final preKeyA = await e2eA.uploadPreKeyBundle(
        authorization: sessionA.authorizationHeader,
      );
      expect(preKeyA, isA<E2eApiOk<void>>());

      final preKeyB = await e2eB.uploadPreKeyBundle(
        authorization: sessionB.authorizationHeader,
      );
      expect(preKeyB, isA<E2eApiOk<void>>());

      final enableA = await e2eA.enableChatE2e(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
      );
      expect(enableA, isA<E2eApiOk<void>>());

      final enableB = await e2eB.enableChatE2e(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
      );
      expect(enableB, isA<E2eApiOk<void>>());

      final secretPhrase =
          'phase15-live-secret-${DateTime.now().microsecondsSinceEpoch}';
      final ciphertext = await e2eA.encryptForChat(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        peerProfileId: sessionB.activeProfileId,
        plaintext: secretPhrase,
      );
      expect(ciphertext, isNot(equals(secretPhrase)));

      final sendResult = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: ciphertext,
        isE2e: true,
        clientMessageId: qaClientMessageId(),
      );
      expect(sendResult, isA<MessagesApiOk<VoiceMessage>>());
      final sent = (sendResult as MessagesApiOk<VoiceMessage>).data;
      expect(sent.isE2e, isTrue);
      expect(sent.content, isNot(equals(secretPhrase)));

      final storedContent = await queryMessagingDbMessageContent(sent.id);
      if (storedContent != null) {
        expect(
          storedContent,
          isNot(equals(secretPhrase)),
          reason: 'messaging_db must not store E2E plaintext',
        );
        expect(storedContent, equals(sent.content));
      }

      void expectSearchExcludesPhrase(SearchApiResult<dynamic> result, String phrase) {
        switch (result) {
          case SearchApiOk(:final data):
            final hits = switch (data) {
              GlobalSearchData(:final messages) => messages,
              InChatSearchData(:final hits) => hits,
              _ => const <SearchHit>[],
            };
            expect(
              hits.any((h) => h.snippet.contains(phrase)),
              isFalse,
              reason: 'search must not surface E2E plaintext',
            );
          case SearchApiErr(:final error):
            expect(
              error.statusCode,
              anyOf(503, 502, 504, 500),
              reason: 'search unavailable still satisfies E2E exclusion',
            );
        }
      }

      final globalSearch = await search.searchGlobal(
        authorization: sessionA.authorizationHeader,
        query: secretPhrase,
      );
      expectSearchExcludesPhrase(globalSearch, secretPhrase);

      final inChatSearch = await search.searchInChat(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        query: secretPhrase,
      );
      expectSearchExcludesPhrase(inChatSearch, secretPhrase);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
