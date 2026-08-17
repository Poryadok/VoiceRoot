import 'dart:convert';
import 'dart:math';

import 'package:flutter/services.dart';
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

  /// Live integration tests on desktop may run without flutter_secure_storage.
  static Future<String> loadOrCreateForLiveIntegration() async {
    const fromEnv = String.fromEnvironment('VOICE_MESSAGE_CACHE_TEST_KEY');
    if (fromEnv.isNotEmpty) {
      return fromEnv;
    }
    try {
      return await loadOrCreate();
    } on MissingPluginException {
      return _liveIntegrationFallbackKey();
    } on PlatformException catch (e) {
      if (e.code == 'channel-error' || e.code == 'NotImplemented') {
        return _liveIntegrationFallbackKey();
      }
      rethrow;
    }
  }

  static String _liveIntegrationFallbackKey() {
    final padded = 'voice-live-integration-cache-key-v1'.padRight(32).substring(0, 32);
    return base64Encode(utf8.encode(padded));
  }
}
