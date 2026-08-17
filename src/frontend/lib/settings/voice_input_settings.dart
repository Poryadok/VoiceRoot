import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

const _modePrefKey = 'voice_input_mode';
const _pttKeyPrefKey = 'voice_ptt_key_id';

enum VoiceInputMode { vad, ptt }

class VoiceInputSettings {
  const VoiceInputSettings({
    this.mode = VoiceInputMode.vad,
    this.pttKey = LogicalKeyboardKey.backquote,
  });

  final VoiceInputMode mode;
  final LogicalKeyboardKey pttKey;

  VoiceInputSettings copyWith({
    VoiceInputMode? mode,
    LogicalKeyboardKey? pttKey,
  }) {
    return VoiceInputSettings(
      mode: mode ?? this.mode,
      pttKey: pttKey ?? this.pttKey,
    );
  }
}

/// Effective local-mic mute: user mute wins; in PTT mode audio is sent only while held.
bool isMicEffectivelyMuted({
  required bool userMuted,
  required VoiceInputMode mode,
  required bool pttHeld,
}) {
  if (userMuted) return true;
  if (mode == VoiceInputMode.ptt) return !pttHeld;
  return false;
}

final voiceInputSettingsProvider =
    NotifierProvider<VoiceInputSettingsNotifier, VoiceInputSettings>(
      VoiceInputSettingsNotifier.new,
    );

class VoiceInputSettingsNotifier extends Notifier<VoiceInputSettings> {
  var _writeGeneration = 0;

  @override
  VoiceInputSettings build() {
    _load();
    return const VoiceInputSettings();
  }

  Future<void> _load() async {
    final gen = _writeGeneration;
    final prefs = await SharedPreferences.getInstance();
    if (gen != _writeGeneration) return;
    final modeRaw = prefs.getString(_modePrefKey);
    final keyId = prefs.getInt(_pttKeyPrefKey);
    state = VoiceInputSettings(
      mode: modeRaw == 'ptt' ? VoiceInputMode.ptt : VoiceInputMode.vad,
      pttKey: keyId == null
          ? LogicalKeyboardKey.backquote
          : LogicalKeyboardKey.findKeyByKeyId(keyId) ??
                LogicalKeyboardKey.backquote,
    );
  }

  Future<void> setMode(VoiceInputMode mode) async {
    _writeGeneration++;
    state = state.copyWith(mode: mode);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      _modePrefKey,
      mode == VoiceInputMode.ptt ? 'ptt' : 'vad',
    );
  }

  Future<void> setPttKey(LogicalKeyboardKey key) async {
    _writeGeneration++;
    state = state.copyWith(pttKey: key);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setInt(_pttKeyPrefKey, key.keyId);
  }
}
