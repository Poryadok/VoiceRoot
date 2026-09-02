import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

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
  @override
  void initState() {
    super.initState();
    HardwareKeyboard.instance.addHandler(_onKeyEvent);
  }

  @override
  void dispose() {
    HardwareKeyboard.instance.removeHandler(_onKeyEvent);
    super.dispose();
  }

  bool _onKeyEvent(KeyEvent event) {
    if (event is KeyDownEvent || event is KeyRepeatEvent) {
      ref.read(idlePresenceControllerProvider).onUserActivity();
    }
    return false;
  }

  void _onActivity() {
    ref.read(idlePresenceControllerProvider).onUserActivity();
  }

  @override
  Widget build(BuildContext context) {
    ref.watch(idlePresenceLifecycleProvider);
    return Listener(
      behavior: HitTestBehavior.translucent,
      onPointerDown: (_) => _onActivity(),
      onPointerSignal: (_) => _onActivity(),
      child: widget.child,
    );
  }
}
