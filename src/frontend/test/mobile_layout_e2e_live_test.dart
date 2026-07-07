import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/theme/voice_layout.dart';

import 'support/live_gateway_harness.dart';

/// Phase-8 mobile layout live check: API stack reachable + narrow breakpoint contract.
///
/// ```text
/// flutter test test/mobile_layout_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'gateway healthy and mobile narrow breakpoint defined',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      expect(VoiceLayout.narrowBreakpoint, 600);
      expect(VoiceLayout.isNarrow(400), isTrue);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
