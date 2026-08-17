import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';

import 'support/live_gateway_harness.dart';

/// TC-DM-08: archive hides DM from list; mute/unmute APIs succeed.
void main() {
  test(
    'DM archive hide and mute (TC-DM-08)',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final a = await ctx.registerUser('dm08-a');
      final b = await ctx.registerUser('dm08-b');

      final chats = ctx.chatsClient();
      final dm = await chats.createDm(
        authorization: a.authorizationHeader,
        otherProfileId: b.activeProfileId,
      );
      expect(dm, isA<ChatsApiOk<VoiceChat>>());
      final dmId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final messages = ctx.messagesClient();
      await messages.sendMessage(
        authorization: a.authorizationHeader,
        chatId: dmId,
        content: 'before-archive',
        clientMessageId: qaClientMessageId(),
      );

      final before = await chats.listChats(authorization: a.authorizationHeader);
      expect(
        (before as ChatsApiOk<ChatListData>).data.items.any((i) => i.chatId == dmId),
        isTrue,
      );

      expect(
        await chats.archiveChat(
          authorization: a.authorizationHeader,
          chatId: dmId,
          archived: true,
        ),
        isA<ChatsApiOk<void>>(),
      );

      final afterArchive =
          await chats.listChats(authorization: a.authorizationHeader);
      expect(
        (afterArchive as ChatsApiOk<ChatListData>)
            .data
            .items
            .any((i) => i.chatId == dmId),
        isFalse,
        reason: 'archived DM must leave caller list',
      );

      final peerList = await chats.listChats(
        authorization: b.authorizationHeader,
        inbox: 'requests',
      );
      expect(
        (peerList as ChatsApiOk<ChatListData>).data.items.any((i) => i.chatId == dmId),
        isTrue,
        reason: 'stranger DM stays in peer requests inbox after caller archives',
      );

      expect(
        await chats.archiveChat(
          authorization: a.authorizationHeader,
          chatId: dmId,
          archived: false,
        ),
        isA<ChatsApiOk<void>>(),
      );

      expect(
        await chats.muteChat(
          authorization: a.authorizationHeader,
          chatId: dmId,
          mutedUntil: DateTime.now().toUtc().add(const Duration(hours: 2)),
        ),
        isA<ChatsApiOk<void>>(),
      );
      expect(
        await chats.muteChat(
          authorization: a.authorizationHeader,
          chatId: dmId,
        ),
        isA<ChatsApiOk<void>>(),
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
