package voice.backend.auth.repository;

import java.util.List;
import java.util.UUID;

public interface BackupCodeRepository {
  void replaceCodes(UUID accountId, List<String> codeHashes);

  /** Deletes every recovery credential when the account's TOTP enrollment is removed. */
  void deleteCodes(UUID accountId);

  boolean consumeCode(UUID accountId, String codeHash);
}
