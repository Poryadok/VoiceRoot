import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/jwt_claims.dart';
import '../backend/subscription_client.dart';
import '../backend/users_client.dart';
import 'auth_providers.dart';
import 'social_providers.dart';

final voiceSubscriptionClientProvider = Provider<VoiceSubscriptionClient>((ref) {
  return VoiceSubscriptionClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final subscriptionProvider = FutureProvider<VoiceSubscription?>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) return null;
  final result = await ref
      .watch(voiceSubscriptionClientProvider)
      .getSubscription(authorization: auth);
  return switch (result) {
    SubscriptionApiOk(:final data) => data,
    SubscriptionApiFailure() => null,
  };
});

/// Effective tier: API subscription plan when loaded, else JWT claim, else `free`.
final subscriptionTierProvider = Provider<String>((ref) {
  final session = ref.watch(authControllerProvider).session;
  if (session == null) return 'free';
  final fromApi = ref.watch(subscriptionProvider).valueOrNull?.plan;
  if (fromApi != null && fromApi.isNotEmpty) return fromApi;
  return subscriptionTierFromAccessToken(session.accessToken) ?? 'free';
});

final accountIsPremiumProvider = Provider<bool>((ref) {
  return ref.watch(subscriptionTierProvider) == 'premium';
});

/// Whether a profile should show the premium ★ badge in chat UI.
final profilePremiumBadgeProvider = Provider.family<bool, String?>((ref, profileId) {
  if (profileId == null || profileId.isEmpty) return false;
  final profile = ref.watch(profileProvider(profileId)).valueOrNull;
  if (profile?.verificationBadge == 'premium') return true;
  final session = ref.watch(authControllerProvider).session;
  if (session == null || profile == null) return false;
  if (profile.accountId != session.accountId) return false;
  return ref.watch(accountIsPremiumProvider);
});

final myProfilesProvider = FutureProvider<List<VoiceProfile>>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) return const [];
  final result = await ref
      .watch(voiceUsersClientProvider)
      .listMyProfiles(authorization: auth);
  return switch (result) {
    UsersApiOk(:final data) => data,
    UsersApiFailure() => const [],
  };
});
