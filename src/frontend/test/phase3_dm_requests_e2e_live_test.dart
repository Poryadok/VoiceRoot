import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'stranger DM appears in requests inbox until accepted',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('dmreq-a');
      final sessionB = await ctx.registerUser('dmreq-b');

      final chats = ctx.chatsClient();
      final dm = await chats.createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final mainBefore = await chats.listChats(
        authorization: sessionB.authorizationHeader,
        inbox: 'main',
      );
      final mainItems = (mainBefore as ChatsApiOk<ChatListData>).data.items;
      expect(mainItems.any((i) => i.chatId == chatId), isFalse);

      final requests = await chats.listChats(
        authorization: sessionB.authorizationHeader,
        inbox: 'requests',
      );
      final reqItems = (requests as ChatsApiOk<ChatListData>).data.items;
      expect(reqItems.length, 1);
      expect(reqItems.single.chatId, chatId);
      expect(reqItems.single.isStranger, isTrue);

      final accept = await chats.acceptDmRequest(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
      );
      expect(accept, isA<ChatsApiOk<void>>());

      final mainAfter = await chats.listChats(
        authorization: sessionB.authorizationHeader,
        inbox: 'main',
      );
      final afterItems = (mainAfter as ChatsApiOk<ChatListData>).data.items;
      expect(afterItems.length, 1);
      expect(afterItems.single.chatId, chatId);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
