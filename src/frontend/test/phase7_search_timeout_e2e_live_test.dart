import 'package:flutter_test/flutter_test.dart';

/// Live E2E: WS `search_timeout` clears searching state.
/// Run with VOICE_RUN_LIVE_COMPOSE=true and MATCHMAKING_LIVE_SHORT_TIMEOUT=true.
void main() {
  test(
    'search timeout live e2e placeholder',
    () {
      // Full browser/WS live harness is covered by gateway compose_matchmaking_timeout_live_test.
    },
    skip: 'Extend with integration_test driver when MM WS harness lands',
  );
}
