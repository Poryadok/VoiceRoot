package voice.backend.auth.repository;

import java.time.Instant;
import java.util.UUID;

public record RefreshTokenRecord(
    UUID id,
    UUID accountId,
    UUID profileId,
    String tokenHash,
    String deviceInfoJson,
    String accessJti,
    Instant expiresAt,
    Instant createdAt,
    Instant revokedAt) {
  public RefreshTokenRecord(
      UUID id,
      UUID accountId,
      String tokenHash,
      String deviceInfoJson,
      String accessJti,
      Instant expiresAt,
      Instant createdAt,
      Instant revokedAt) {
    this(id, accountId, null, tokenHash, deviceInfoJson, accessJti, expiresAt, createdAt, revokedAt);
  }

  public boolean revoked() {
    return revokedAt != null;
  }
}
