package voice.backend.auth.service;

import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

/** In-memory subscription tier cache for tests and optional NATS-backed sync. */
public final class InMemorySubscriptionTierStore implements SubscriptionTierResolver {
  private final Map<UUID, String> tiers = new ConcurrentHashMap<>();

  public void setTier(UUID accountId, String tier) {
    if (accountId == null || tier == null || tier.isBlank()) {
      return;
    }
    tiers.put(accountId, tier);
  }

  @Override
  public String resolveTier(UUID accountId) {
    if (accountId == null) {
      return "free";
    }
    return tiers.getOrDefault(accountId, "free");
  }
}
