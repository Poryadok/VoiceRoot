import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/livekit_room.dart';

void main() {
  test('initial SDK publication honors microphone and camera intent', () {
    for (final muted in [false, true]) {
      for (final video in [false, true]) {
        final options = initialVoiceConnectOptions(
          microphoneMuted: muted,
          video: video,
        );
        expect(options.microphone.enabled, !muted);
        expect(options.camera.enabled, video);
      }
    }
  });
}
