package voice.backend.auth.userdb;

import java.util.UUID;

/** Ensures {@code user_db.profiles} has a primary row for the account before JWT is issued. */
public interface PrimaryProfileProvisioner {
  String ensurePrimaryProfile(UUID accountId, String displayHint);
}
