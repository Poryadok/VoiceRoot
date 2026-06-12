import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/theme/voice_layout.dart';

void main() {
  test('narrow breakpoint matches navigation spec', () {
    expect(VoiceLayout.narrowBreakpoint, 600);
    expect(VoiceLayout.isNarrow(599), isTrue);
    expect(VoiceLayout.isNarrow(600), isFalse);
    expect(VoiceLayout.isNarrow(1280), isFalse);
  });
}
