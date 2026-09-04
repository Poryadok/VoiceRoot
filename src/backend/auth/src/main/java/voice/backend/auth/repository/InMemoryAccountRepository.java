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
  private final Map<UUID, Instant> guestReminderShownAt = new ConcurrentHashMap<>();

  @Override
  public synchronized Account create(String email, String phone, String passwordHash, String type) {
    if (email != null && byEmail.containsKey(email)) {
      throw new IllegalArgumentException("duplicate email");
    }
    if (phone != null && byPhone.containsKey(phone)) {
      throw new IllegalArgumentException("duplicate phone");
    }
    Account account =
        new Account(
            UUID.randomUUID(),
            email,
            phone,
            passwordHash,
            type,
            "active",
            null,
            false,
            1L,
            Instant.now(),
            null);
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
    byId.put(
        accountId,
        copy(existing, existing.status(), secretCopy, enabled, existing.deletedAt()));
  }

  @Override
  public synchronized void setTotpEnabled(UUID accountId, boolean enabled) {
    Account existing = byId.get(accountId);
    if (existing == null) {
      return;
    }
    byId.put(accountId, copy(existing, existing.status(), existing.totpSecret(), enabled, existing.deletedAt()));
  }

  @Override
  public synchronized void setStatus(UUID accountId, String status) {
    Account existing = byId.get(accountId);
    if (existing == null) {
      return;
    }
    byId.put(accountId, copy(existing, status, existing.totpSecret(), existing.totpEnabled(), existing.deletedAt()));
  }

  @Override
  public synchronized Account convertGuest(UUID accountId, String email, String phone, String passwordHash) {
    Account existing = byId.get(accountId);
    if (existing == null || !"guest".equals(existing.type())) {
      throw new IllegalArgumentException("not a guest account");
    }
    if (email != null && byEmail.containsKey(email) && !byEmail.get(email).equals(accountId)) {
      throw new IllegalArgumentException("duplicate email");
    }
    if (phone != null && byPhone.containsKey(phone) && !byPhone.get(phone).equals(accountId)) {
      throw new IllegalArgumentException("duplicate phone");
    }
    if (existing.email() != null) {
      byEmail.remove(existing.email());
    }
    if (existing.phone() != null) {
      byPhone.remove(existing.phone());
    }
    Account converted =
        new Account(
            existing.id(),
            email,
            phone,
            passwordHash,
            "regular",
            existing.status(),
            existing.totpSecret(),
            existing.totpEnabled(),
            existing.sessionEpoch(),
            existing.createdAt(),
            existing.deletedAt());
    byId.put(accountId, converted);
    if (email != null) {
      byEmail.put(email, accountId);
    }
    if (phone != null) {
      byPhone.put(phone, accountId);
    }
    return converted;
  }

  @Override
  public synchronized void updatePasswordHash(UUID accountId, String passwordHash) {
    Account existing = byId.get(accountId);
    if (existing == null) {
      throw new IllegalArgumentException("account not found");
    }
    byId.put(
        accountId,
        new Account(
            existing.id(),
            existing.email(),
            existing.phone(),
            passwordHash,
            existing.type(),
            existing.status(),
            existing.totpSecret(),
            existing.totpEnabled(),
            existing.sessionEpoch(),
            existing.createdAt(),
            existing.deletedAt()));
  }

  @Override
  public synchronized void touchLastOnlineAt(UUID accountId, Instant at) {
    // In-memory tests do not model last_online_at column.
  }

  @Override
  public synchronized int deactivateExpiredGuests(Instant lastOnlineBefore) {
    return 0;
  }

  @Override
  public synchronized void markDeleted(UUID accountId, Instant deletedAt) {
    Account existing = byId.get(accountId);
    if (existing == null) {
      return;
    }
    byId.put(accountId, copy(existing, "deleted", existing.totpSecret(), existing.totpEnabled(), deletedAt));
  }

  @Override
  public synchronized void restoreDeleted(UUID accountId) {
    Account existing = byId.get(accountId);
    if (existing == null) {
      return;
    }
    byId.put(accountId, copy(existing, "active", existing.totpSecret(), existing.totpEnabled(), null));
  }

  @Override
  public synchronized long incrementSessionEpoch(UUID accountId) {
    Account existing = byId.get(accountId);
    if (existing == null) {
      throw new IllegalArgumentException("account not found");
    }
    long next = Math.addExact(existing.sessionEpoch(), 1L);
    if (next <= 0) {
      throw new IllegalStateException("invalid session epoch");
    }
    byId.put(
        accountId,
        copy(
            existing,
            existing.status(),
            existing.totpSecret(),
            existing.totpEnabled(),
            existing.deletedAt(),
            next));
    return next;
  }

  @Override
  public synchronized Optional<Instant> getGuestReminderLastShownAt(UUID accountId) {
    return Optional.ofNullable(guestReminderShownAt.get(accountId));
  }

  @Override
  public synchronized void markGuestReminderShown(UUID accountId, Instant shownAt) {
    if (byId.containsKey(accountId)) {
      guestReminderShownAt.put(accountId, shownAt);
    }
  }

  @Override
  public synchronized java.util.Set<UUID> findDeletedAmong(java.util.Collection<UUID> accountIds) {
    if (accountIds == null || accountIds.isEmpty()) {
      return java.util.Set.of();
    }
    java.util.Set<UUID> out = new java.util.HashSet<>();
    for (UUID id : accountIds) {
      if (id == null) {
        continue;
      }
      Account account = byId.get(id);
      if (account != null && account.deletedAt() != null) {
        out.add(id);
      }
    }
    return out;
  }

  private static Account copy(
      Account existing, String status, byte[] totpSecret, boolean totpEnabled, Instant deletedAt) {
    return copy(existing, status, totpSecret, totpEnabled, deletedAt, existing.sessionEpoch());
  }

  private static Account copy(
      Account existing,
      String status,
      byte[] totpSecret,
      boolean totpEnabled,
      Instant deletedAt,
      long sessionEpoch) {
    return new Account(
        existing.id(),
        existing.email(),
        existing.phone(),
        existing.passwordHash(),
        existing.type(),
        status,
        totpSecret,
        totpEnabled,
        sessionEpoch,
        existing.createdAt(),
        deletedAt);
  }
}
