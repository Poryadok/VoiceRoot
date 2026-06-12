import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_limits.dart';
import 'package:voice_frontend/backend/messages_client.dart';

void main() {
  group('InMemoryMessageCacheStore', () {
    late InMemoryMessageCacheStore store;

    setUp(() {
      store = InMemoryMessageCacheStore();
    });

    VoiceMessage message(
      String id, {
      String chatId = 'chat-1',
      DateTime? createdAt,
    }) {
      return VoiceMessage(
        id: id,
        chatId: chatId,
        senderProfileId: 'peer-1',
        content: 'body $id',
        createdAt:
            createdAt ??
            DateTime.parse('2024-01-01T00:00:${id.padLeft(2, '0')}Z'),
      );
    }

    test('stores and returns messages sorted by createdAt', () async {
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: [
          message('2', createdAt: DateTime.parse('2024-01-01T00:00:02Z')),
          message('1', createdAt: DateTime.parse('2024-01-01T00:00:01Z')),
        ],
      );

      final loaded = await store.getMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
      );
      expect(loaded.map((m) => m.id), ['1', '2']);
    });

    test('keeps only the newest 50 messages per chat', () async {
      final messages = List.generate(
        kOfflineCacheMessageLimit + 5,
        (index) => message('${index + 1}'),
      );
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: messages,
      );

      final loaded = await store.getMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
      );
      expect(loaded, hasLength(kOfflineCacheMessageLimit));
      expect(loaded.first.id, '6');
      expect(loaded.last.id, '55');
    });

    test('isolates cache rows by profile and chat', () async {
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-a',
        messages: [
          message(
            'a1',
            chatId: 'chat-a',
            createdAt: DateTime.parse('2024-01-01T00:00:01Z'),
          ),
        ],
      );
      await store.replaceChatMessages(
        profileId: 'prof-2',
        chatId: 'chat-a',
        messages: [
          message(
            'b1',
            chatId: 'chat-a',
            createdAt: DateTime.parse('2024-01-01T00:00:01Z'),
          ),
        ],
      );

      final prof1 = await store.getMessages(
        profileId: 'prof-1',
        chatId: 'chat-a',
      );
      final prof2 = await store.getMessages(
        profileId: 'prof-2',
        chatId: 'chat-a',
      );
      expect(prof1.single.id, 'a1');
      expect(prof2.single.id, 'b1');
    });

    test('upsert merges and trims without dropping unrelated chats', () async {
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: [message('1'), message('2')],
      );
      await store.upsertMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: [message('3')],
      );
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-2',
        messages: [
          message(
            'x1',
            chatId: 'chat-2',
            createdAt: DateTime.parse('2024-01-01T00:00:10Z'),
          ),
        ],
      );

      final chat1 = await store.getMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
      );
      final chat2 = await store.getMessages(
        profileId: 'prof-1',
        chatId: 'chat-2',
      );
      expect(chat1.map((m) => m.id), ['1', '2', '3']);
      expect(chat2.single.id, 'x1');
    });

    test('clearProfile and clearAll remove cached rows', () async {
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: [message('1')],
      );
      await store.replaceChatMessages(
        profileId: 'prof-2',
        chatId: 'chat-1',
        messages: [message('2')],
      );

      await store.clearProfile('prof-1');
      expect(
        await store.getMessages(profileId: 'prof-1', chatId: 'chat-1'),
        isEmpty,
      );
      expect(
        (await store.getMessages(profileId: 'prof-2', chatId: 'chat-1')).single.id,
        '2',
      );

      await store.clearAll();
      expect(
        await store.getMessages(profileId: 'prof-2', chatId: 'chat-1'),
        isEmpty,
      );
    });

    test('VoiceMessage json round-trip preserves content', () {
      final original = message('42');
      final decoded = VoiceMessage.fromJson(original.toJson());
      expect(decoded.id, original.id);
      expect(decoded.chatId, original.chatId);
      expect(decoded.content, original.content);
      expect(decoded.createdAt, original.createdAt);
    });
  });
}
