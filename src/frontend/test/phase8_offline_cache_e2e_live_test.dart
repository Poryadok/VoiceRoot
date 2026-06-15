import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/message_cache/drift_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_database.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

/// Opt-in live check: REST messages persist in drift cache and reload offline.
///
/// flutter test test/phase8_offline_cache_e2e_live_test.dart ^
void main() {
  test('offline cache stores last REST page in drift and reloads it', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final sessionA = await ctx.registerUser('offline-cache-a');
    final sessionB = await ctx.registerUser('offline-cache-b');

    final chats = ctx.chatsClient();
    final dm = await chats.createDm(
      authorization: sessionA.authorizationHeader,
      otherProfileId: sessionB.activeProfileId,
    );
    expect(dm, isA<ChatsApiOk<VoiceChat>>());
    final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

    final messages = ctx.messagesClient();
    const body = 'offline-cache-live';
    final sent = await messages.sendMessage(
      authorization: sessionA.authorizationHeader,
      chatId: chatId,
      content: body,
      clientMessageId: qaClientMessageId(),
    );
    expect(sent, isA<MessagesApiOk<VoiceMessage>>());
    final messageId = (sent as MessagesApiOk<VoiceMessage>).data.id;

    final history = await messages.getMessages(
      authorization: sessionB.authorizationHeader,
      chatId: chatId,
    );
    expect(history, isA<MessagesApiOk<MessageListData>>());
    final page = (history as MessagesApiOk<MessageListData>).data.messages;
    expect(page.any((m) => m.id == messageId && m.content == body), isTrue);

    final db = await MessageCacheDatabase.openEncrypted();
    addTearDown(db.close);
    final store = DriftMessageCacheStore(db);
    await store.replaceChatMessages(
      profileId: sessionB.activeProfileId,
      chatId: chatId,
      messages: page,
    );

    final cached = await store.getMessages(
      profileId: sessionB.activeProfileId,
      chatId: chatId,
    );
    expect(
      cached.any((m) => m.id == messageId && m.content == body),
      isTrue,
      reason: 'drift cache should retain REST history for offline read',
    );
  }, skip: runLiveIntegration ? false : 'Set RUN_LIVE_INTEGRATION=1');
}
