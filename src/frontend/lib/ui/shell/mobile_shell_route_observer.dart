import 'package:flutter/material.dart';

/// Tracks full-screen [PageRoute] pushes above the shell (not bottom sheets).
class MobileShellOverlayObserver extends NavigatorObserver {
  MobileShellOverlayObserver(this.onDepthChanged);

  final void Function(int delta) onDepthChanged;

  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (_countsAsShellOverlay(route, previousRoute)) {
      onDepthChanged(1);
    }
  }

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (_countsAsShellOverlay(route, previousRoute)) {
      onDepthChanged(-1);
    }
  }

  @override
  void didRemove(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (_countsAsShellOverlay(route, previousRoute)) {
      onDepthChanged(-1);
    }
  }

  bool _countsAsShellOverlay(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (previousRoute == null) return false;
    if (route is! PageRoute<dynamic>) return false;
    if (route.fullscreenDialog) return false;
    return true;
  }
}
