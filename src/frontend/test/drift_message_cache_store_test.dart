import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/message_cache/drift_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_database.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_limits.dart';
import 'package:voice_frontend/backend/messages_client.dart';

void main() {
  group('DriftMessageCacheStore', () {
    late MessageCacheDatabase db;
    late DriftMessageCacheStore store;

    setUp(() {
      db = MessageCacheDatabase(NativeDatabase.memory());
      store = DriftMessageCacheStore(db);
    });

    tearDown(() async {
      await db.close();
    });

    VoiceMessage message(String id) {
      return VoiceMessage(
        id: id,
        chatId: 'chat-1',
        senderProfileId: 'peer-1',
        content: 'body $id',
        createdAt: DateTime.parse('2024-01-01T00:00:${id.padLeft(2, '0')}Z'),
      );
    }

    test('persists and reloads messages from sqlite memory db', () async {
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: [message('1'), message('2')],
      );

      final reloaded = await store.getMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
      );
      expect(reloaded.map((m) => m.id), ['1', '2']);
      expect(reloaded.last.content, 'body 2');
    });

    test('enforces cache limit in sqlite', () async {
      final messages = List.generate(
        kOfflineCacheMessageLimit + 3,
        (index) => message('${index + 1}'),
      );
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: messages,
      );

      final reloaded = await store.getMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
      );
      expect(reloaded, hasLength(kOfflineCacheMessageLimit));
      expect(reloaded.first.id, '4');
    });

    test('clearAll removes rows', () async {
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: [message('1')],
      );
      await store.clearAll();
      expect(
        await store.getMessages(profileId: 'prof-1', chatId: 'chat-1'),
        isEmpty,
      );
    });

    test('upsert merges existing rows', () async {
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: [message('1')],
      );
      await store.upsertMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: [message('2')],
      );

      final reloaded = await store.getMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
      );
      expect(reloaded.map((m) => m.id), ['1', '2']);
    });

    test('clearProfile removes only that profile rows', () async {
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
    });
  });
}
