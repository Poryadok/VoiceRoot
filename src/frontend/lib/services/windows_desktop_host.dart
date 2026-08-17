import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Native Windows tray + global PTT (docs/features/platforms.md П.17).
abstract class WindowsDesktopHost {
  static const channelName = 'voice/windows_desktop';

  void setListener(WindowsDesktopHostListener? listener);

  Future<void> setTrayState({
    required bool muted,
    required bool deafened,
    required String muteLabel,
    required String unmuteLabel,
    required String deafenLabel,
    required String undeafenLabel,
    required String quitLabel,
  });

  Future<void> registerPttHotkey({required int vkCode, required int modifiers});

  Future<void> unregisterPttHotkey();

  Future<void> showWindow();

  Future<void> hideToTray();

  Future<void> quit();
}

typedef WindowsDesktopHostListener =
    void Function(WindowsDesktopHostEvent event);

enum WindowsDesktopHostEventKind {
  ptt,
  trayMute,
  trayDeafen,
  trayQuit,
  trayShow,
}

class WindowsDesktopHostEvent {
  const WindowsDesktopHostEvent(this.kind, {this.pttHeld});

  final WindowsDesktopHostEventKind kind;
  final bool? pttHeld;
}

class MethodChannelWindowsDesktopHost implements WindowsDesktopHost {
  MethodChannelWindowsDesktopHost({MethodChannel? channel})
    : _channel =
          channel ?? const MethodChannel(WindowsDesktopHost.channelName) {
    _channel.setMethodCallHandler(_onMethodCall);
  }

  final MethodChannel _channel;
  WindowsDesktopHostListener? _listener;

  @override
  void setListener(WindowsDesktopHostListener? listener) {
    _listener = listener;
  }

  Future<dynamic> _onMethodCall(MethodCall call) async {
    switch (call.method) {
      case 'ptt':
        final args = call.arguments;
        final held = args is Map && args['held'] == true;
        _listener?.call(
          WindowsDesktopHostEvent(
            WindowsDesktopHostEventKind.ptt,
            pttHeld: held,
          ),
        );
      case 'trayMute':
        _listener?.call(
          const WindowsDesktopHostEvent(WindowsDesktopHostEventKind.trayMute),
        );
      case 'trayDeafen':
        _listener?.call(
          const WindowsDesktopHostEvent(WindowsDesktopHostEventKind.trayDeafen),
        );
      case 'trayQuit':
        _listener?.call(
          const WindowsDesktopHostEvent(WindowsDesktopHostEventKind.trayQuit),
        );
      case 'trayShow':
        _listener?.call(
          const WindowsDesktopHostEvent(WindowsDesktopHostEventKind.trayShow),
        );
    }
    return null;
  }

  @override
  Future<void> setTrayState({
    required bool muted,
    required bool deafened,
    required String muteLabel,
    required String unmuteLabel,
    required String deafenLabel,
    required String undeafenLabel,
    required String quitLabel,
  }) async {
    await _channel.invokeMethod<void>('setTrayState', {
      'muted': muted,
      'deafened': deafened,
      'muteLabel': muteLabel,
      'unmuteLabel': unmuteLabel,
      'deafenLabel': deafenLabel,
      'undeafenLabel': undeafenLabel,
      'quitLabel': quitLabel,
    });
  }

  @override
  Future<void> registerPttHotkey({
    required int vkCode,
    required int modifiers,
  }) async {
    await _channel.invokeMethod<void>('registerPttHotkey', {
      'vkCode': vkCode,
      'modifiers': modifiers,
    });
  }

  @override
  Future<void> unregisterPttHotkey() async {
    await _channel.invokeMethod<void>('unregisterPttHotkey');
  }

  @override
  Future<void> showWindow() async {
    await _channel.invokeMethod<void>('showWindow');
  }

  @override
  Future<void> hideToTray() async {
    await _channel.invokeMethod<void>('hideToTray');
  }

  @override
  Future<void> quit() async {
    await _channel.invokeMethod<void>('quit');
  }
}

class RecordingWindowsDesktopHost implements WindowsDesktopHost {
  WindowsDesktopHostListener? listener;
  int hideCalls = 0;
  int quitCalls = 0;
  int registerHotkeyCalls = 0;
  int? lastVkCode;

  @override
  void setListener(WindowsDesktopHostListener? next) {
    listener = next;
  }

  void emitPtt({required bool held}) {
    listener?.call(
      WindowsDesktopHostEvent(WindowsDesktopHostEventKind.ptt, pttHeld: held),
    );
  }

  void emitTrayMute() {
    listener?.call(
      const WindowsDesktopHostEvent(WindowsDesktopHostEventKind.trayMute),
    );
  }

  void emitTrayDeafen() {
    listener?.call(
      const WindowsDesktopHostEvent(WindowsDesktopHostEventKind.trayDeafen),
    );
  }

  @override
  Future<void> setTrayState({
    required bool muted,
    required bool deafened,
    required String muteLabel,
    required String unmuteLabel,
    required String deafenLabel,
    required String undeafenLabel,
    required String quitLabel,
  }) async {}

  @override
  Future<void> registerPttHotkey({
    required int vkCode,
    required int modifiers,
  }) async {
    registerHotkeyCalls++;
    lastVkCode = vkCode;
  }

  @override
  Future<void> unregisterPttHotkey() async {}

  @override
  Future<void> showWindow() async {}

  @override
  Future<void> hideToTray() async {
    hideCalls++;
  }

  @override
  Future<void> quit() async {
    quitCalls++;
  }
}

class NoopWindowsDesktopHost implements WindowsDesktopHost {
  const NoopWindowsDesktopHost();

  @override
  void setListener(WindowsDesktopHostListener? listener) {}

  @override
  Future<void> setTrayState({
    required bool muted,
    required bool deafened,
    required String muteLabel,
    required String unmuteLabel,
    required String deafenLabel,
    required String undeafenLabel,
    required String quitLabel,
  }) async {}

  @override
  Future<void> registerPttHotkey({
    required int vkCode,
    required int modifiers,
  }) async {}

  @override
  Future<void> unregisterPttHotkey() async {}

  @override
  Future<void> showWindow() async {}

  @override
  Future<void> hideToTray() async {}

  @override
  Future<void> quit() async {}
}

final windowsDesktopHostProvider = Provider<WindowsDesktopHost>((ref) {
  if (!kIsWeb && defaultTargetPlatform == TargetPlatform.windows) {
    return MethodChannelWindowsDesktopHost();
  }
  return const NoopWindowsDesktopHost();
});
