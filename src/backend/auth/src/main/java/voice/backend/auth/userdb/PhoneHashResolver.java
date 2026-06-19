package voice.backend.auth.userdb;

import java.util.Collection;
import java.util.Map;

/** Maps stored phone hashes (accounts.phone) to primary profile IDs (user_db.profiles). */
public interface PhoneHashResolver {
  Map<String, String> resolvePrimaryProfileIdsByPhoneHashes(Collection<String> phoneHashes);
}
