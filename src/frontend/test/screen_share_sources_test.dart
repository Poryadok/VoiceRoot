import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/screen_share_sources.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const channel = MethodChannel('FlutterWebRTC.Method');
  final messenger =
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  tearDown(() => messenger.setMockMethodCallHandler(channel, null));

  testWidgets(
    'Windows catalog preserves native screen and window IDs',
    (tester) async {
      final calls = <MethodCall>[];
      messenger.setMockMethodCallHandler(channel, (call) async {
        calls.add(call);
        if (call.method != 'getDesktopSources') return null;
        return {
          'sources': [
            {
              'id': 'screen:0',
              'name': 'Display 1',
              'type': 'screen',
              'thumbnailSize': {'width': 160, 'height': 90},
            },
            {
              'id': 'window:123',
              'name': 'Editor',
              'type': 'window',
              'thumbnailSize': {'width': 160, 'height': 90},
            },
            {
              'id': '',
              'name': 'Invalid',
              'type': 'window',
              'thumbnailSize': {'width': 160, 'height': 90},
            },
            {
              'id': 'window:456',
              'name': '  ',
              'type': 'window',
              'thumbnailSize': {'width': 160, 'height': 90},
            },
          ],
        };
      });
      final sources = await FlutterWebRtcScreenShareSourceCatalog()
          .listSources();
      expect(sources.map((source) => source.id), ['screen:0', 'window:123']);
      expect(sources.map((source) => source.name), ['Display 1', 'Editor']);
      expect(sources.map((source) => source.type), [
        ScreenShareSourceType.screen,
        ScreenShareSourceType.window,
      ]);
      final request = calls.singleWhere(
        (call) => call.method == 'getDesktopSources',
      );
      expect(request.arguments['types'], ['screen', 'window']);
    },
    variant: TargetPlatformVariant.only(TargetPlatform.windows),
  );

  testWidgets(
    'catalog failure yields no fabricated default source',
    (tester) async {
      messenger.setMockMethodCallHandler(channel, (call) async {
        if (call.method == 'getDesktopSources') {
          throw PlatformException(code: 'capture_unavailable');
        }
        return null;
      });
      expect(
        await FlutterWebRtcScreenShareSourceCatalog().listSources(),
        isEmpty,
      );
    },
    variant: TargetPlatformVariant.only(TargetPlatform.windows),
  );

  testWidgets(
    'unsupported platform never invokes desktop source API',
    (tester) async {
      final calls = <MethodCall>[];
      messenger.setMockMethodCallHandler(channel, (call) async {
        calls.add(call);
        return null;
      });
      expect(
        await FlutterWebRtcScreenShareSourceCatalog().listSources(),
        isEmpty,
      );
      expect(calls, isEmpty);
    },
    variant: TargetPlatformVariant.only(TargetPlatform.macOS),
  );
}
