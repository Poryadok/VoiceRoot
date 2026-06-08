import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('phase1 two users e2e live test is present', () {
    expect(
      File('test/phase1_two_users_e2e_live_test.dart').existsSync(),
      isTrue,
      reason: 'opt-in live E2E entry point for Phase 1',
    );
  });
}
