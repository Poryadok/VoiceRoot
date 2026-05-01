package voice.backend.auth.repository;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

public interface RefreshTokenRepository {
  RefreshTokenRecord create(UUID accountId, String tokenHash, String deviceInfoJson, String accessJti, Instant expiresAt, Instant now);

  Optional<RefreshTokenRecord> findByHash(String tokenHash);

  RefreshTokenRecord revoke(String tokenHash, Instant now);
}
