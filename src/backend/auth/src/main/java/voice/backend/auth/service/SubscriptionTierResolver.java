package voice.backend.auth.service;

import java.util.UUID;

/** Resolves subscription tier for JWT claims (subscription (docs/features/subscription.md)). */
public interface SubscriptionTierResolver {
  String resolveTier(UUID accountId);
}
