// ignore_for_file: avoid_print, avoid_web_libraries_in_flutter, deprecated_member_use
@TestOn('browser')
library;

import 'dart:html' as html;
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/services/app_shell_icon_web.dart' as app_icon;
import 'package:voice_frontend/settings/app_icon_preference.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'replaces and revokes only the current document favicon object URL',
    () async {
      final head = html.document.head!;
      final icon = html.LinkElement()
        ..rel = 'icon'
        ..type = 'image/png'
        ..href = 'favicon.png';
      head.insertBefore(icon, head.firstChild);
      final originalIconLinks = html.document
          .querySelectorAll('link[rel~="icon"]')
          .toList();
      expect(originalIconLinks, isNotEmpty);
      expect(originalIconLinks.first, same(icon));
      final manifest = html.LinkElement()
        ..rel = 'manifest'
        ..href = '/manifest.webmanifest';
      final appleTouchIcon = html.LinkElement()
        ..rel = 'apple-touch-icon'
        ..href = '/apple-touch-icon.png';
      head.append(manifest);
      head.append(appleTouchIcon);
      addTearDown(() {
        icon.remove();
        manifest.remove();
        appleTouchIcon.remove();
      });

      print('APP_ICON_WEB_STAGE apply_voice_sky:start');
      await app_icon.apply(AppIconPreference.voiceSky);
      print('APP_ICON_WEB_STAGE apply_voice_sky:done');
      final firstUrl = icon.href;
      expect(firstUrl, startsWith('blob:'));
      print('APP_ICON_WEB_STAGE fetch_first_png:start');
      final firstBytes = await _loadBytes(firstUrl);
      print('APP_ICON_WEB_STAGE fetch_first_png:done');
      expect(
        firstBytes,
        containsAllInOrder(<int>[137, 80, 78, 71, 13, 10, 26, 10]),
      );
      expect(manifest.href, '/manifest.webmanifest');
      expect(appleTouchIcon.href, '/apple-touch-icon.png');

      print('APP_ICON_WEB_STAGE apply_coral:start');
      await app_icon.apply(AppIconPreference.coral);
      print('APP_ICON_WEB_STAGE apply_coral:done');
      final secondUrl = icon.href;
      expect(secondUrl, startsWith('blob:'));
      expect(secondUrl, isNot(firstUrl));
      print('APP_ICON_WEB_STAGE fetch_second_png:start');
      final secondBytes = await _loadBytes(secondUrl);
      print('APP_ICON_WEB_STAGE fetch_second_png:done');
      expect(secondBytes, containsAllInOrder(<int>[137, 80, 78, 71]));
      print('APP_ICON_WEB_STAGE fetch_revoked_png:start');
      await expectLater(_loadBytes(firstUrl), throwsA(anything));
      print('APP_ICON_WEB_STAGE fetch_revoked_png:done');
      expect(manifest.href, '/manifest.webmanifest');
      expect(appleTouchIcon.href, '/apple-touch-icon.png');
      expect(
        html.document.querySelectorAll('link[rel~="icon"]').length,
        originalIconLinks.length,
      );
    },
  );
}

Future<Uint8List> _loadBytes(String url) async {
  final request = await html.HttpRequest.request(
    url,
    responseType: 'arraybuffer',
  );
  final buffer = request.response as ByteBuffer;
  return buffer.asUint8List();
}
