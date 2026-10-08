import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/services/app_shell_icon.dart';
import 'package:voice_frontend/settings/app_icon_preference.dart';

void main() {
  test('non-Web shell does not claim browser-tab icon support', () async {
    expect(supportsWebAppShellIcon, isFalse);
    await expectLater(
      applyWebAppShellIcon(AppIconPreference.coral),
      throwsA(isA<UnsupportedError>()),
    );
  });
}
