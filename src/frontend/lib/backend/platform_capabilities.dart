import 'package:flutter/foundation.dart'
    show TargetPlatform, defaultTargetPlatform, kIsWeb;

export 'screen_share_capabilities.dart';

/// Global PTT hotkeys require focus outside the browser tab (docs/features/platforms.md).
bool get canUseGlobalPushToTalkHotkey => !kIsWeb;

/// Close-to-tray is Windows-only (docs/features/platforms.md П.17).
bool get canHideToSystemTray =>
    !kIsWeb && defaultTargetPlatform == TargetPlatform.windows;
