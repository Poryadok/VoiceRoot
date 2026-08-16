import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/client_version.dart';
import 'package:voice_frontend/backend/platform_capabilities.dart';
import 'package:voice_frontend/services/desktop_updater_service.dart';

/// Windows desktop smoke for what ships today (P3.2 / inventory PL-03, UPD-03).
///
/// Product gaps (roadmap П.17–18): system tray, real global PTT hotkey binding,
/// background-voice tray lifecycle, and game overlay are not implemented — this
/// file only asserts capability gates + WinSparkle auto-update stub wiring.
void main() {
  tearDown(() {
    debugDefaultTargetPlatformOverride = null;
  });

  group('Windows desktop capability gates', () {
    test('global PTT hotkey capability is enabled off-web (desktop/mobile)', () {
      // VM tests are never kIsWeb; desktop Windows inherits the same gate.
      expect(canUseGlobalPushToTalkHotkey, isTrue);
    });

    test('windows platform id and version headers for Gateway policy', () {
      debugDefaultTargetPlatformOverride = TargetPlatform.windows;

      expect(ClientVersion.platform, 'windows');
      expect(ClientVersion.sendVersionHeaders, isTrue);
      expect(ClientVersion.headers['X-Voice-Client-Platform'], 'windows');
      expect(ClientVersion.headers['X-Voice-Client-Version'], isNotEmpty);
    });
  });

  group('Windows desktop auto-update stub (UPD-03)', () {
    test('uses WinSparkle-backed auto updater on windows', () {
      debugDefaultTargetPlatformOverride = TargetPlatform.windows;

      expect(ClientVersion.usesDesktopAutoUpdater, isTrue);
      expect(ClientVersion.desktopUpdaterChannel, 'voice/desktop_updater');
    });

    test('provider selects AutoUpdaterDesktopService on windows', () {
      debugDefaultTargetPlatformOverride = TargetPlatform.windows;

      final container = ProviderContainer();
      addTearDown(container.dispose);

      expect(
        container.read(desktopUpdaterServiceProvider),
        isA<AutoUpdaterDesktopService>(),
      );
    });

    test('provider selects NoopDesktopUpdaterService off windows', () {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;

      final container = ProviderContainer();
      addTearDown(container.dispose);

      expect(
        container.read(desktopUpdaterServiceProvider),
        isA<NoopDesktopUpdaterService>(),
      );
    });

    test('method-channel stub checkForUpdate maps downloading status', () async {
      TestWidgetsFlutterBinding.ensureInitialized();

      const channel = MethodChannel(ClientVersion.desktopUpdaterChannel);
      final service = MethodChannelDesktopUpdaterService();

      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, (call) async {
        expect(call.method, 'checkForUpdate');
        expect(
          call.arguments,
          'https://updates.voice.example/windows/appcast.xml',
        );
        return {'status': 'downloading'};
      });
      addTearDown(() {
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(channel, null);
      });

      final status = await service.checkForUpdate(
        'https://updates.voice.example/windows/appcast.xml',
      );
      expect(status, DesktopUpdateStatus.downloading);
    });
  });
}
