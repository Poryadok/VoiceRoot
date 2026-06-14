package voice.backend.auth.repository;

import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryE2EKeyBackupRepository implements E2EKeyBackupRepository {
  private final Map<UUID, E2EKeyBackupRecord> storage = new ConcurrentHashMap<>();

  @Override
  public void put(UUID accountId, String encryptedBlob, String passwordHint) {
    storage.put(accountId, new E2EKeyBackupRecord(encryptedBlob, passwordHint));
  }

  @Override
  public Optional<E2EKeyBackupRecord> get(UUID accountId) {
    return Optional.ofNullable(storage.get(accountId));
  }
}
