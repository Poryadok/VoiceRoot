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
}
