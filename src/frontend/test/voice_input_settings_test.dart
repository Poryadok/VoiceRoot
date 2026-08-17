import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  group('isMicEffectivelyMuted (voice-chat.md PTT vs VAD)', () {
    test('VAD follows user mute only', () {
      expect(
        isMicEffectivelyMuted(
          userMuted: false,
          mode: VoiceInputMode.vad,
          pttHeld: false,
        ),
        isFalse,
      );
      expect(
        isMicEffectivelyMuted(
          userMuted: true,
          mode: VoiceInputMode.vad,
          pttHeld: true,
        ),
        isTrue,
      );
    });

    test('PTT is muted until the configured key/button is held', () {
      expect(
        isMicEffectivelyMuted(
          userMuted: false,
          mode: VoiceInputMode.ptt,
          pttHeld: false,
        ),
        isTrue,
      );
      expect(
        isMicEffectivelyMuted(
          userMuted: false,
          mode: VoiceInputMode.ptt,
          pttHeld: true,
        ),
        isFalse,
      );
    });

    test('user mute overrides PTT hold', () {
      expect(
        isMicEffectivelyMuted(
          userMuted: true,
          mode: VoiceInputMode.ptt,
          pttHeld: true,
        ),
        isTrue,
      );
    });
  });

  test('default voice input is VAD with backtick PTT key', () {
    const settings = VoiceInputSettings();
    expect(settings.mode, VoiceInputMode.vad);
    expect(settings.pttKey, LogicalKeyboardKey.backquote);
  });

  test('notifier persists PTT mode and keybind', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    expect(container.read(voiceInputSettingsProvider).mode, VoiceInputMode.vad);

    await container
        .read(voiceInputSettingsProvider.notifier)
        .setMode(VoiceInputMode.ptt);
    await container
        .read(voiceInputSettingsProvider.notifier)
        .setPttKey(LogicalKeyboardKey.keyV);

    expect(container.read(voiceInputSettingsProvider).mode, VoiceInputMode.ptt);
    expect(
      container.read(voiceInputSettingsProvider).pttKey,
      LogicalKeyboardKey.keyV,
    );

    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('voice_input_mode'), 'ptt');
    expect(prefs.getInt('voice_ptt_key_id'), LogicalKeyboardKey.keyV.keyId);
  });
}
