import 'package:flutter/foundation.dart'
    show TargetPlatform, defaultTargetPlatform, kIsWeb;

export 'screen_share_capabilities.dart';

/// Windows supplies the OS-level PTT hotkey; Web keeps focused in-app PTT only
/// (docs/features/platforms.md П.17).
bool get canUseGlobalPushToTalkHotkey =>
    !kIsWeb && defaultTargetPlatform == TargetPlatform.windows;

/// Close-to-tray is Windows-only (docs/features/platforms.md П.17).
bool get canHideToSystemTray =>
    !kIsWeb && defaultTargetPlatform == TargetPlatform.windows;
