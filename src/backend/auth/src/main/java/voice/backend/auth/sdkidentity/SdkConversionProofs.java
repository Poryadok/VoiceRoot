package voice.backend.auth.sdkidentity;

import java.time.Clock;
import java.time.Instant;
import java.util.UUID;

/** Purpose-bound conversion proof bytes; recovery proof authorizes status reads only. */
public final class SdkConversionProofs {
  private SdkConversionProofs() {}

  public static String preparePayload(String mode, String token, UUID idempotencyKey, UUID bindingId) {
    mode(mode);
    if (token == null || !token.matches("[A-Za-z0-9_-]{43}") || idempotencyKey == null || bindingId == null) {
      throw new SdkIdentityDeniedException();
    }
    return "voice-sdk-conversion-" + mode + "-v1\n" + SdkIdentityService.hash(token)
        + "\n" + idempotencyKey + "\n" + bindingId;
  }

  public static String requestHash(String mode, UUID idempotencyKey, UUID bindingId,
                                   UUID targetAccountId, UUID targetProfileId) {
    mode(mode);
    if (idempotencyKey == null || bindingId == null
        || ("new".equals(mode) && (targetAccountId != null || targetProfileId != null))
        || ("existing".equals(mode) && (targetAccountId == null || targetProfileId == null))) {
      throw new SdkIdentityDeniedException();
    }
    return SdkIdentityService.hash(mode + "\n" + idempotencyKey + "\n" + bindingId + "\n"
        + (targetAccountId == null ? "-" : targetAccountId) + "\n"
        + (targetProfileId == null ? "-" : targetProfileId));
  }

  public static String statusPayload(UUID operationId, long issuedAt, Clock clock) {
    try {
      if (operationId == null || clock == null) throw new SdkIdentityDeniedException();
      Instant issued = Instant.ofEpochSecond(issuedAt);
      Instant now = clock.instant();
      if (issued.isBefore(now.minusSeconds(60)) || issued.isAfter(now.plusSeconds(30))) {
        throw new SdkIdentityDeniedException();
      }
      return "voice-sdk-conversion-status-v1\n" + operationId + "\n" + issuedAt;
    } catch (java.time.DateTimeException | ArithmeticException invalid) {
      throw new SdkIdentityDeniedException();
    }
  }

  private static void mode(String mode) {
    if (!"new".equals(mode) && !"existing".equals(mode)) throw new SdkIdentityDeniedException();
  }
}
