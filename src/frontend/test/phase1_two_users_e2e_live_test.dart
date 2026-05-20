import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-1 product E2E (API-level): two accounts, DM, Realtime WS, JWT refresh, mark read.
///
/// Run when Gateway and Phase-1 upstreams are available (local stack or staging):
/// ```text
/// flutter test test/phase1_two_users_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:8080
/// ```
void main() {
  test(
    'two users: DM, WS delivery, auth refresh, mark read',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('e2e-a');
      var sessionB = await ctx.registerUser('e2e-b');

      final chats = VoiceChatsClient(httpClient: ctx.httpClient, config: ctx.config);
      final dmResult = await chats.createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      expect(dmResult, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (dmResult as ChatsApiOk<VoiceChat>).data.id;

      final messages =
          VoiceMessagesClient(httpClient: ctx.httpClient, config: ctx.config);
      final wsUri = gatewayWebSocketUri(ctx.config.baseUrl);

      Future<VoiceRealtimeConnection> connectB(AuthSession session) async {
        final conn = VoiceRealtimeConnection(
          uri: wsUri,
          headers: {'Authorization': session.authorizationHeader},
        );
        await conn.connect();
        await waitForOp(conn.events, 'hello');
        conn.sendSubscribe(chatId);
        await waitForOp(
          conn.events,
          'subscribe_ack',
          where: (f) => f.data?['chat_id'] == chatId,
        );
        return conn;
      }

      var realtimeB = await connectB(sessionB);
      addTearDown(realtimeB.dispose);

      const firstContent = 'phase1-e2e-first';
      final frame1Future = waitForOp(realtimeB.events, 'message_create');
      final send1 = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: firstContent,
        clientMessageId: qaClientMessageId(),
      );
      expect(send1, isA<MessagesApiOk<VoiceMessage>>());
      final msg1 = (send1 as MessagesApiOk<VoiceMessage>).data;

      final frame1 = await frame1Future;
      expect(frame1.data?['chat_id'], chatId);
      expect(frame1.data?['message_id'], msg1.id);

      sessionB = await ctx.refreshSession(sessionB);
      await realtimeB.dispose();
      realtimeB = await connectB(sessionB);

      final historyAfterRefresh = await messages.getMessages(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
      );
      expect(historyAfterRefresh, isA<MessagesApiOk<MessageListData>>());
      final listed = (historyAfterRefresh as MessagesApiOk<MessageListData>).data;
      expect(
        listed.messages.any((m) => m.id == msg1.id && m.content == firstContent),
        isTrue,
      );

      const secondContent = 'phase1-e2e-after-refresh';
      final frame2Future = waitForOp(realtimeB.events, 'message_create');
      final send2 = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: secondContent,
        clientMessageId: qaClientMessageId(),
      );
      expect(send2, isA<MessagesApiOk<VoiceMessage>>());
      final msg2 = (send2 as MessagesApiOk<VoiceMessage>).data;

      final frame2 = await frame2Future;
      expect(frame2.data?['message_id'], msg2.id);

      final markResult = await messages.markRead(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
        lastReadMessageId: msg2.id,
      );
      expect(markResult, isA<MessagesApiOk<void>>());

      final readState = await messages.getReadState(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
      );
      expect(readState, isA<MessagesApiOk<ReadStateData>>());
      final state = (readState as MessagesApiOk<ReadStateData>).data;
      expect(state.chatId, chatId);
      expect(state.profileId, sessionB.activeProfileId);
      expect(state.lastReadMessageId, msg2.id);

      final realtimeBDevice2 = await connectB(sessionB);
      addTearDown(realtimeBDevice2.dispose);
      realtimeBDevice2.sendMarkRead(chatId: chatId, messageId: msg2.id);
      final markWs = await waitForOp(
        realtimeB.events,
        'mark_read',
        where: (f) =>
            f.data?['chat_id'] == chatId && f.data?['message_id'] == msg2.id,
      );
      expect(markWs.data?['message_id'], msg2.id);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
