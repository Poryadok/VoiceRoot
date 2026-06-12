import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/client_version.dart';

void main() {
  test('platform returns android on Android target', () {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    addTearDown(() => debugDefaultTargetPlatformOverride = null);
    expect(ClientVersion.platform, 'android');
  });

  test('platform returns ios on iOS target', () {
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    addTearDown(() => debugDefaultTargetPlatformOverride = null);
    expect(ClientVersion.platform, 'ios');
  });

  test('headers include platform and version on mobile', () {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    addTearDown(() => debugDefaultTargetPlatformOverride = null);
    final headers = ClientVersion.headers;
    expect(headers['X-Voice-Client-Platform'], 'android');
    expect(headers['X-Voice-Client-Version'], isNotEmpty);
  });

  test('platform returns windows on Windows target', () {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    addTearDown(() => debugDefaultTargetPlatformOverride = null);
    expect(ClientVersion.platform, 'windows');
  });

  test('headers include platform and version on windows desktop', () {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    addTearDown(() => debugDefaultTargetPlatformOverride = null);
    final headers = ClientVersion.headers;
    expect(headers['X-Voice-Client-Platform'], 'windows');
    expect(headers['X-Voice-Client-Version'], isNotEmpty);
  });

  test('uses desktop auto updater on windows', () {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    addTearDown(() => debugDefaultTargetPlatformOverride = null);
    expect(ClientVersion.usesDesktopAutoUpdater, isTrue);
    expect(ClientVersion.desktopUpdaterChannel, 'voice/desktop_updater');
  });
}
