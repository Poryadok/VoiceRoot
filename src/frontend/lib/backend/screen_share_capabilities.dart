import 'dart:io' show Platform;

import 'package:flutter/foundation.dart'
    show TargetPlatform, defaultTargetPlatform, kIsWeb;

/// Desktop and web can initiate screen share; mobile is view-only per docs/features/screen-share.md.
bool get canStartScreenShare {
  if (kIsWeb) return true;
  return Platform.isWindows || Platform.isLinux || Platform.isMacOS;
}

bool get canCaptureSystemAudioWithScreenShare {
  return !kIsWeb && defaultTargetPlatform == TargetPlatform.windows;
}

/// The Windows desktop capturer exposes screen and window source IDs.
bool get canSelectDesktopScreenShareSource =>
    !kIsWeb && defaultTargetPlatform == TargetPlatform.windows;
