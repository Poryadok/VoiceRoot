package voice.backend.auth.repository;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

public interface AccountRepository {
  Account create(String email, String phone, String passwordHash, String type);

  Optional<Account> findByEmail(String email);

  Optional<Account> findByPhone(String phone);

  Optional<Account> findById(String id);

  void saveTotpSecret(UUID accountId, byte[] encryptedSecret, boolean enabled);

  void setTotpEnabled(UUID accountId, boolean enabled);

  void setStatus(UUID accountId, String status);

  Account convertGuest(UUID accountId, String email, String phone, String passwordHash);

  void updatePasswordHash(UUID accountId, String passwordHash);

  void touchLastOnlineAt(UUID accountId, Instant at);

  int deactivateExpiredGuests(Instant lastOnlineBefore);

  void markDeleted(UUID accountId, Instant deletedAt);

  void restoreDeleted(UUID accountId);

  /** Atomically advances the account-wide session epoch and returns the new positive value. */
  long incrementSessionEpoch(UUID accountId);

  Optional<Instant> getGuestReminderLastShownAt(UUID accountId);

  void markGuestReminderShown(UUID accountId, Instant shownAt);

  /** Returns account ids among the given set that are soft-deleted (deleted_at IS NOT NULL). */
  java.util.Set<UUID> findDeletedAmong(java.util.Collection<UUID> accountIds);
}
