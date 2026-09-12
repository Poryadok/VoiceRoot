package voice.backend.auth.spacedeletionproof;

import java.util.Arrays;
import java.util.UUID;

/** Exact immutable inputs required to recover an already committed receipt. */
public record DeletionProofLookup(
    UUID accountId,
    UUID profileId,
    long sessionEpoch,
    UUID spaceId,
    UUID operationId,
    byte[] confirmationNameSha256,
    byte[] proofDigestSha256) {
  public DeletionProofLookup {
    if (accountId == null || profileId == null || sessionEpoch <= 0 || spaceId == null
        || operationId == null || confirmationNameSha256 == null
        || confirmationNameSha256.length != 32 || proofDigestSha256 == null
        || proofDigestSha256.length != 32) {
      throw new IllegalArgumentException("invalid deletion proof lookup");
    }
    confirmationNameSha256 = confirmationNameSha256.clone();
    proofDigestSha256 = proofDigestSha256.clone();
  }

  @Override public byte[] confirmationNameSha256() { return confirmationNameSha256.clone(); }
  @Override public byte[] proofDigestSha256() { return proofDigestSha256.clone(); }

  @Override public boolean equals(Object other) {
    return other instanceof DeletionProofLookup value
        && accountId.equals(value.accountId) && profileId.equals(value.profileId)
        && sessionEpoch == value.sessionEpoch && spaceId.equals(value.spaceId)
        && operationId.equals(value.operationId)
        && Arrays.equals(confirmationNameSha256, value.confirmationNameSha256)
        && Arrays.equals(proofDigestSha256, value.proofDigestSha256);
  }

  @Override public int hashCode() {
    int result = java.util.Objects.hash(accountId, profileId, sessionEpoch, spaceId, operationId);
    result = 31 * result + Arrays.hashCode(confirmationNameSha256);
    return 31 * result + Arrays.hashCode(proofDigestSha256);
  }
}
