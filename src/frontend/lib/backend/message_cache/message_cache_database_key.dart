import 'dart:convert';
import 'dart:math';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Device-local encryption key for the offline message cache SQLite file.
class MessageCacheDatabaseKey {
  MessageCacheDatabaseKey._();

  static const _storageKey = 'voice_message_cache_db_key';

  static const FlutterSecureStorage _storage = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
  );

  static Future<String> loadOrCreate() async {
    final existing = await _storage.read(key: _storageKey);
    if (existing != null && existing.isNotEmpty) {
      return existing;
    }
    final key = base64Encode(
      List<int>.generate(32, (_) => Random.secure().nextInt(256)),
    );
    await _storage.write(key: _storageKey, value: key);
    return key;
  }
}
