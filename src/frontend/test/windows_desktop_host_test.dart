import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/platform_capabilities.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/backend/windows_virtual_key.dart';
import 'package:voice_frontend/services/windows_desktop_host.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/windows_desktop_lifecycle.dart';

import 'support/auth_test_overrides.dart';

class _FixedVoiceInputSettings extends VoiceInputSettingsNotifier {
  _FixedVoiceInputSettings(this.fixed);
  final VoiceInputSettings fixed;

  @override
  VoiceInputSettings build() => fixed;
}

void main() {
  tearDown(() {
    debugDefaultTargetPlatformOverride = null;
  });

  test('canHideToSystemTray is Windows-only', () {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    expect(canHideToSystemTray, isTrue);

    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    expect(canHideToSystemTray, isFalse);
  });

  test('backquote maps to VK_OEM_3 for global PTT', () {
    expect(windowsVkCodeForLogicalKey(LogicalKeyboardKey.backquote), 0xC0);
    expect(windowsVkCodeForLogicalKey(LogicalKeyboardKey.keyV), 0x56);
  });

  test('method channel registers PTT hotkey and hide-to-tray', () async {
    TestWidgetsFlutterBinding.ensureInitialized();
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;

    const channel = MethodChannel(WindowsDesktopHost.channelName);
    final calls = <MethodCall>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
          calls.add(call);
          return null;
        });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    final host = MethodChannelWindowsDesktopHost();
    await host.hideToTray();
    await host.registerPttHotkey(vkCode: 0xC0, modifiers: 0);
    await host.quit();

    expect(calls.map((c) => c.method), [
      'hideToTray',
      'registerPttHotkey',
      'quit',
    ]);
    expect(calls[1].arguments, {'vkCode': 0xC0, 'modifiers': 0});
  });

  test('tray mute/deafen toggle call state; hide does not hang up', () async {
    final host = RecordingWindowsDesktopHost();
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        windowsDesktopHostProvider.overrideWithValue(host),
        voiceInputSettingsProvider.overrideWith(
          () => _FixedVoiceInputSettings(
            const VoiceInputSettings(mode: VoiceInputMode.ptt),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.active,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'me',
        calleeProfileId: 'peer',
        mediaKind: VoiceCallMediaKind.audio,
        status: VoiceCallStatus.active,
      ),
    );

    final lifecycle = container.read(windowsDesktopLifecycleProvider);
    await Future<void>.delayed(Duration.zero);

    host.emitPtt(held: true);
    await Future<void>.delayed(Duration.zero);
    expect(container.read(callControllerProvider).isPttHeld, isTrue);

    host.emitTrayMute();
    await Future<void>.delayed(Duration.zero);
    expect(container.read(callControllerProvider).isMuted, isTrue);

    host.emitTrayDeafen();
    await Future<void>.delayed(Duration.zero);
    expect(container.read(callControllerProvider).isSpeakerMuted, isTrue);

    await lifecycle.hideToTray();
    expect(host.hideCalls, 1);
    expect(container.read(callControllerProvider).phase, CallPhase.active);
    expect(host.quitCalls, 0);

    expect(host.registerHotkeyCalls, greaterThan(0));
  });
}
