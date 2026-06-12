import 'package:flutter/foundation.dart';
import 'package:in_app_update/in_app_update.dart';
import 'package:url_launcher/url_launcher.dart';

/// Opens store / update URL for mobile clients ([docs/features/updates.md]).
abstract interface class VersionUpdateLauncher {
  Future<void> launchUpdate({
    required String updateUrl,
    required bool immediate,
  });
}

class DefaultVersionUpdateLauncher implements VersionUpdateLauncher {
  const DefaultVersionUpdateLauncher();

  @override
  Future<void> launchUpdate({
    required String updateUrl,
    required bool immediate,
  }) async {
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      try {
        final info = await InAppUpdate.checkForUpdate();
        if (info.updateAvailability == UpdateAvailability.updateAvailable) {
          if (immediate) {
            await InAppUpdate.performImmediateUpdate();
            return;
          }
          await InAppUpdate.startFlexibleUpdate();
          await InAppUpdate.completeFlexibleUpdate();
          return;
        }
      } on Object {
        // Sideload / emulator: fall through to store URL.
      }
    }
    if (updateUrl.isEmpty) return;
    final uri = Uri.tryParse(updateUrl);
    if (uri == null) return;
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }
}

VersionUpdateLauncher? versionUpdateLauncherTestOverride;

VersionUpdateLauncher createVersionUpdateLauncher() {
  return versionUpdateLauncherTestOverride ?? const DefaultVersionUpdateLauncher();
}
