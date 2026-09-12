package voice.backend.auth.spacedeletionproof;

import java.util.Arrays;
import java.util.UUID;

/** Exact Space persistence acknowledgement for one immutable Auth receipt. */
public record DeletionProofAcknowledgement(
    UUID receiptId, UUID spaceId, UUID operationId, byte[] receiptSha256) {
  public DeletionProofAcknowledgement {
    if (receiptId == null || spaceId == null || operationId == null
        || receiptSha256 == null || receiptSha256.length != 32) {
      throw new IllegalArgumentException("invalid deletion proof acknowledgement");
    }
    receiptSha256 = receiptSha256.clone();
  }

  @Override public byte[] receiptSha256() { return receiptSha256.clone(); }

  @Override public boolean equals(Object other) {
    return other instanceof DeletionProofAcknowledgement value
        && receiptId.equals(value.receiptId) && spaceId.equals(value.spaceId)
        && operationId.equals(value.operationId)
        && Arrays.equals(receiptSha256, value.receiptSha256);
  }

  @Override public int hashCode() {
    return 31 * java.util.Objects.hash(receiptId, spaceId, operationId)
        + Arrays.hashCode(receiptSha256);
  }
}
