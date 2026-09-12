package voice.backend.auth.spacedeletionproof;

import java.time.Duration;
import java.time.Instant;

/** Rotation and destruction boundaries for the dedicated Auth receipt-index family. */
public final class ReceiptErasureKeyPolicy {
  private final Duration rotation;
  private final Duration maximumRestorableBackupAge;

  public ReceiptErasureKeyPolicy(Duration rotation, Duration maximumRestorableBackupAge) {
    if (rotation == null || rotation.isZero() || rotation.isNegative()
        || maximumRestorableBackupAge == null || maximumRestorableBackupAge.isZero()
        || maximumRestorableBackupAge.isNegative()) {
      throw new IllegalArgumentException("invalid receipt erasure key policy");
    }
    this.rotation = rotation;
    this.maximumRestorableBackupAge = maximumRestorableBackupAge;
  }

  public boolean rotationDue(Instant activatedAt, Instant now) {
    if (activatedAt == null || now == null) throw new IllegalArgumentException("timestamps required");
    return !now.isBefore(activatedAt.plus(rotation));
  }

  public boolean canDestroy(
      int version, Instant now, long dependentRetainedRows, Instant latestRestorableBackupAt) {
    if (version <= 0 || now == null || dependentRetainedRows < 0) {
      throw new IllegalArgumentException("invalid key destruction state");
    }
    if (dependentRetainedRows != 0) return false;
    return latestRestorableBackupAt == null
        || now.isAfter(latestRestorableBackupAt.plus(maximumRestorableBackupAge));
  }
}
