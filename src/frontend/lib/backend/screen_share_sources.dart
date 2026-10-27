import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart' as rtc;

import 'screen_share_capabilities.dart';

enum ScreenShareSourceType { screen, window }

class ScreenShareSource {
  const ScreenShareSource({
    required this.id,
    required this.name,
    required this.type,
  });

  final String id;
  final String name;
  final ScreenShareSourceType type;
}

abstract interface class ScreenShareSourceCatalog {
  Future<List<ScreenShareSource>> listSources();
}

class FlutterWebRtcScreenShareSourceCatalog
    implements ScreenShareSourceCatalog {
  @override
  Future<List<ScreenShareSource>> listSources() async {
    if (!canSelectDesktopScreenShareSource) return const [];
    try {
      final sources = await rtc.desktopCapturer.getSources(
        types: [rtc.SourceType.Screen, rtc.SourceType.Window],
        thumbnailSize: rtc.ThumbnailSize(160, 90),
      );
      return [
        for (final source in sources)
          if (source.id.isNotEmpty && source.name.trim().isNotEmpty)
            ScreenShareSource(
              id: source.id,
              name: source.name,
              type: source.type == rtc.SourceType.Screen
                  ? ScreenShareSourceType.screen
                  : ScreenShareSourceType.window,
            ),
      ];
    } on Object {
      return const [];
    }
  }
}

final screenShareSourceCatalogProvider = Provider<ScreenShareSourceCatalog>(
  (ref) => FlutterWebRtcScreenShareSourceCatalog(),
);
