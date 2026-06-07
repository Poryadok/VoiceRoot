import 'package:flutter/foundation.dart';

/// Client version headers for Gateway policy ([docs/features/updates.md]).
abstract final class ClientVersion {
  static const appVersion = String.fromEnvironment(
    'VOICE_APP_VERSION',
    defaultValue: '1.0.0',
  );

  /// Platform id for `GET /api/v1/version` and `X-Voice-Client-Platform`.
  static String get platform {
    if (kIsWeb) return 'web';
    return switch (defaultTargetPlatform) {
      TargetPlatform.android => 'android',
      TargetPlatform.iOS => 'ios',
      TargetPlatform.windows => 'windows',
      TargetPlatform.macOS => 'macos',
      TargetPlatform.linux => 'linux',
      _ => 'unknown',
    };
  }

  /// Whether to attach version headers on REST (web skips per updates.md).
  static bool get sendVersionHeaders => !kIsWeb;

  static Map<String, String> get headers {
    if (!sendVersionHeaders) return const {};
    return {
      'X-Voice-Client-Platform': platform,
      'X-Voice-Client-Version': appVersion,
    };
  }
}
