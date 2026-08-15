import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../theme/voice_theme_providers.dart';

const themePreferencePrefKey = 'voice_theme_preference';

final appThemePreferenceProvider =
    NotifierProvider<AppThemePreferenceNotifier, AppThemePreference>(
      AppThemePreferenceNotifier.new,
    );

class AppThemePreferenceNotifier extends Notifier<AppThemePreference> {
  @override
  AppThemePreference build() {
    _load();
    return AppThemePreference.system;
  }

  Future<void> _load() async {
    final prefs = await SharedPreferences.getInstance();
    state = readThemePreference(prefs);
  }

  Future<void> setPreference(AppThemePreference value) async {
    state = value;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(themePreferencePrefKey, value.name);
  }
}

class SeededAppThemePreferenceNotifier extends AppThemePreferenceNotifier {
  SeededAppThemePreferenceNotifier(this.initial);

  final AppThemePreference initial;

  @override
  AppThemePreference build() => initial;
}

AppThemePreference readThemePreference(SharedPreferences prefs) {
  final raw = prefs.getString(themePreferencePrefKey);
  if (raw == null || raw.isEmpty) {
    return AppThemePreference.system;
  }
  for (final pref in AppThemePreference.values) {
    if (pref.name == raw) {
      return pref;
    }
  }
  return AppThemePreference.system;
}
