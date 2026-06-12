import 'package:flutter/foundation.dart';

/// Returns Gateway `platform` for device registration.
String pushPlatformForTarget() {
  if (kIsWeb) return 'web';
  switch (defaultTargetPlatform) {
    case TargetPlatform.android:
      return 'android';
    case TargetPlatform.iOS:
      return 'ios';
    default:
      return 'desktop';
  }
}

/// Returns Notification Service `push_service` for the current platform.
String pushServiceForTarget() {
  if (!kIsWeb && defaultTargetPlatform == TargetPlatform.iOS) {
    return 'apns';
  }
  return 'fcm';
}
