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
  private final Map<String, VerificationSourceSyncTarget> syncTargets = new ConcurrentHashMap<>();
  private final Map<String, Long> syncedRevisions = new ConcurrentHashMap<>();
  private final AtomicLong versions = new AtomicLong();

  private static String key(UUID accountId, String platform) {
    return accountId + "|" + platform;
  }

  @Override
  public synchronized LinkedIdentity linkActive(
      UUID accountId,
      UUID profileId,
      String platform,
      String externalId,
      String externalLogin,
      byte[] accessTokenEncrypted,
      byte[] refreshTokenEncrypted) {
    LinkedIdentity current = byAccountPlatform.get(key(accountId, platform));
    if (current != null
        && "active".equals(current.status())
        && !profileId.equals(current.profileId())) {
      throw new LinkedIdentityProfileConflictException();
    }
    UUID id =
        Optional.ofNullable(current)
            .map(LinkedIdentity::id)
            .orElseGet(UUID::randomUUID);
    long revision = versions.incrementAndGet();
    LinkedIdentity linked =
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
            revision);
    byAccountPlatform.put(
        key(accountId, platform),
        linked);
    syncTargets.put(
        targetKey(accountId, profileId, platform),
        new VerificationSourceSyncTarget(accountId, profileId, platform, revision, true, platform));
    return linked;
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
    long revision = versions.incrementAndGet();
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
            revision));
    syncTargets.put(
        targetKey(row.accountId(), row.profileId(), row.platform()),
        new VerificationSourceSyncTarget(
            row.accountId(), row.profileId(), row.platform(), revision, false, row.platform()));
    return Optional.of(row);
  }

  @Override
  public List<VerificationSourceSyncTarget> listPendingVerificationSyncTargets() {
    List<VerificationSourceSyncTarget> out = new ArrayList<>();
    for (var entry : syncTargets.entrySet()) {
      if (entry.getValue().revision() > syncedRevisions.getOrDefault(entry.getKey(), 0L)) {
        out.add(entry.getValue());
      }
    }
    out.sort((a, b) -> Long.compare(a.revision(), b.revision()));
    return out;
  }

  @Override
  public synchronized void markVerificationSyncTargetSynced(VerificationSourceSyncTarget target) {
    String key = targetKey(target.accountId(), target.profileId(), target.platform());
    VerificationSourceSyncTarget current = syncTargets.get(key);
    if (current != null && current.revision() == target.revision()) {
      syncedRevisions.put(key, target.revision());
    }
  }

  private static String targetKey(UUID accountId, UUID profileId, String platform) {
    return accountId + "|" + profileId + "|" + platform;
  }
}
