package voice.backend.auth.userdb;

import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

/** Used with {@code auth.persistence=memory} (tests). */
public final class InMemoryPrimaryProfileProvisioner implements PrimaryProfileProvisioner {
  private final Map<UUID, UUID> accountToProfile = new ConcurrentHashMap<>();

  @Override
  @SuppressWarnings("unused")
  public String ensurePrimaryProfile(UUID accountId, String displayHint) {
    return accountToProfile
        .computeIfAbsent(accountId, id -> UUID.randomUUID())
        .toString();
  }
}
