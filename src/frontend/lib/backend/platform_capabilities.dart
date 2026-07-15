import 'package:flutter/foundation.dart' show kIsWeb;

export 'screen_share_capabilities.dart';

/// Global PTT hotkeys require focus outside the browser tab (docs/features/platforms.md).
bool get canUseGlobalPushToTalkHotkey => !kIsWeb;
