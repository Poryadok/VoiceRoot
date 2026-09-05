package voice.backend.auth.service;

import java.time.Duration;
import java.time.Instant;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryAccountRestoreTokenStore implements AccountRestoreTokenStore {
  private final Map<String, Entry> tokens = new ConcurrentHashMap<>();

  @Override
  public void store(String token, UUID accountId, Duration ttl) {
    tokens.put(AccountRestoreTokenHash.of(token), new Entry(accountId, Instant.now().plus(ttl)));
  }

  @Override
  public Optional<UUID> consume(String token) {
    Entry entry = tokens.remove(AccountRestoreTokenHash.of(token));
    if (entry == null || entry.expiresAt().isBefore(Instant.now())) {
      return Optional.empty();
    }
    return Optional.of(entry.accountId());
  }

  private record Entry(UUID accountId, Instant expiresAt) {}
}
