@TestOn('browser')
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:hive/hive.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/chat_draft_storage.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'browser IndexedDB restores migrated drafts after closing storage',
    () async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      const key = ChatDraftKey(
        profileId: 'browser-profile-a',
        chatId: 'browser-chat',
      );
      const other = ChatDraftKey(
        profileId: 'browser-profile-b',
        chatId: 'browser-chat',
      );
      final first = await HiveChatDraftStorage.open(prefs);
      addTearDown(() async {
        await Hive.close();
        await Hive.deleteBoxFromDisk('voice_chat_drafts');
      });
      await first.clear(key);
      await first.clear(other);
      await SharedPreferencesChatDraftStorage(
        prefs,
      ).write(key, 'unsent before reload');
      expect(await first.read(key), 'unsent before reload');
      await first.write(other, 'other account');
      await Hive.close();
      final reopened = await HiveChatDraftStorage.open(prefs);
      expect(await reopened.read(key), 'unsent before reload');
      expect(await reopened.read(other), 'other account');
      await reopened.clear(key);
      await Hive.close();
      final third = await HiveChatDraftStorage.open(prefs);
      expect(await third.read(key), isNull);
      expect(await third.read(other), 'other account');
    },
  );
}
