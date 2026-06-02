import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  test('profileAccentColorProvider uses default index from storage', () async {
    await testProfileAccentStorage.writeProfileIndex('prof-b', 1);

    final container = ProviderContainer(
      overrides: [
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
      ],
    );
    addTearDown(container.dispose);

    final color =
        await container.read(profileAccentColorProvider('prof-b').future);
    expect(color, const Color(0xFF9ED9A6));
  });

  test('profileAccentColorProvider uses hex override when set', () async {
    await testProfileAccentStorage.writeOverride('prof-c', '#F0A8A8');

    final container = ProviderContainer(
      overrides: [
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
      ],
    );
    addTearDown(container.dispose);

    final color =
        await container.read(profileAccentColorProvider('prof-c').future);
    expect(color, const Color(0xFFF0A8A8));
  });
}
