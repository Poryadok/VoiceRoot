import 'dart:io';

import 'package:drift/drift.dart';
import 'package:drift_flutter/drift_flutter.dart';
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';
import 'package:sqlite3/sqlite3.dart' as sqlite3;

import 'message_cache_database_key.dart';

const messageCacheDatabaseName = 'voice_message_cache';

/// Escapes a value for safe inclusion in a SQL single-quoted string literal.
String escapeSqlString(String source) => source.replaceAll("'", "''");

bool databaseHasCipher(sqlite3.Database db) {
  return db.select('pragma cipher').isNotEmpty;
}

void applyDatabaseCipherKey(sqlite3.Database db, String encryptionKey) {
  if (!databaseHasCipher(db)) {
    throw UnsupportedError(
      'Encrypted message cache requires SQLite3MultipleCiphers (sqlite3mc build hook).',
    );
  }
  db.execute("pragma key = '${escapeSqlString(encryptionKey)}'");
  db.execute('select count(*) from sqlite_master');
}

Future<String> messageCacheDatabasePath() async {
  final dir = await getApplicationDocumentsDirectory();
  return p.join(dir.path, '$messageCacheDatabaseName.sqlite');
}

/// Converts a legacy plaintext cache file created before encryption shipped.
Future<void> migrateUnencryptedMessageCacheIfNeeded({
  required String dbPath,
  required String encryptionKey,
}) async {
  final dbFile = File(dbPath);
  if (!await dbFile.exists()) {
    return;
  }

  final probe = sqlite3.sqlite3.open(dbPath);
  try {
    probe.execute('SELECT count(*) FROM sqlite_master');
  } catch (_) {
    return;
  } finally {
    probe.close();
  }

  final tmpPath = '$dbPath.migrating';
  final tmpFile = File(tmpPath);
  if (await tmpFile.exists()) {
    await tmpFile.delete();
  }

  final plain = sqlite3.sqlite3.open(dbPath);
  try {
    plain.execute("VACUUM INTO '${escapeSqlString(tmpPath)}'");
  } finally {
    plain.close();
  }

  final encrypted = sqlite3.sqlite3.open(tmpPath);
  try {
    if (!databaseHasCipher(encrypted)) {
      throw UnsupportedError(
        'Encrypted message cache requires SQLite3MultipleCiphers (sqlite3mc build hook).',
      );
    }
    encrypted.execute("PRAGMA rekey = '${escapeSqlString(encryptionKey)}'");
    encrypted.execute('select count(*) from sqlite_master');
  } finally {
    encrypted.close();
  }

  await dbFile.delete();
  await tmpFile.rename(dbPath);
}

/// Opens the encrypted offline message cache executor (native platforms only).
Future<QueryExecutor> openEncryptedMessageCacheExecutor() async {
  final encryptionKey = await MessageCacheDatabaseKey.loadOrCreate();
  final dbPath = await messageCacheDatabasePath();
  await migrateUnencryptedMessageCacheIfNeeded(
    dbPath: dbPath,
    encryptionKey: encryptionKey,
  );

  return driftDatabase(
    name: messageCacheDatabaseName,
    native: DriftNativeOptions(
      setup: (db) => applyDatabaseCipherKey(
        db as sqlite3.Database,
        encryptionKey,
      ),
    ),
  );
}
