import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/platform_capabilities.dart';
import '../backend/windows_virtual_key.dart';
import '../l10n/app_localizations.dart';
import '../services/windows_desktop_host.dart';
import '../settings/voice_input_settings.dart';
import 'call_providers.dart';

/// Wires Windows tray + global PTT to [CallController] (platforms.md П.17).
class WindowsDesktopLifecycle {
  WindowsDesktopLifecycle(this._ref);

  final Ref _ref;
  var _attached = false;

  Future<void> attach() async {
    if (_attached) return;
    _attached = true;
    _ref.read(windowsDesktopHostProvider).setListener(_onEvent);
    _ref.listen<CallState>(callControllerProvider, (_, _) {
      unawaited(sync());
    });
    _ref.listen<VoiceInputSettings>(voiceInputSettingsProvider, (_, _) {
      unawaited(sync());
    });
    await sync();
  }

  Future<void> hideToTray() async {
    await _ref.read(windowsDesktopHostProvider).hideToTray();
  }

  Future<void> sync({AppLocalizations? l10n}) async {
    final host = _ref.read(windowsDesktopHostProvider);
    final call = _ref.read(callControllerProvider);
    final input = _ref.read(voiceInputSettingsProvider);
    await host.setTrayState(
      muted: call.isMuted,
      deafened: call.isSpeakerMuted,
      muteLabel: l10n?.callMute ?? 'Mute',
      unmuteLabel: l10n?.callUnmute ?? 'Unmute',
      deafenLabel: l10n?.callSpeakerOff ?? 'Deafen',
      undeafenLabel: l10n?.callSpeakerOn ?? 'Undeafen',
      quitLabel: l10n?.trayQuit ?? 'Quit',
    );

    if (canUseGlobalPushToTalkHotkey && input.mode == VoiceInputMode.ptt) {
      final vk = windowsVkCodeForLogicalKey(input.pttKey);
      if (vk != null) {
        await host.registerPttHotkey(vkCode: vk, modifiers: 0);
        return;
      }
    }
    await host.unregisterPttHotkey();
  }

  void _onEvent(WindowsDesktopHostEvent event) {
    final call = _ref.read(callControllerProvider.notifier);
    switch (event.kind) {
      case WindowsDesktopHostEventKind.ptt:
        unawaited(call.setPttHeld(event.pttHeld ?? false));
      case WindowsDesktopHostEventKind.trayMute:
        unawaited(call.setMuted(!_ref.read(callControllerProvider).isMuted));
      case WindowsDesktopHostEventKind.trayDeafen:
        unawaited(
          call.setSpeakerMuted(
            !_ref.read(callControllerProvider).isSpeakerMuted,
          ),
        );
      case WindowsDesktopHostEventKind.trayQuit:
        unawaited(_ref.read(windowsDesktopHostProvider).quit());
      case WindowsDesktopHostEventKind.trayShow:
        unawaited(_ref.read(windowsDesktopHostProvider).showWindow());
    }
  }
}

final windowsDesktopLifecycleProvider = Provider<WindowsDesktopLifecycle>((
  ref,
) {
  final lifecycle = WindowsDesktopLifecycle(ref);
  unawaited(lifecycle.attach());
  return lifecycle;
});
