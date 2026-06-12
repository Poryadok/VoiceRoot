import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/state/version_update_launcher.dart';

void main() {
  test('empty update url is a no-op', () async {
    await const DefaultVersionUpdateLauncher().launchUpdate(
      updateUrl: '',
      immediate: false,
    );
  });

  test('createVersionUpdateLauncher returns default implementation', () {
    expect(createVersionUpdateLauncher(), isA<DefaultVersionUpdateLauncher>());
  });

  test('test override is used when set', () {
    final mock = _NoopLauncher();
    versionUpdateLauncherTestOverride = mock;
    addTearDown(() => versionUpdateLauncherTestOverride = null);
    expect(createVersionUpdateLauncher(), same(mock));
  });
}

class _NoopLauncher implements VersionUpdateLauncher {
  @override
  Future<void> launchUpdate({
    required String updateUrl,
    required bool immediate,
  }) async {}
}
