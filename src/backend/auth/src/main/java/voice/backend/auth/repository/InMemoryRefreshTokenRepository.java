package voice.backend.auth.repository;

import java.time.Instant;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryRefreshTokenRepository implements RefreshTokenRepository {
  private final Map<String, RefreshTokenRecord> byHash = new ConcurrentHashMap<>();

  @Override
  public synchronized RefreshTokenRecord create(
      UUID accountId, String tokenHash, String deviceInfoJson, String accessJti, Instant expiresAt, Instant now) {
    RefreshTokenRecord record = new RefreshTokenRecord(
        UUID.randomUUID(), accountId, tokenHash, deviceInfoJson == null ? "{}" : deviceInfoJson, accessJti, expiresAt, now, null);
    byHash.put(tokenHash, record);
    return record;
  }

  @Override
  public Optional<RefreshTokenRecord> findByHash(String tokenHash) {
    return Optional.ofNullable(byHash.get(tokenHash));
  }

  @Override
  public synchronized RefreshTokenRecord revoke(String tokenHash, Instant now) {
    RefreshTokenRecord current = byHash.get(tokenHash);
    if (current == null) {
      return null;
    }
    if (current.revoked()) {
      return current;
    }
    RefreshTokenRecord revoked = new RefreshTokenRecord(
        current.id(),
        current.accountId(),
        current.tokenHash(),
        current.deviceInfoJson(),
        current.accessJti(),
        current.expiresAt(),
        current.createdAt(),
        now);
    byHash.put(tokenHash, revoked);
    return revoked;
  }
}
