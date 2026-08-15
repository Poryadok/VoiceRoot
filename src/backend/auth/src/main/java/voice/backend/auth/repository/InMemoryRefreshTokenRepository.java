package voice.backend.auth.repository;

import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryRefreshTokenRepository implements RefreshTokenRepository {
  private final Map<String, RefreshTokenRecord> byHash = new ConcurrentHashMap<>();
  private final Map<UUID, String> hashById = new ConcurrentHashMap<>();

  @Override
  public synchronized RefreshTokenRecord create(
      UUID accountId, String tokenHash, String deviceInfoJson, String accessJti, Instant expiresAt, Instant now) {
    RefreshTokenRecord record = new RefreshTokenRecord(
        UUID.randomUUID(), accountId, tokenHash, deviceInfoJson == null ? "{}" : deviceInfoJson, accessJti, expiresAt, now, null);
    byHash.put(tokenHash, record);
    hashById.put(record.id(), tokenHash);
    return record;
  }

  @Override
  public Optional<RefreshTokenRecord> findByHash(String tokenHash) {
    return Optional.ofNullable(byHash.get(tokenHash));
  }

  @Override
  public Optional<RefreshTokenRecord> findById(UUID id) {
    String hash = hashById.get(id);
    if (hash == null) {
      return Optional.empty();
    }
    return findByHash(hash);
  }

  @Override
  public synchronized List<RefreshTokenRecord> listActiveByAccount(UUID accountId) {
    Instant now = Instant.now();
    List<RefreshTokenRecord> out = new ArrayList<>();
    for (RefreshTokenRecord record : byHash.values()) {
      if (record.accountId().equals(accountId) && !record.revoked() && record.expiresAt().isAfter(now)) {
        out.add(record);
      }
    }
    out.sort(Comparator.comparing(RefreshTokenRecord::createdAt).reversed());
    return out;
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

  @Override
  public synchronized RefreshTokenRecord revokeById(UUID id, Instant now) {
    String hash = hashById.get(id);
    if (hash == null) {
      return null;
    }
    return revoke(hash, now);
  }

  @Override
  public synchronized void revokeAllForAccount(UUID accountId, Instant now) {
    byHash.replaceAll(
        (hash, record) -> {
          if (!record.accountId().equals(accountId) || record.revoked()) {
            return record;
          }
          return new RefreshTokenRecord(
              record.id(),
              record.accountId(),
              record.tokenHash(),
              record.deviceInfoJson(),
              record.accessJti(),
              record.expiresAt(),
              record.createdAt(),
              now);
        });
  }
}
