import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/state/voip_push_platform.dart';

void main() {
  test('voipPushServiceForTarget is null on test VM (non-iOS)', () {
    expect(voipPushServiceForTarget(), isNull);
    expect(isVoIPPushSupported, isFalse);
  });
}
