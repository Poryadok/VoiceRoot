import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;

/// Desktop and web can initiate screen share; mobile is view-only per docs/features/screen-share.md.
bool get canStartScreenShare {
  if (kIsWeb) return true;
  return Platform.isWindows || Platform.isLinux || Platform.isMacOS;
}

bool get canCaptureSystemAudioWithScreenShare {
  if (kIsWeb) return false;
  return canStartScreenShare;
}
