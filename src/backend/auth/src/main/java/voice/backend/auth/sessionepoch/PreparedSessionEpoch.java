package voice.backend.auth.sessionepoch;

import java.util.UUID;

/** Immutable account epoch prepared by the issuance gate for a later JWT signer. */
public record PreparedSessionEpoch(UUID accountId, long sessionEpoch) {
  public PreparedSessionEpoch {
    if (accountId == null || sessionEpoch <= 0) {
      throw new IllegalArgumentException("account id and positive session epoch are required");
    }
  }
}
