package voice.backend.auth.userdb;

import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

/** Used with {@code auth.persistence=memory} (tests). */
public final class InMemoryPrimaryProfileProvisioner implements PrimaryProfileProvisioner {
  private final Map<UUID, UUID> accountToProfile = new ConcurrentHashMap<>();
  private final java.util.Set<UUID> guestAccounts = ConcurrentHashMap.newKeySet();

  @Override
  @SuppressWarnings("unused")
  public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
    if (guestAccount) {
      guestAccounts.add(accountId);
    }
    return accountToProfile
        .computeIfAbsent(accountId, id -> UUID.randomUUID())
        .toString();
  }

  @Override
  public void clearGuestAccountFlag(UUID accountId) {
    guestAccounts.remove(accountId);
  }

  public boolean isGuestAccount(UUID accountId) {
    return guestAccounts.contains(accountId);
  }
}
