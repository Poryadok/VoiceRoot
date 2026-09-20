import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/platform_capabilities.dart';
import '../backend/windows_virtual_key.dart';
import '../l10n/app_localizations.dart';
import '../services/windows_desktop_host.dart';
import '../settings/voice_input_settings.dart';
import 'auth_providers.dart';
import 'call_providers.dart';

/// Wires Windows tray + global PTT to [CallController] (platforms.md П.17).
class WindowsDesktopLifecycle {
  WindowsDesktopLifecycle(this._ref);

  final Ref _ref;
  var _attached = false;
  var _pttRegistered = false;
  int? _registeredVk;
  Future<void>? _syncInFlight;
  var _syncQueued = false;

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
    _ref.listen<String?>(authorizationHeaderProvider, (previous, next) {
      if (previous != null && next == null) {
        unawaited(_releasePtt());
      }
      unawaited(sync());
    });
    await sync();
  }

  Future<void> hideToTray() async {
    await _ref.read(windowsDesktopHostProvider).hideToTray();
  }

  Future<void> sync({AppLocalizations? l10n}) {
    final current = _syncInFlight;
    if (current != null) {
      _syncQueued = true;
      return current;
    }
    final next = _syncUntilCurrent(l10n: l10n);
    _syncInFlight = next;
    return next.whenComplete(() {
      if (identical(_syncInFlight, next)) {
        _syncInFlight = null;
      }
    });
  }

  Future<void> _syncUntilCurrent({AppLocalizations? l10n}) async {
    do {
      _syncQueued = false;
      await _syncImpl(l10n: l10n);
    } while (_syncQueued);
  }

  Future<void> _syncImpl({AppLocalizations? l10n}) async {
    final host = _ref.read(windowsDesktopHostProvider);
    final call = _ref.read(callControllerProvider);
    final input = _ref.read(voiceInputSettingsProvider);
    await host.setTrayState(
      voiceActive: call.hasBoundVoiceSession,
      muted: call.isMuted,
      deafened: call.isSpeakerMuted,
      muteLabel: l10n?.callMute ?? 'Mute',
      unmuteLabel: l10n?.callUnmute ?? 'Unmute',
      deafenLabel: l10n?.callSpeakerOff ?? 'Deafen',
      undeafenLabel: l10n?.callSpeakerOn ?? 'Undeafen',
      quitLabel: l10n?.trayQuit ?? 'Quit',
    );

    final hasAuthenticatedVoiceSession =
        _ref.read(authorizationHeaderProvider) != null &&
        call.hasBoundVoiceSession;
    if (canUseGlobalPushToTalkHotkey &&
        hasAuthenticatedVoiceSession &&
        input.mode == VoiceInputMode.ptt) {
      final vk = windowsVkCodeForLogicalKey(input.pttKey);
      if (vk != null) {
        if (!_pttRegistered || _registeredVk != vk) {
          await host.registerPttHotkey(vkCode: vk, modifiers: 0);
          _pttRegistered = true;
          _registeredVk = vk;
        }
        return;
      }
    }
    await _releasePtt();
  }

  Future<void> _releasePtt() async {
    final call = _ref.read(callControllerProvider);
    if (call.isPttHeld) {
      await _ref.read(callControllerProvider.notifier).setPttHeld(false);
    }
    if (_pttRegistered) {
      await _ref.read(windowsDesktopHostProvider).unregisterPttHotkey();
      _pttRegistered = false;
      _registeredVk = null;
    }
  }

  void _onEvent(WindowsDesktopHostEvent event) {
    final call = _ref.read(callControllerProvider.notifier);
    switch (event.kind) {
      case WindowsDesktopHostEventKind.ptt:
        unawaited(call.setPttHeld(event.pttHeld ?? false));
      case WindowsDesktopHostEventKind.trayMute:
        if (_ref.read(callControllerProvider).hasBoundVoiceSession) {
          unawaited(call.setMuted(!_ref.read(callControllerProvider).isMuted));
        }
      case WindowsDesktopHostEventKind.trayDeafen:
        if (_ref.read(callControllerProvider).hasBoundVoiceSession) {
          unawaited(
            call.setSpeakerMuted(
              !_ref.read(callControllerProvider).isSpeakerMuted,
            ),
          );
        }
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
