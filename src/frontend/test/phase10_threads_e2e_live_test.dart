import 'package:flutter_test/flutter_test.dart';

import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase 10 threads E2E (API-level): DM reply excluded from main feed, visible in thread.
///
/// ```text
/// flutter test test/phase10_threads_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'DM reply: main feed excludes thread, thread endpoint returns reply',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final userA = await ctx.registerUser('threads-a');
      final userB = await ctx.registerUser('threads-b');
      await ctx.inviteAndAcceptFriends(userA, userB);

      final chats = ctx.chatsClient();
      final dm = await chats.createDm(
        authorization: userA.authorizationHeader,
        otherProfileId: userB.activeProfileId,
      );
      expect(dm, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final messages = ctx.messagesClient();
      final root = await messages.sendMessage(
        authorization: userA.authorizationHeader,
        chatId: chatId,
        content: 'root-for-thread',
      );
      expect(root, isA<MessagesApiOk<VoiceMessage>>());
      final parentId = (root as MessagesApiOk<VoiceMessage>).data.id;

      final reply = await messages.sendMessage(
        authorization: userB.authorizationHeader,
        chatId: chatId,
        content: 'live-thread-reply',
        threadParentId: parentId,
      );
      expect(reply, isA<MessagesApiOk<VoiceMessage>>());

      final main = await messages.getMessages(
        authorization: userA.authorizationHeader,
        chatId: chatId,
      );
      expect(main, isA<MessagesApiOk<MessageListData>>());
      final mainMsgs = (main as MessagesApiOk<MessageListData>).data.messages;
      expect(
        mainMsgs.where((m) => m.content == 'live-thread-reply'),
        isEmpty,
        reason: 'reply excluded from main feed',
      );

      final thread = await messages.getThreadMessages(
        authorization: userA.authorizationHeader,
        chatId: chatId,
        threadParentId: parentId,
      );
      expect(thread, isA<MessagesApiOk<MessageListData>>());
      final replies = (thread as MessagesApiOk<MessageListData>).data.messages;
      expect(
        replies.any((m) => m.content == 'live-thread-reply'),
        isTrue,
        reason: 'thread endpoint returns reply',
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
