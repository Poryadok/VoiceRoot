package voice.backend.auth.service;

import java.util.UUID;

/** Resolves subscription tier for JWT claims (Phase 12). */
public interface SubscriptionTierResolver {
  String resolveTier(UUID accountId);
}
