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

  Account convertGuest(UUID accountId, String email, String phone);

  void touchLastOnlineAt(UUID accountId, Instant at);

  int deactivateExpiredGuests(Instant lastOnlineBefore);
}
