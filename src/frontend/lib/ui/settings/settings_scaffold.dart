import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';

/// Marks detail routes hosted inside the persistent desktop Settings shell.
class SettingsDetailScope extends InheritedWidget {
  const SettingsDetailScope({
    super.key,
    required this.leaveGuards,
    required super.child,
  });

  final SettingsLeaveGuards leaveGuards;

  static SettingsDetailScope? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<SettingsDetailScope>();

  void registerLeaveGuard(Object owner, Future<bool> Function() guard) =>
      leaveGuards.register(owner, guard);

  void unregisterLeaveGuard(Object owner) => leaveGuards.unregister(owner);

  static bool isEmbedded(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<SettingsDetailScope>() != null;

  @override
  bool updateShouldNotify(SettingsDetailScope oldWidget) =>
      oldWidget.leaveGuards != leaveGuards;
}

/// Lets an editor confirm an asynchronous discard before its route is replaced.
class SettingsLeaveGuards {
  final Map<Object, Future<bool> Function()> _guards = {};
  bool get isNotEmpty => _guards.isNotEmpty;
  void register(Object owner, Future<bool> Function() guard) =>
      _guards[owner] = guard;
  void unregister(Object owner) => _guards.remove(owner);
  Future<bool> canLeave() async {
    for (final guard in List.of(_guards.values)) {
      if (!await guard()) return false;
    }
    return true;
  }
}

/// Shared full-height desktop chrome. The app rail is supplied by its live host.
class SettingsScaffold extends StatelessWidget {
  const SettingsScaffold({
    super.key,
    required this.rail,
    required this.navigation,
    required this.detail,
    required this.onClose,
    required this.leaveGuards,
  });

  final Widget rail;
  final Widget navigation;
  final Widget detail;
  final VoidCallback onClose;
  final SettingsLeaveGuards leaveGuards;

  @override
  Widget build(BuildContext context) {
    final colors = VoiceColors.of(context);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        rail,
        Container(
          key: const Key('settings_navigation'),
          width: VoiceLayout.desktopNavigationWidth,
          decoration: BoxDecoration(
            color: colors.surface,
            border: Border(right: BorderSide(color: colors.borderDefault)),
          ),
          child: navigation,
        ),
        Expanded(
          child: Stack(
            fit: StackFit.expand,
            children: [
              SettingsDetailScope(leaveGuards: leaveGuards, child: detail),
              Positioned(
                top: 8,
                right: 12,
                child: IconButton(
                  key: const Key('settings_close'),
                  tooltip: MaterialLocalizations.of(context).closeButtonTooltip,
                  onPressed: onClose,
                  icon: const Icon(Icons.close, size: 20),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
