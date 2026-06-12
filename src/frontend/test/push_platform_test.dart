import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/state/push_platform.dart';

void main() {
  group('pushPlatformForTarget', () {
    test('returns non-empty platform on test VM', () {
      expect(pushPlatformForTarget(), isNotEmpty);
    });
  });
}
