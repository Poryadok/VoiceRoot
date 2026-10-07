// ignore_for_file: avoid_print, avoid_web_libraries_in_flutter, deprecated_member_use
@TestOn('browser')
library;

import 'dart:convert';
import 'dart:html' as html;
import 'dart:typed_data';

import 'package:flutter/services.dart' show rootBundle;
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/services/app_shell_icon_web.dart' as app_icon;
import 'package:voice_frontend/settings/app_icon_preference.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'replaces and revokes only the current document favicon object URL',
    () async {
      final messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
      expect(
        messenger.checkMockMessageHandler('flutter/assets', null),
        isTrue,
        reason: 'the test must not replace an existing asset handler',
      );
      final requestedAssets = <String>[];
      final iconAssets = <String, Uint8List>{
        'assets/app_icons/voice_sky.png': base64Decode(_voiceSkyPngBase64),
        'assets/app_icons/coral.png': base64Decode(_coralPngBase64),
      };
      messenger.setMockMessageHandler('flutter/assets', (message) async {
        if (message == null) {
          throw StateError('Missing Flutter asset request message');
        }
        final assetPath = utf8.decode(Uint8List.sublistView(message));
        requestedAssets.add(assetPath);
        final bytes = iconAssets[assetPath];
        if (bytes == null) {
          throw StateError('Unexpected Flutter asset request: $assetPath');
        }
        return ByteData.sublistView(bytes);
      });
      addTearDown(() {
        messenger.setMockMessageHandler('flutter/assets', null);
        rootBundle.evict('assets/app_icons/voice_sky.png');
        rootBundle.evict('assets/app_icons/coral.png');
      });

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
      expect(requestedAssets, <String>[
        'assets/app_icons/voice_sky.png',
        'assets/app_icons/coral.png',
      ]);
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

// Exact byte copies of src/frontend/assets/app_icons/voice_sky.png and coral.png.
// SHA-256: voice_sky 0dfc6ec6d7239bb8fcc35544d2ed2e70af2fa74387ba9a877921ece24141f98b;
// coral 779af3a9453b51d4c6ddceed27de11cc36a66c2811ac5c6becfa8b52ccbd5c9e.
const _voiceSkyPngBase64 =
    'iVBORw0KGgoAAAANSUhEUgAAAQAAAAEACAYAAABccqhmAAAE1klEQVR42u3UQRGAMAwAwUrACgKwhj58IAQGBXzgAbedOQNpsmP8'
    '8K3bfkhPNzzHLkHBwUtAcPQSDBy9BAOHL4HA4UsgcPgSCBy+BAKHL+Uh8MFSEAGfKkUh8JFSFAEfKEUR8HFSEAGfJUUh8EFSFAEf'
    'I0UR8CFSFAEfIUUR8AFSGAHDl6IAGLwURcDApSgCBi1FETBgKYyA4UpRAAxWiiJgoFIYAcOUogAYpBRGwBClKAAGKIURMDwpCoDB'
    'SWEEDE2KAmBgUhgBw5KiABiUFEbAkCQASKoBYEBSGAHDkQAgqQaAwUhhBAxFAoAkAEjKAGAgUhgBw5AAIAkAkgAg6f8AGIQURsAQ'
    'JABIAoAkAEgCgCQASAKAJABEmublNnPyVwAILpPl8lcAsEwWy38BwDJZLH8FAAtlqfwXANrLZKn8FwAsFAT8FQDKCwUA/wWA+EJB'
    'wF8BILxQAPBfAIgvFAT8FQAslfwVAKpLBQH/BACLJf8EgOpiQcAfAcByyR8BoLpcEPA/ALBg8j8AsGTyLwCwaPIvALBs8h8AsHAA'
    '8B8AsHT+wT8AwOL5B/8AAMtn/uYPAAto/uYPAEto7uYOAIto7uYOAMto3uYNAAtp3uYNAEtpzuYMAItpzuYMAMtpvo4fABbUfAEA'
    'AEtqrnYaABbVXAUAy2qeAoCFNU8BwNKaIwBkcc0RALK85gcAWWDzA4AssbkBQBbZ3AAgy2xeAACAhTYvAEDAUpsTAABgsc0JABCw'
    '3OYDAABYcPMBAATKS24uAAAAAMwFABCoLbt5AAAAADAPAECgtvTmAAAAAAAAAIBAbfkdPwAAAAAAAAACtSNw/AAQAAAAANWOwfED'
    'QAAAAABUOwrHDwABAAAAUO04HD8ABAAAAEC1I3H8ABAAAAAA1Y7F8QNAAAAAAFQ7GscPAAEAAABQ7XgcPwAEAAAAQLUjcvwAEAAA'
    'AADVjsnxA0AAAAAAVEPA8QNAAAAAAFRDwPEDQAAAAABUQ8DxA0AAAAAAVEPA8QNAAAAAAFRDwPEDQAAAAABUQ8DxA0AAAAAAVEPA'
    '8QPAIAAAAAAIAo4fAAIAAAAgCDh+AAgAAACAIOD4ASAAAAAAgoDjB4AAAAAACAKOHwACAAAAIAg4fgAIAAAAgMoI+DsACAACgGoI'
    '+DMACAACgGoI+CsACAACgGoI+CMACAACgGoI+BsACAACgGoI+BMACAACgGoI+AsACAACgGoI+AMACAACgGoImD0ABAABQDUEzBwA'
    'AoAAoBoCZg0AAUAAUA0BMwaAACAAqIaA2QJAUQTMFACKImCWAFAUATMEgKIImB0AFMPAjAAgCQCSACAJAJIAIAkAkgAgCQCSPgrA'
    '9QxCih4/ACQAGIgEAEkAkNQBAAJS+PgBIAHAYCQASMoBAAEpfPwAkABgQFIVAAhI4eMHgAQAg5KqAEBACh8/AKQ4ABCQwscPACkO'
    'AASk8PEDQIoDAAEpfPwAkOIAQEAKHz8ApDgAEJDCxw8BKX78AJDiAEBACh8/BKT48UNAih8/BKT48QNAigMAASl8/BCQ4scPASl+'
    '/BCQ4scPASl+/CCQ4ocPAcnxQ0CqHz8EpPjxg0CKHz4EJMcPAql++CCQHD4I5PA9EMjheyCQw/dgIEfvwUCO3gOCHLwHBTn2194J'
    'NqD9bgUBRnkAAAAASUVORK5CYII=';

const _coralPngBase64 =
    'iVBORw0KGgoAAAANSUhEUgAAAQAAAAEACAYAAABccqhmAAAE1ElEQVR42u3UQRGAMAwAwUrACgLwW3k4gEEBH3jAbWfOQJrsGD98'
    '+5yH9HTDc+wSFBy8BARHL8HA0UswcPgSCBy+BAKHL4HA4UsgcPhSHgIfLAUR8KlSFAIfKUUR8IFSFAEfJwUR8FlSFAIfJEUR8DFS'
    'FAEfIkUR8BFSFAEfIIURMHwpCoDBS1EEDFyKImDQUhQBA5bCCBiuFAXAYKUoAgYqhREwTCkKgEFKYQQMUYoCYIBSGAHDk6IAGJwU'
    'RsDQpCgABiaFETAsKQqAQUlhBAxJAoCkGgAGJIURMBwJAJJqABiMFEbAUCQASAKApAwABiKFETAMCQCSACAJAJL+D4BBSGEEDEEC'
    'gCQASAKAJABIAoAkAEgCQKRl3W4zJ38FgOAyWS5/BQDLZLH8FwAsk8XyVwCwUJbKfwGgvUyWyn8BwEJBwF8BoLxQAPBfAIgvFAT8'
    'FQDCCwUA/wWA+EJBwF8BwFLJXwGgulQQ8E8AsFjyTwCoLhYE/BEALJf8EQCqywUB/wMACyb/AwBLJv8CAIsm/wIAyyb/AQALBwD/'
    'AQBL5x/8AwAsnn/wDwCwfOZv/gCwgOZv/gCwhOZu7gCwiOZu7gCwjOZt3gCwkOZt3gCwlOZszgCwmOZszgCwnObr+AFgQc0XAACw'
    'pOZqpwFgUc1VALCs5ikAWFjzFAAsrTkCQBbXHAEgy2t+AJAFNj8AyBKbGwBkkc0NALLM5gUAAFho8wIABCy1OQEAABbbnAAAActt'
    'PgAAgAU3HwBAoLzk5gIAAADAXAAAgdqymwcAAAAA8wAABGpLbw4AAAAAAAAACNSW3/EDAAAAAAAAIFA7AscPAAEAAABQ7RgcPwAE'
    'AAAAQLWjcPwAEAAAAADVjsPxA0AAAAAAVDsSxw8AAQAAAFDtWBw/AAQAAABAtaNx/AAQAAAAANWOx/EDQAAAAABUOyLHDwABAAAA'
    'UO2YHD8ABAAAAEA1BBw/AAQAAABANQQcPwAEAAAAQDUEHD8ABAAAAEA1BBw/AAQAAABANQQcPwAEAAAAQDUEHD8ABAAAAEA1BBw/'
    'AAwCAAAAgCDg+AEgAAAAAIKA4weAAAAAAAgCjh8AAgAAACAIOH4ACAAAAIAg4PgBIAAAAACCgOMHgAAAAACojIC/A4AAIACohoA/'
    'A4AAIACohoC/AoAAIACohoA/AoAAIACohoC/AYAAIACohoA/AYAAIACohoC/AIAAIACohoA/AIAAIACohoDZA0AAEABUQ8DMASAA'
    'CACqIWDWABAABADVEDBjAAgAAoBqCJgtABRFwEwBoCgCZgkARREwQwAoioDZAUAxDMwIAJIAIAkAkgAgCQCSACAJAJIAIOmjAFzP'
    'IKTo8QNAAoCBSACQBABJHQAgIIWPHwASAAxGAoCkHAAQkMLHDwAJAAYkVQGAgBQ+fgBIADAoqQoABKTw8QNAigMAASl8/ACQ4gBA'
    'QAofPwCkOAAQkMLHDwApDgAEpPDxA0CKAwABKXz8EJDixw8AKQ4ABKTw8UNAih8/BKT48UNAih8/AKQ4ABCQwscPASl+/BCQ4scP'
    'ASl+/BCQ4scPAil++BCQHD8EpPrxQ0CKHz8IpPjhQ0By/CCQ6ocPAsnhg0AO3wOBHL4HAjl8DwZy9B4M5Og9IMjBe1CQY3/tnY5M'
    'dmqBBeMWAAAAAElFTkSuQmCC';
