package voice.backend.auth.repository;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

public interface LinkedIdentityRepository {
  void upsertActive(
      UUID accountId,
      UUID profileId,
      String platform,
      String externalId,
      String externalLogin,
      byte[] accessTokenEncrypted,
      byte[] refreshTokenEncrypted);

  List<LinkedIdentity> listActiveByAccount(UUID accountId);

  List<LinkedIdentity> listAllActive();

  Optional<LinkedIdentity> findActive(UUID accountId, String platform);

  void revoke(UUID accountId, String platform);
}
