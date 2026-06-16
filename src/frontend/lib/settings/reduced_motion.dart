import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

const _prefKey = 'voice_reduced_motion';

final reducedMotionEnabledProvider =
    NotifierProvider<ReducedMotionNotifier, bool>(ReducedMotionNotifier.new);

class ReducedMotionNotifier extends Notifier<bool> {
  @override
  bool build() {
    _load();
    return false;
  }

  Future<void> _load() async {
    final prefs = await SharedPreferences.getInstance();
    state = prefs.getBool(_prefKey) ?? false;
  }

  Future<void> setEnabled(bool value) async {
    state = value;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_prefKey, value);
  }
}

/// Panel/sheet animation duration respecting reduced motion (docs/features/accessibility.md).
Duration panelAnimationDurationOf(BuildContext context) {
  final reduced = ProviderScope.containerOf(context, listen: true)
      .read(reducedMotionEnabledProvider);
  if (reduced || MediaQuery.disableAnimationsOf(context)) {
    return Duration.zero;
  }
  return const Duration(milliseconds: 200);
}
