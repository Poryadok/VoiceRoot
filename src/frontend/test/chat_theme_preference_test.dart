import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/settings/chat_theme_preference.dart';

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  test(
    'stored themes are isolated by chat and survive container restart',
    () async {
      final first = ProviderContainer();
      addTearDown(first.dispose);

      expect(await first.read(chatThemePreferenceProvider.future), isEmpty);
      await first
          .read(chatThemePreferenceProvider.notifier)
          .setForChat('chat-a', ChatTheme.violet);
      await first
          .read(chatThemePreferenceProvider.notifier)
          .setForChat('chat-b', ChatTheme.midnight);

      final prefs = await SharedPreferences.getInstance();
      expect(readChatThemePreferences(prefs), {
        'chat-a': ChatTheme.violet,
        'chat-b': ChatTheme.midnight,
      });

      final restarted = ProviderContainer();
      addTearDown(restarted.dispose);
      expect(await restarted.read(chatThemePreferenceProvider.future), {
        'chat-a': ChatTheme.violet,
        'chat-b': ChatTheme.midnight,
      });
    },
  );

  test('reset removes only the selected chat override', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    await container.read(chatThemePreferenceProvider.future);

    final controller = container.read(chatThemePreferenceProvider.notifier);
    await controller.setForChat('chat-a', ChatTheme.sunset);
    await controller.setForChat('chat-b', ChatTheme.ocean);
    await controller.resetForChat('chat-a');

    expect(await container.read(chatThemePreferenceProvider.future), {
      'chat-b': ChatTheme.ocean,
    });
    expect(readChatThemePreferences(await SharedPreferences.getInstance()), {
      'chat-b': ChatTheme.ocean,
    });
  });

  test('unknown stored catalog names are ignored safely', () async {
    SharedPreferences.setMockInitialValues({
      chatThemePreferencePrefKey: '{"chat-a":"violet","chat-b":"new_theme"}',
    });

    final container = ProviderContainer();
    addTearDown(container.dispose);
    expect(await container.read(chatThemePreferenceProvider.future), {
      'chat-a': ChatTheme.violet,
    });
  });
}
