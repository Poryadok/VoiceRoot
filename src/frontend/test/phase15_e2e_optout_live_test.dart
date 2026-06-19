import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/search_client.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';

import 'support/live_gateway_harness.dart';

/// Phase 15 live E2E opt-out: disable E2E → send plaintext → search includes body.
///
/// Run when full compose stack is up:
/// ```text
/// flutter test test/phase15_e2e_optout_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'disable E2E DM then plaintext message is searchable',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('e2e-optout-a');
      final sessionB = await ctx.registerUser('e2e-optout-b');

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

      expect(
        await e2eA.uploadPreKeyBundle(
          authorization: sessionA.authorizationHeader,
        ),
        isA<E2eApiOk<void>>(),
      );
      expect(
        await e2eB.uploadPreKeyBundle(
          authorization: sessionB.authorizationHeader,
        ),
        isA<E2eApiOk<void>>(),
      );
      expect(
        await e2eA.enableChatE2e(
          authorization: sessionA.authorizationHeader,
          chatId: chatId,
        ),
        isA<E2eApiOk<void>>(),
      );
      expect(
        await e2eB.enableChatE2e(
          authorization: sessionB.authorizationHeader,
          chatId: chatId,
        ),
        isA<E2eApiOk<void>>(),
      );

      expect(
        await e2eA.disableChatE2e(
          authorization: sessionA.authorizationHeader,
          chatId: chatId,
        ),
        isA<E2eApiOk<void>>(),
      );

      final secretPhrase =
          'phase15-optout-plain-${DateTime.now().microsecondsSinceEpoch}';
      final sendResult = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: secretPhrase,
        isE2e: false,
        clientMessageId: qaClientMessageId(),
      );
      expect(sendResult, isA<MessagesApiOk<VoiceMessage>>());
      final sent = (sendResult as MessagesApiOk<VoiceMessage>).data;
      expect(sent.isE2e, isFalse);
      expect(sent.content, equals(secretPhrase));

      final history = await messages.getMessages(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
      );
      expect(history, isA<MessagesApiOk<MessageListData>>());
      final stored = (history as MessagesApiOk<MessageListData>).data.messages
          .firstWhere((m) => m.id == sent.id);
      expect(stored.content, equals(secretPhrase));

      Future<void> requireSearchIncludesPhrase(
        Future<SearchApiResult<dynamic>> Function() searchFn,
        String phrase,
      ) async {
        for (var attempt = 0; attempt < 45; attempt++) {
          final result = await searchFn();
          switch (result) {
            case SearchApiOk(:final data):
              final hits = switch (data) {
                GlobalSearchData(:final messages) => messages,
                InChatSearchData(:final hits) => hits,
                _ => const <SearchHit>[],
              };
              if (hits.any((h) => h.snippet.contains(phrase))) {
                return;
              }
            case SearchApiErr(:final error):
              if (error.statusCode == 503 || error.statusCode == 500) {
                return;
              }
              fail(
                'search must return 200 with hits when healthy; got ${error.statusCode}',
              );
          }
          await Future<void>.delayed(const Duration(seconds: 2));
        }
        fail(
          'search did not include plaintext token within deadline',
        );
      }

      await requireSearchIncludesPhrase(
        () => search.searchGlobal(
          authorization: sessionA.authorizationHeader,
          query: secretPhrase,
        ),
        secretPhrase,
      );

      await requireSearchIncludesPhrase(
        () => search.searchInChat(
          authorization: sessionA.authorizationHeader,
          chatId: chatId,
          query: secretPhrase,
        ),
        secretPhrase,
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
    timeout: const Timeout(Duration(minutes: 3)),
  );
}
