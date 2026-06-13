package voice.backend.auth.repository;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryBackupCodeRepository implements BackupCodeRepository {
  private final Map<UUID, Map<String, Boolean>> storage = new ConcurrentHashMap<>();

  @Override
  public synchronized void replaceCodes(UUID accountId, List<String> codeHashes) {
    Map<String, Boolean> byHash = new HashMap<>();
    for (String hash : codeHashes) {
      byHash.put(hash, Boolean.FALSE);
    }
    storage.put(accountId, byHash);
  }

  @Override
  public synchronized boolean consumeCode(UUID accountId, String codeHash) {
    Map<String, Boolean> byHash = storage.get(accountId);
    if (byHash == null) {
      return false;
    }
    Boolean used = byHash.get(codeHash);
    if (used == null || used) {
      return false;
    }
    byHash.put(codeHash, Boolean.TRUE);
    return true;
  }
}
