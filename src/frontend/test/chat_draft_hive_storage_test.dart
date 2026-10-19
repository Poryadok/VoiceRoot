import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hive/hive.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/chat_draft_storage.dart';

void main() {
  late Directory directory;
  late Box<String> box;

  setUp(() async {
    directory = await Directory.systemTemp.createTemp('voice-draft-test-');
    box = await Hive.openBox<String>('drafts', path: directory.path);
    SharedPreferences.setMockInitialValues({});
  });
  tearDown(() async {
    await box.close();
    await directory.delete(recursive: true);
  });

  test(
    'disk reopen restores exact text and isolates profile/chat keys',
    () async {
      final storage = HiveChatDraftStorage(box);
      const first = ChatDraftKey(
        profileId: 'account-a-profile',
        chatId: 'chat-a',
      );
      const second = ChatDraftKey(
        profileId: 'account-b-profile',
        chatId: 'chat-a',
      );
      const otherChat = ChatDraftKey(
        profileId: 'account-a-profile',
        chatId: 'chat-b',
      );
      await storage.write(first, '  first\nчерновик 🦔');
      await storage.write(second, 'second account');
      await storage.write(otherChat, 'another chat');
      await box.close();
      box = await Hive.openBox<String>('drafts', path: directory.path);
      final reopened = HiveChatDraftStorage(box);
      expect(await reopened.read(first), '  first\nчерновик 🦔');
      expect(await reopened.read(second), 'second account');
      expect(await reopened.read(otherChat), 'another chat');
      await reopened.clear(first);
      await box.close();
      box = await Hive.openBox<String>('drafts', path: directory.path);
      expect(await HiveChatDraftStorage(box).read(first), isNull);
      expect(await HiveChatDraftStorage(box).read(second), 'second account');
    },
  );

  test(
    'migrates legacy text once and never resurrects cleared values',
    () async {
      final prefs = await SharedPreferences.getInstance();
      final legacy = SharedPreferencesChatDraftStorage(prefs);
      const key = ChatDraftKey(profileId: 'profile-a', chatId: 'chat-a');
      await legacy.write(key, 'existing draft');
      final storage = HiveChatDraftStorage(box, legacyPreferences: prefs);
      expect(await storage.read(key), 'existing draft');
      expect(await legacy.read(key), isNull);
      await legacy.write(key, 'stale old value');
      expect(await storage.read(key), 'existing draft');
      await storage.clear(key);
      expect(await storage.read(key), isNull);
      expect(await legacy.read(key), isNull);
    },
  );

  test('key component punctuation cannot alias another draft', () async {
    final storage = HiveChatDraftStorage(box);
    const first = ChatDraftKey(profileId: 'a.b', chatId: 'c');
    const second = ChatDraftKey(profileId: 'a', chatId: 'b.c');
    await storage.write(first, 'first');
    await storage.write(second, 'second');
    expect(await storage.read(first), 'first');
    expect(await storage.read(second), 'second');
  });

  test('typing during legacy restore keeps the newer draft', () async {
    final prefs = await SharedPreferences.getInstance();
    const key = ChatDraftKey(profileId: 'profile-a', chatId: 'chat-a');
    await SharedPreferencesChatDraftStorage(prefs).write(key, 'old draft');
    final storage = HiveChatDraftStorage(box, legacyPreferences: prefs);
    await Future.wait<Object?>([
      storage.read(key),
      storage.write(key, 'new draft'),
    ]);
    expect(await storage.read(key), 'new draft');
  });
}
