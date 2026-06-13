import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/screen_share_capabilities.dart';

void main() {
  test('canStartScreenShare is true in VM tests (desktop host)', () {
    expect(canStartScreenShare, isTrue);
  });
}
