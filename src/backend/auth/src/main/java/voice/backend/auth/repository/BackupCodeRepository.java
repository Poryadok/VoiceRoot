package voice.backend.auth.repository;

import java.util.List;
import java.util.UUID;

public interface BackupCodeRepository {
  void replaceCodes(UUID accountId, List<String> codeHashes);

  boolean consumeCode(UUID accountId, String codeHash);
}
