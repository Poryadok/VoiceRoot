import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/settings/app_icon_preference.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() => SharedPreferences.setMockInitialValues({}));

  test('defines the six stable supported identities', () {
    expect(AppIconPreference.values.map((value) => value.id), [
      'voice_sky',
      'midnight',
      'violet',
      'sunrise',
      'mint',
      'coral',
    ]);
  });

  test(
    'persists a selected identity and reloads it across notifier instances',
    () async {
      final first = AppIconPreferenceStore();
      expect(await first.load(), AppIconPreference.voiceSky);

      await first.save(AppIconPreference.mint);

      final second = AppIconPreferenceStore();
      expect(await second.load(), AppIconPreference.mint);
    },
  );

  test('unknown stored identity fails back to Voice Sky', () async {
    SharedPreferences.setMockInitialValues({'voice_app_icon': 'future_icon'});
    final store = AppIconPreferenceStore();

    expect(await store.load(), AppIconPreference.voiceSky);
  });

  test('Plus active and grace allow paid identities; lapse uses Voice Sky', () {
    expect(
      effectiveAppIcon(AppIconPreference.coral, isPlusActiveOrGrace: true),
      AppIconPreference.coral,
    );
    expect(
      effectiveAppIcon(AppIconPreference.coral, isPlusActiveOrGrace: false),
      AppIconPreference.voiceSky,
    );
  });
}
