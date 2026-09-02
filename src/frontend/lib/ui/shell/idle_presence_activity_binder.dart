import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../state/auth_providers.dart';
import '../../state/idle_presence_controller.dart';

/// Forwards pointer / keyboard activity to [IdlePresenceController].
///
/// Spec: idle after 5 minutes with no activity → `UpdatePresence idle`
/// ([docs/features/presence.md]).
class IdlePresenceActivityBinder extends ConsumerStatefulWidget {
  const IdlePresenceActivityBinder({super.key, required this.child});

  final Widget child;

  @override
  ConsumerState<IdlePresenceActivityBinder> createState() =>
      _IdlePresenceActivityBinderState();
}

class _IdlePresenceActivityBinderState
    extends ConsumerState<IdlePresenceActivityBinder> {
  IdlePresenceController? _controller;

  @override
  void initState() {
    super.initState();
    HardwareKeyboard.instance.addHandler(_onKeyEvent);
  }

  @override
  void dispose() {
    HardwareKeyboard.instance.removeHandler(_onKeyEvent);
    // Sync cancel: UncontrolledProviderScope does not dispose an external
    // ProviderContainer, so ref.onDispose alone leaves the idle Timer pending.
    _controller?.stop();
    _controller = null;
    super.dispose();
  }

  bool _onKeyEvent(KeyEvent event) {
    if (event is KeyDownEvent || event is KeyRepeatEvent) {
      _controller?.onUserActivity();
    }
    return false;
  }

  void _onActivity() {
    _controller?.onUserActivity();
  }

  @override
  Widget build(BuildContext context) {
    final controller = ref.watch(idlePresenceControllerProvider);
    _controller = controller;
    ref.watch(idlePresenceLifecycleProvider);
    // Lifecycle Provider may not rebuild after [stop] on dispose; re-arm when
    // this binder is (re)mounted under an authenticated session.
    final auth = ref.watch(authControllerProvider);
    if (auth.isAuthenticated) {
      controller.start();
    }
    return Listener(
      behavior: HitTestBehavior.translucent,
      onPointerDown: (_) => _onActivity(),
      onPointerSignal: (_) => _onActivity(),
      child: widget.child,
    );
  }
}
