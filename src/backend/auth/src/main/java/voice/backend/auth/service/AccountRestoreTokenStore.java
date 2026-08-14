package voice.backend.auth.service;

import java.time.Duration;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public interface AccountRestoreTokenStore {
  Duration RESTORE_TTL = Duration.ofDays(30);

  void store(String token, UUID accountId, Duration ttl);

  Optional<UUID> consume(String token);
}
