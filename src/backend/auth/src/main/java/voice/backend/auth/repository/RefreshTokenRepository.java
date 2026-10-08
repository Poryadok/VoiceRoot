package voice.backend.auth.repository;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

public interface RefreshTokenRepository {
  RefreshTokenRecord create(
      UUID accountId,
      UUID profileId,
      String tokenHash,
      String deviceInfoJson,
      String accessJti,
      Instant expiresAt,
      Instant now);

  default RefreshTokenRecord create(
      UUID accountId,
      String tokenHash,
      String deviceInfoJson,
      String accessJti,
      Instant expiresAt,
      Instant now) {
    return create(accountId, null, tokenHash, deviceInfoJson, accessJti, expiresAt, now);
  }

  Optional<RefreshTokenRecord> findByHash(String tokenHash);

  Optional<RefreshTokenRecord> findById(UUID id);

  java.util.List<RefreshTokenRecord> listActiveByAccount(UUID accountId);

  RefreshTokenRecord revoke(String tokenHash, Instant now);

  /** Consumes a token only if it remains unrevoked and unexpired at the supplied instant. */
  /** Production repositories override this with an atomic active/unexpired compare-and-set. */
  default boolean revokeIfActive(String tokenHash, Instant now) {
    RefreshTokenRecord current = findByHash(tokenHash).orElse(null);
    if (current == null || current.revoked() || !current.expiresAt().isAfter(now)) {
      return false;
    }
    revoke(tokenHash, now);
    return true;
  }

  RefreshTokenRecord revokeById(UUID id, Instant now);

  void revokeAllForAccount(UUID accountId, Instant now);
}
