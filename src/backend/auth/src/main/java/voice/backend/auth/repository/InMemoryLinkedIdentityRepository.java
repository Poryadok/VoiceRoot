package voice.backend.auth.repository;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryLinkedIdentityRepository implements LinkedIdentityRepository {
  private final Map<String, LinkedIdentity> byAccountPlatform = new ConcurrentHashMap<>();
  private final AtomicLong versions = new AtomicLong();

  private static String key(UUID accountId, String platform) {
    return accountId + "|" + platform;
  }

  @Override
  public synchronized void upsertActive(
      UUID accountId,
      UUID profileId,
      String platform,
      String externalId,
      String externalLogin,
      byte[] accessTokenEncrypted,
      byte[] refreshTokenEncrypted) {
    UUID id =
        Optional.ofNullable(byAccountPlatform.get(key(accountId, platform)))
            .map(LinkedIdentity::id)
            .orElseGet(UUID::randomUUID);
    byAccountPlatform.put(
        key(accountId, platform),
        new LinkedIdentity(
            id,
            accountId,
            profileId,
            platform,
            externalId,
            externalLogin,
            accessTokenEncrypted,
            refreshTokenEncrypted,
            "active",
            versions.incrementAndGet()));
  }

  @Override
  public List<LinkedIdentity> listActiveByAccount(UUID accountId) {
    List<LinkedIdentity> out = new ArrayList<>();
    for (LinkedIdentity row : byAccountPlatform.values()) {
      if (row.accountId().equals(accountId) && "active".equals(row.status())) {
        out.add(row);
      }
    }
    out.sort((a, b) -> a.platform().compareTo(b.platform()));
    return out;
  }

  @Override
  public List<LinkedIdentity> listAllActive() {
    List<LinkedIdentity> out = new ArrayList<>();
    for (LinkedIdentity row : byAccountPlatform.values()) {
      if ("active".equals(row.status())) {
        out.add(row);
      }
    }
    return out;
  }

  @Override
  public List<LinkedIdentity> listAllPersonalVerificationProfiles() {
    List<LinkedIdentity> out = new ArrayList<>();
    for (LinkedIdentity row : byAccountPlatform.values()) {
      if ("twitch".equals(row.platform()) || "youtube".equals(row.platform())) {
        out.add(row);
      }
    }
    return out;
  }

  @Override
  public Optional<LinkedIdentity> findActive(UUID accountId, String platform) {
    LinkedIdentity row = byAccountPlatform.get(key(accountId, platform));
    if (row == null || !"active".equals(row.status())) {
      return Optional.empty();
    }
    return Optional.of(row);
  }

  @Override
  public synchronized Optional<LinkedIdentity> revokeIfUnchanged(LinkedIdentity expected) {
    LinkedIdentity row = byAccountPlatform.get(key(expected.accountId(), expected.platform()));
    if (row == null || !"active".equals(row.status()) || row.version() != expected.version()) {
      return Optional.empty();
    }
    byAccountPlatform.put(
        key(expected.accountId(), expected.platform()),
        new LinkedIdentity(
            row.id(),
            row.accountId(),
            row.profileId(),
            row.platform(),
            row.externalId(),
            row.externalLogin(),
            null,
            null,
            "revoked",
            versions.incrementAndGet()));
    return Optional.of(row);
  }
}
