package voice.backend.auth.repository;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

public interface LinkedIdentityRepository {
  LinkedIdentity linkActive(
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

  /** Revokes only the exact active snapshot and returns the row actually revoked. */
  Optional<LinkedIdentity> revokeIfUnchanged(LinkedIdentity expected);

  List<VerificationSourceSyncTarget> listPendingVerificationSyncTargets();

  void markVerificationSyncTargetSynced(VerificationSourceSyncTarget target);
}
