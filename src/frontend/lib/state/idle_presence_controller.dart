import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'auth_providers.dart';
import 'social_providers.dart';

/// Default idle threshold from [docs/features/presence.md] («Не активен» after 5 min).
const Duration kIdlePresenceTimeout = Duration(minutes: 5);

/// Override in tests to shorten the idle timer under [fakeAsync].
final idlePresenceTimeoutProvider = Provider<Duration>(
  (ref) => kIdlePresenceTimeout,
);

/// Tracks local input and sends `UpdatePresence` when the user goes idle.
///
/// Spec: [docs/features/presence.md], [docs/todo/client.md] Common Chat UX —
/// idle 5 min → `UpdatePresence idle`. Manual DND / invisible are not overridden.
class IdlePresenceController {
  IdlePresenceController(this._ref);

  final Ref _ref;

  Timer? _timer;
  var _started = false;
  var _disposed = false;

  /// When true, auto-idle must not change presence (user chose DND / invisible).
  var _manualLocked = false;

  /// True after we (or the user) published idle; cleared on activity → online.
  var _isIdle = false;

  /// Last status we attempted to push (for tests / debugging).
  String? lastPushedStatus;

  bool get isIdle => _isIdle;
  bool get isManualLocked => _manualLocked;

  void start() {
    if (_disposed || _started) return;
    _started = true;
    _armTimer();
  }

  /// Pointer / keyboard / scroll activity while the authenticated shell is open.
  void onUserActivity() {
    if (_disposed || !_started) return;
    _armTimer();
    if (_manualLocked) return;
    if (!_isIdle) return;
    _isIdle = false;
    unawaited(_push('online'));
  }

  /// Called after a successful manual presence change from the profile menu.
  void onManualStatus(String status) {
    if (_disposed) return;
    switch (status) {
      case 'dnd':
      case 'invisible':
        _manualLocked = true;
        _isIdle = false;
        _armTimer();
      case 'idle':
        _manualLocked = false;
        _isIdle = true;
        _armTimer();
      case 'online':
      default:
        _manualLocked = false;
        _isIdle = false;
        _armTimer();
    }
  }

  void _armTimer() {
    _timer?.cancel();
    if (_disposed || !_started) return;
    final timeout = _ref.read(idlePresenceTimeoutProvider);
    _timer = Timer(timeout, _onIdleTimeout);
  }

  void _onIdleTimeout() {
    if (_disposed || !_started) return;
    if (_manualLocked) return;
    if (_isIdle) return;
    _isIdle = true;
    unawaited(_push('idle'));
  }

  Future<void> _push(String status) async {
    lastPushedStatus = status;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    await _ref
        .read(voiceUsersClientProvider)
        .updatePresence(authorization: auth, status: status);
  }

  void dispose() {
    _disposed = true;
    _timer?.cancel();
    _timer = null;
  }
}

final idlePresenceControllerProvider = Provider<IdlePresenceController>((ref) {
  final controller = IdlePresenceController(ref);
  ref.onDispose(controller.dispose);
  return controller;
});

/// Starts idle tracking once the session is authenticated.
final idlePresenceLifecycleProvider = Provider<void>((ref) {
  final auth = ref.watch(authControllerProvider);
  if (!auth.isAuthenticated) return;
  ref.watch(idlePresenceControllerProvider).start();
});
