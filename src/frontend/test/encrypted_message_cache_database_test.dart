import 'dart:io';

import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:path/path.dart' as p;
import 'package:sqlite3/sqlite3.dart' as sqlite3;
import 'package:voice_frontend/backend/message_cache/drift_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_database.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_database_open.dart';
import 'package:voice_frontend/backend/messages_client.dart';

void main() {
  group('encrypted message cache database', () {
    test('roundtrips messages through encrypted sqlite file', () async {
      final tempDir = await Directory.systemTemp.createTemp('voice_enc_cache_');
      addTearDown(() async {
        if (await tempDir.exists()) {
          await tempDir.delete(recursive: true);
        }
      });

      const encryptionKey = 'test-cache-key';
      final dbPath = p.join(tempDir.path, 'cache.sqlite');
      final db = MessageCacheDatabase(
        encryptedExecutorAtPath(dbPath: dbPath, encryptionKey: encryptionKey),
      );
      addTearDown(db.close);

      final store = DriftMessageCacheStore(db);
      final message = VoiceMessage(
        id: 'msg-1',
        chatId: 'chat-1',
        senderProfileId: 'peer-1',
        content: 'encrypted body',
        createdAt: DateTime.parse('2024-01-01T00:00:01Z'),
      );
      await store.replaceChatMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
        messages: [message],
      );
      await db.close();

      final raw = File(dbPath).readAsBytesSync();
      expect(
        String.fromCharCodes(raw),
        isNot(contains('encrypted body')),
        reason: 'message body must not appear as plaintext in the db file',
      );

      final reopened = MessageCacheDatabase(
        encryptedExecutorAtPath(dbPath: dbPath, encryptionKey: encryptionKey),
      );
      addTearDown(reopened.close);
      final reloaded = await DriftMessageCacheStore(reopened).getMessages(
        profileId: 'prof-1',
        chatId: 'chat-1',
      );
      expect(reloaded.single.content, 'encrypted body');
    });

    test('migrateUnencryptedMessageCacheIfNeeded rekeys legacy plaintext db',
        () async {
      final tempDir = await Directory.systemTemp.createTemp('voice_plain_cache_');
      addTearDown(() async {
        if (await tempDir.exists()) {
          await tempDir.delete(recursive: true);
        }
      });

      final dbPath = p.join(tempDir.path, 'legacy.sqlite');
      final plain = sqlite3.sqlite3.open(dbPath);
      try {
        plain.execute('''
CREATE TABLE cached_messages (
  profile_id TEXT NOT NULL,
  chat_id TEXT NOT NULL,
  message_id TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  created_at INTEGER,
  PRIMARY KEY (profile_id, chat_id, message_id)
);
''');
        plain.execute(
          "INSERT INTO cached_messages VALUES ('p', 'c', 'm', '{\"content\":\"legacy\"}', NULL);",
        );
      } finally {
        plain.close();
      }

      const encryptionKey = 'legacy-migration-key';
      await migrateUnencryptedMessageCacheIfNeeded(
        dbPath: dbPath,
        encryptionKey: encryptionKey,
      );

      final raw = File(dbPath).readAsBytesSync();
      expect(String.fromCharCodes(raw), isNot(contains('legacy')));

      final db = MessageCacheDatabase(
        encryptedExecutorAtPath(dbPath: dbPath, encryptionKey: encryptionKey),
      );
      addTearDown(db.close);
      final rows = await db.select(db.cachedMessages).get();
      expect(rows, hasLength(1));
      expect(rows.single.payloadJson, contains('legacy'));
    });
  });
}

NativeDatabase encryptedExecutorAtPath({
  required String dbPath,
  required String encryptionKey,
}) {
  return NativeDatabase(
    File(dbPath),
    setup: (db) => applyDatabaseCipherKey(db, encryptionKey),
  );
}
