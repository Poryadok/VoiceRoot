package voice.backend.auth.repository;

import java.util.Optional;
import java.util.UUID;

public interface E2EKeyBackupRepository {
  void put(UUID accountId, String encryptedBlob, String passwordHint);

  Optional<E2EKeyBackupRecord> get(UUID accountId);
}
