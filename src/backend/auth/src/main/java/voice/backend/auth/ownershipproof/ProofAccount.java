package voice.backend.auth.ownershipproof;

import java.util.UUID;

/** Account state read while holding its transaction lock. Never log credential material. */
public record ProofAccount(UUID accountId, String passwordHash, byte[] totpSecret,
    boolean totpEnabled, long sessionEpoch, long securityRevision, boolean active) {
  @Override public String toString() { return "ProofAccount[redacted]"; }
}
