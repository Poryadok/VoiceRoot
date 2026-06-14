package voice.backend.auth.repository;

import java.time.Instant;
import java.util.Arrays;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryAccountRepository implements AccountRepository {
  private final Map<UUID, Account> byId = new ConcurrentHashMap<>();
  private final Map<String, UUID> byEmail = new ConcurrentHashMap<>();
  private final Map<String, UUID> byPhone = new ConcurrentHashMap<>();

  @Override
  public synchronized Account create(String email, String phone, String passwordHash, String type) {
    if (email != null && byEmail.containsKey(email)) {
      throw new IllegalArgumentException("duplicate email");
    }
    if (phone != null && byPhone.containsKey(phone)) {
      throw new IllegalArgumentException("duplicate phone");
    }
    Account account = new Account(UUID.randomUUID(), email, phone, passwordHash, type, "active", null, false, Instant.now());
    byId.put(account.id(), account);
    if (email != null) {
      byEmail.put(email, account.id());
    }
    if (phone != null) {
      byPhone.put(phone, account.id());
    }
    return account;
  }

  @Override
  public Optional<Account> findByEmail(String email) {
    return Optional.ofNullable(email).map(byEmail::get).map(byId::get);
  }

  @Override
  public Optional<Account> findByPhone(String phone) {
    return Optional.ofNullable(phone).map(byPhone::get).map(byId::get);
  }

  @Override
  public Optional<Account> findById(String id) {
    try {
      return Optional.ofNullable(byId.get(UUID.fromString(id)));
    } catch (IllegalArgumentException ex) {
      return Optional.empty();
    }
  }

  @Override
  public synchronized void saveTotpSecret(UUID accountId, byte[] encryptedSecret, boolean enabled) {
    Account existing = byId.get(accountId);
    if (existing == null) {
      return;
    }
    byte[] secretCopy = encryptedSecret == null ? null : Arrays.copyOf(encryptedSecret, encryptedSecret.length);
    byId.put(accountId, new Account(
        existing.id(),
        existing.email(),
        existing.phone(),
        existing.passwordHash(),
        existing.type(),
        existing.status(),
        secretCopy,
        enabled,
        existing.createdAt()));
  }

  @Override
  public synchronized void setTotpEnabled(UUID accountId, boolean enabled) {
    Account existing = byId.get(accountId);
    if (existing == null) {
      return;
    }
    byId.put(accountId, new Account(
        existing.id(),
        existing.email(),
        existing.phone(),
        existing.passwordHash(),
        existing.type(),
        existing.status(),
        existing.totpSecret(),
        enabled,
        existing.createdAt()));
  }

  @Override
  public synchronized void setStatus(UUID accountId, String status) {
    Account existing = byId.get(accountId);
    if (existing == null) {
      return;
    }
    byId.put(accountId, new Account(
        existing.id(),
        existing.email(),
        existing.phone(),
        existing.passwordHash(),
        existing.type(),
        status,
        existing.totpSecret(),
        existing.totpEnabled(),
        existing.createdAt()));
  }
}
