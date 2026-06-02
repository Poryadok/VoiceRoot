import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';

import 'support/live_gateway_harness.dart';

/// Live stack check: Flutter HTTP/WS clients ↔ API Gateway (REST auth, DM, Realtime).
///
/// Run when Phase-1 backends are wired to Gateway (staging or full local stack):
/// ```text
/// flutter test test/gateway_dm_ws_live_integration_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:8080
/// ```
void main() {
  test(
    'REST auth + create DM + WS message_create via Gateway',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('user-a');
      final sessionB = await ctx.registerUser('user-b');

      final chats = VoiceChatsClient(
        httpClient: ctx.httpClient,
        config: ctx.config,
      );
      final dmResult = await chats.createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      expect(dmResult, isA<ChatsApiOk<VoiceChat>>());
      final chat = (dmResult as ChatsApiOk<VoiceChat>).data;
      expect(chat.isDm, isTrue);
      final chatId = chat.id;

      final wsUri = gatewayWebSocketUri(ctx.config.baseUrl);
      final realtime = VoiceRealtimeConnection(
        uri: wsUri,
        headers: {'Authorization': sessionB.authorizationHeader},
      );
      addTearDown(realtime.dispose);

      await realtime.connect();
      await waitForOp(realtime.events, 'hello');

      realtime.sendSubscribe(chatId);
      await waitForOp(
        realtime.events,
        'subscribe_ack',
        where: (f) => f.data?['chat_id'] == chatId,
      );

      const content = 'flutter-qa-live-dm-ws';
      final messages = VoiceMessagesClient(
        httpClient: ctx.httpClient,
        config: ctx.config,
      );
      final frameFuture = waitForOp(realtime.events, 'message_create');
      final sendResult = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: content,
        clientMessageId: qaClientMessageId(),
      );
      expect(sendResult, isA<MessagesApiOk<VoiceMessage>>());
      final sent = (sendResult as MessagesApiOk<VoiceMessage>).data;
      expect(sent.content, content);
      expect(sent.chatId, chatId);

      final frame = await frameFuture;
      expect(frame.data?['chat_id'], chatId);
      expect(frame.data?['message_id'], sent.id);
      expect(frame.data?['sender_profile_id'], sessionA.activeProfileId);
      expect(frame.sequence, isNotNull);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
