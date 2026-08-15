package voice.backend.auth.repository;

import java.time.Instant;
import java.util.UUID;

public record OtpCodeRecord(
    UUID id, UUID accountId, String codeHash, String type, Instant expiresAt, Instant usedAt) {
  public boolean isUsable(Instant now) {
    return usedAt == null && expiresAt.isAfter(now);
  }
}
