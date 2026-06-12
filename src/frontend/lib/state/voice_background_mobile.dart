import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Keeps voice audio active when the app is backgrounded on mobile
/// ([docs/features/voice-chat.md]).
abstract interface class VoiceBackgroundSession {
  Future<void> setActive(bool active);
}

class NoopVoiceBackgroundSession implements VoiceBackgroundSession {
  const NoopVoiceBackgroundSession();

  @override
  Future<void> setActive(bool active) async {}
}

/// Records [setActive] calls for tests.
class RecordingVoiceBackgroundSession implements VoiceBackgroundSession {
  bool isActive = false;
  int callCount = 0;

  @override
  Future<void> setActive(bool active) async {
    isActive = active;
    callCount++;
  }
}

final voiceBackgroundSessionProvider = Provider<VoiceBackgroundSession>((ref) {
  return createVoiceBackgroundSession();
});

/// Test override for [voiceBackgroundSessionProvider].
VoiceBackgroundSession? voiceBackgroundSessionTestOverride;

VoiceBackgroundSession createVoiceBackgroundSession() {
  if (voiceBackgroundSessionTestOverride != null) {
    return voiceBackgroundSessionTestOverride!;
  }
  if (kIsWeb) return const NoopVoiceBackgroundSession();
  return switch (defaultTargetPlatform) {
    TargetPlatform.android || TargetPlatform.iOS =>
      const _MobileVoiceBackgroundSession(),
    _ => const NoopVoiceBackgroundSession(),
  };
}

/// Mobile platforms: LiveKit continues playback when backgrounded once audio
/// session / foreground service requirements are met in native config.
class _MobileVoiceBackgroundSession implements VoiceBackgroundSession {
  const _MobileVoiceBackgroundSession();

  @override
  Future<void> setActive(bool active) async {
    // Native manifests (Android foreground service, iOS UIBackgroundModes audio)
    // satisfy platform requirements; LiveKit holds the audio session while
    // connected. No extra Dart-side work required beyond connect lifecycle.
  }
}
