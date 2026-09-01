import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/state/subscription_providers.dart';

VoiceProfile _profile(String id, String name, {bool primary = false}) {
  return VoiceProfile(
    id: id,
    accountId: 'a1',
    username: name.toLowerCase(),
    discriminator: '0001',
    displayName: name,
    isPrimary: primary,
  );
}

void main() {
  test('profileDowngradeRequired when free tier has excess profiles', () async {
    final profiles = [
      _profile('p1', 'One', primary: true),
      _profile('p2', 'Two'),
      _profile('p3', 'Three'),
    ];
    final container = ProviderContainer(
      overrides: [
        accountIsPremiumProvider.overrideWithValue(false),
        myProfilesProvider.overrideWith((ref) async => profiles),
      ],
    );
    await container.read(myProfilesProvider.future);
    expect(container.read(profileDowngradeRequiredProvider), isTrue);
    container.dispose();
  });

  test('profileDowngradeRequired false for premium', () async {
    final profiles = [
      _profile('p1', 'One', primary: true),
      _profile('p2', 'Two'),
      _profile('p3', 'Three'),
    ];
    final container = ProviderContainer(
      overrides: [
        accountIsPremiumProvider.overrideWithValue(true),
        myProfilesProvider.overrideWith((ref) async => profiles),
      ],
    );
    await container.read(myProfilesProvider.future);
    expect(container.read(profileDowngradeRequiredProvider), isFalse);
    container.dispose();
  });
}
