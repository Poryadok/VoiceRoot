package voice.backend.auth.repository;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

public interface OtpCodeRepository {
  OtpCodeRecord create(UUID accountId, String codeHash, String type, Instant expiresAt, Instant now);

  Optional<OtpCodeRecord> findLatestValid(UUID accountId, String type, Instant now);

  void markUsed(UUID id, Instant usedAt);
}
