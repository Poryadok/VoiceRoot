import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

const appIconPreferencePrefKey = 'voice_app_icon';

enum AppIconPreference {
  voiceSky('voice_sky', 'appIconVoiceSky', false),
  midnight('midnight', 'appIconMidnight', true),
  violet('violet', 'appIconViolet', true),
  sunrise('sunrise', 'appIconSunrise', true),
  mint('mint', 'appIconMint', true),
  coral('coral', 'appIconCoral', true);

  const AppIconPreference(this.id, this.labelKey, this.requiresPlus);

  final String id;
  final String labelKey;
  final bool requiresPlus;

  static AppIconPreference fromId(String? id) => values.firstWhere(
    (value) => value.id == id,
    orElse: () => AppIconPreference.voiceSky,
  );
}

AppIconPreference effectiveAppIcon(
  AppIconPreference selected, {
  required bool isPlusActiveOrGrace,
}) {
  return selected.requiresPlus && !isPlusActiveOrGrace
      ? AppIconPreference.voiceSky
      : selected;
}

class AppIconPreferenceStore {
  Future<AppIconPreference> load() async {
    final preferences = await SharedPreferences.getInstance();
    return AppIconPreference.fromId(
      preferences.getString(appIconPreferencePrefKey),
    );
  }

  Future<void> save(AppIconPreference value) async {
    final preferences = await SharedPreferences.getInstance();
    final saved = await preferences.setString(
      appIconPreferencePrefKey,
      value.id,
    );
    if (!saved) throw StateError('App icon preference was not saved');
  }
}

final appIconPreferenceStoreProvider = Provider<AppIconPreferenceStore>(
  (ref) => AppIconPreferenceStore(),
);

final appIconPreferenceProvider =
    AsyncNotifierProvider<AppIconPreferenceNotifier, AppIconPreference>(
      AppIconPreferenceNotifier.new,
    );

class AppIconPreferenceNotifier extends AsyncNotifier<AppIconPreference> {
  var _writeGeneration = 0;
  Future<void> _writeTail = Future<void>.value();
  AppIconPreference? _lastRequested;

  @override
  Future<AppIconPreference> build() async {
    final loaded = await ref.read(appIconPreferenceStoreProvider).load();
    _lastRequested = loaded;
    return loaded;
  }

  Future<void> setPreference(AppIconPreference value) {
    final generation = ++_writeGeneration;
    _lastRequested = value;
    state = AsyncData(value);
    final write = _writeTail.then(
      (_) => ref.read(appIconPreferenceStoreProvider).save(value),
    );
    _writeTail = write.catchError((_) {});
    return write.catchError((Object error, StackTrace stackTrace) {
      if (generation == _writeGeneration) {
        state = AsyncError(error, stackTrace);
      }
      Error.throwWithStackTrace(error, stackTrace);
    });
  }

  Future<void> retrySave() async {
    final value = state.valueOrNull ?? _lastRequested;
    if (value != null) await setPreference(value);
  }
}
