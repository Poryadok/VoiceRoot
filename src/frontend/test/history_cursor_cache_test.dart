import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/message_cache/drift_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_database.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';

VoiceMessage _message(String id) => VoiceMessage(
  id: id,
  chatId: 'chat',
  senderProfileId: 'peer',
  content: id,
  createdAt: DateTime.utc(2024),
);

void main() {
  for (final sqlite in [false, true]) {
    test(
      '${sqlite ? 'SQLite' : 'memory'} scopes history progress and clears it with bodies',
      () async {
        final db = sqlite ? MessageCacheDatabase.forTesting() : null;
        if (db != null) addTearDown(db.close);
        final HistoryCursorMessageCacheStore store = sqlite
            ? DriftMessageCacheStore(db!)
            : InMemoryMessageCacheStore();
        for (final profile in ['one', 'two']) {
          for (final chat in ['a', 'b']) {
            await store.replaceChatHistory(
              profileId: profile,
              chatId: chat,
              messages: [_message('display-last')],
              historyCursor: MessageHistoryCursor(
                messageId: '$profile-$chat-server',
                pageCursor: '$profile-$chat-page',
              ),
            );
          }
        }
        // A new store object over the same database restores metadata independent
        // of whether its message body survives the display/cache trim.
        final reader = sqlite ? DriftMessageCacheStore(db!) : store;
        final cursor = await reader.getHistoryCursor(
          profileId: 'one',
          chatId: 'b',
        );
        expect(cursor?.messageId, 'one-b-server');
        expect(cursor?.pageCursor, 'one-b-page');
        await store.clearProfile('one');
        expect(
          await reader.getHistoryCursor(profileId: 'one', chatId: 'a'),
          isNull,
        );
        expect(
          await reader.getHistoryCursor(profileId: 'one', chatId: 'b'),
          isNull,
        );
        expect(
          (await reader.getHistoryCursor(
            profileId: 'two',
            chatId: 'a',
          ))?.messageId,
          'two-a-server',
        );
        await store.replaceChatMessages(
          profileId: 'two',
          chatId: 'a',
          messages: [],
        );
        expect(
          await reader.getHistoryCursor(profileId: 'two', chatId: 'a'),
          isNull,
        );
        await store.clearAll();
        expect(
          await reader.getHistoryCursor(profileId: 'two', chatId: 'b'),
          isNull,
        );
      },
    );
  }

  test('SQLite body failure rolls back cursor and messages together', () async {
    final db = MessageCacheDatabase.forTesting();
    addTearDown(db.close);
    final store = DriftMessageCacheStore(db);
    await store.replaceChatHistory(
      profileId: 'one',
      chatId: 'a',
      messages: [_message('kept')],
      historyCursor: const MessageHistoryCursor(
        messageId: 'accepted',
        pageCursor: 'retry-page',
      ),
    );
    await expectLater(
      store.replaceChatHistory(
        profileId: 'one',
        chatId: 'a',
        messages: [_message('duplicate'), _message('duplicate')],
        historyCursor: const MessageHistoryCursor(
          messageId: 'must-not-advance',
        ),
      ),
      throwsA(anything),
    );
    expect(
      (await store.getMessages(profileId: 'one', chatId: 'a')).single.id,
      'kept',
    );
    expect(
      (await store.getHistoryCursor(profileId: 'one', chatId: 'a'))?.messageId,
      'accepted',
    );
    expect(
      (await store.getHistoryCursor(profileId: 'one', chatId: 'a'))?.pageCursor,
      'retry-page',
    );
  });

  test(
    'version1 cache upgrades without inventing a cursor from display order',
    () async {
      final payload = InMemoryMessageCacheStore.encodePayload(
        _message('legacy'),
      );
      final db = MessageCacheDatabase(
        NativeDatabase.memory(
          setup: (raw) {
            raw.execute(
              '''CREATE TABLE cached_messages(profile_id TEXT NOT NULL,chat_id TEXT NOT NULL,
message_id TEXT NOT NULL,payload_json TEXT NOT NULL,created_at INTEGER,
PRIMARY KEY(profile_id,chat_id,message_id))''',
            );
            raw.execute('INSERT INTO cached_messages VALUES(?,?,?,?,?)', [
              'one',
              'a',
              'legacy',
              payload,
              1,
            ]);
            raw.execute('PRAGMA user_version=1');
          },
        ),
      );
      addTearDown(db.close);
      final store = DriftMessageCacheStore(db);
      expect(
        (await store.getMessages(profileId: 'one', chatId: 'a')).single.id,
        'legacy',
      );
      expect(
        await store.getHistoryCursor(profileId: 'one', chatId: 'a'),
        isNull,
      );
      await store.replaceChatHistory(
        profileId: 'one',
        chatId: 'a',
        messages: [_message('legacy')],
        historyCursor: const MessageHistoryCursor(messageId: 'from-server'),
      );
      expect(
        (await store.getHistoryCursor(
          profileId: 'one',
          chatId: 'a',
        ))?.messageId,
        'from-server',
      );
      expect(
        (await db.customSelect('PRAGMA user_version').getSingle())
            .data
            .values
            .single,
        2,
      );
    },
  );
}
