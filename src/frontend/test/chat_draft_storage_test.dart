import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/backend/chat_draft_storage.dart';
import 'package:voice_frontend/state/chat_draft_providers.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  group('InMemoryChatDraftStorage', () {
    test('keeps drafts isolated by active profile and chat', () async {
      final storage = InMemoryChatDraftStorage();

      await storage.write(
        const ChatDraftKey(profileId: 'profile-a', chatId: 'chat-a'),
        'alpha',
      );
      await storage.write(
        const ChatDraftKey(profileId: 'profile-a', chatId: 'chat-b'),
        'bravo',
      );
      await storage.write(
        const ChatDraftKey(profileId: 'profile-b', chatId: 'chat-a'),
        'charlie',
      );

      expect(
        await storage.read(
          const ChatDraftKey(profileId: 'profile-a', chatId: 'chat-a'),
        ),
        'alpha',
      );
      expect(
        await storage.read(
          const ChatDraftKey(profileId: 'profile-a', chatId: 'chat-b'),
        ),
        'bravo',
      );
      expect(
        await storage.read(
          const ChatDraftKey(profileId: 'profile-b', chatId: 'chat-a'),
        ),
        'charlie',
      );
    });

    test(
      'restores a persisted draft and clears it after send or discard',
      () async {
        final storage = InMemoryChatDraftStorage();
        const key = ChatDraftKey(profileId: 'profile-a', chatId: 'chat-a');

        await storage.write(key, 'unsent message');
        expect(await storage.read(key), 'unsent message');

        await storage.clear(key);
        expect(await storage.read(key), isNull);
      },
    );
  });

  test('SharedPreferences storage restores after a simulated reload', () async {
    SharedPreferences.setMockInitialValues({});
    final first = SharedPreferencesChatDraftStorage(
      await SharedPreferences.getInstance(),
    );
    const key = ChatDraftKey(profileId: 'profile-a', chatId: 'chat-a');
    await first.write(key, 'survives reload');

    final reloaded = SharedPreferencesChatDraftStorage(
      await SharedPreferences.getInstance(),
    );
    expect(await reloaded.read(key), 'survives reload');
  });

  test(
    'draft state restores after reload and clears on an explicit discard',
    () async {
      final storage = InMemoryChatDraftStorage();
      const key = ChatDraftKey(profileId: 'profile-a', chatId: 'chat-a');
      await storage.write(key, 'reloaded draft');
      final container = ProviderContainer(
        overrides: [chatDraftStorageProvider.overrideWithValue(storage)],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(chatDraftProvider(key), (_, _) {});
      addTearDown(subscription.close);

      await Future<void>.delayed(Duration.zero);
      expect(
        container.read(chatDraftProvider(key)).valueOrNull,
        'reloaded draft',
      );
      await container.read(chatDraftProvider(key).notifier).clear();
      expect(container.read(chatDraftProvider(key)).valueOrNull, isEmpty);
      expect(await storage.read(key), isNull);
    },
  );
}
