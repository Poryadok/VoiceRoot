package voice.backend.auth.userdb;

import java.util.Collection;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;
import voice.backend.auth.repository.AccountRepository;

/** Used with {@code auth.persistence=memory} (tests). */
public final class InMemoryPhoneHashResolver implements PhoneHashResolver {
  private final AccountRepository accounts;
  private final PrimaryProfileProvisioner profiles;

  public InMemoryPhoneHashResolver(AccountRepository accounts, PrimaryProfileProvisioner profiles) {
    this.accounts = accounts;
    this.profiles = profiles;
  }

  @Override
  public Map<String, String> resolvePrimaryProfileIdsByPhoneHashes(Collection<String> phoneHashes) {
    Map<String, String> out = new HashMap<>();
    if (phoneHashes == null) {
      return out;
    }
    for (String hash : phoneHashes) {
      if (hash == null || hash.isBlank()) {
        continue;
      }
      accounts
          .findByPhone(hash.trim())
          .filter(account -> "active".equals(account.status()))
          .ifPresent(
              account -> {
                String profileId = profiles.ensurePrimaryProfile(account.id(), hash);
                if (profileId != null && !profileId.isBlank()) {
                  out.put(hash.trim(), profileId);
                }
              });
    }
    return out;
  }
}
