import 'package:flutter/foundation.dart';

/// Returns Notification Service `push_service` for VoIP tokens on iOS.
String? voipPushServiceForTarget() {
  if (!kIsWeb && defaultTargetPlatform == TargetPlatform.iOS) {
    return 'voip_apns';
  }
  return null;
}

/// Whether VoIP push registration should run on the current platform.
bool get isVoIPPushSupported => voipPushServiceForTarget() != null;
