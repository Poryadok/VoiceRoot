package voice.backend.auth.repository;

import java.util.Optional;

public interface AccountRepository {
  Account create(String email, String phone, String passwordHash, String type);

  Optional<Account> findByEmail(String email);

  Optional<Account> findByPhone(String phone);

  Optional<Account> findById(String id);
}
