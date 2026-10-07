// ignore_for_file: avoid_web_libraries_in_flutter, deprecated_member_use
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
      final originalIconLinks = html.document
          .querySelectorAll('link[rel~="icon"]')
          .toList();
      expect(originalIconLinks, isNotEmpty);
      final icon = originalIconLinks.first as html.LinkElement;
      final originalIconHref = icon.href;
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
        icon.href = originalIconHref;
        manifest.remove();
        appleTouchIcon.remove();
        for (final link in originalIconLinks) {
          if (link.parent == null) head.append(link);
        }
      });

      await app_icon.apply(AppIconPreference.voiceSky);
      final firstUrl = icon.href;
      expect(firstUrl, startsWith('blob:'));
      final firstBytes = await _loadBytes(firstUrl);
      expect(
        firstBytes,
        containsAllInOrder(<int>[137, 80, 78, 71, 13, 10, 26, 10]),
      );
      expect(manifest.href, '/manifest.webmanifest');
      expect(appleTouchIcon.href, '/apple-touch-icon.png');

      await app_icon.apply(AppIconPreference.coral);
      final secondUrl = icon.href;
      expect(secondUrl, startsWith('blob:'));
      expect(secondUrl, isNot(firstUrl));
      expect(
        await _loadBytes(secondUrl),
        containsAllInOrder(<int>[137, 80, 78, 71]),
      );
      await expectLater(_loadBytes(firstUrl), throwsA(anything));
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
