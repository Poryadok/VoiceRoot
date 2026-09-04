package voice.backend.auth.repository;

import java.time.Instant;
import java.util.List;
import java.util.UUID;

/** Auth-owned durable guest-to-regular conversion operation store. */
public interface GuestConversionOperationRepository {
  GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now);

  List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant leaseUntil);

  /**
   * Advances one currently leased operation from {@code expectedState} to its next durable state.
   *
   * <p>The expected lease is a fencing token: callers that no longer own it receive
   * {@link GuestConversionAdvanceResult#LEASE_LOST}. A lost response after a successful prior
   * transition is reported as {@link GuestConversionAdvanceResult#ALREADY_APPLIED} without
   * rewriting durable markers.
   */
  GuestConversionAdvanceResult advance(
      UUID operationId, GuestConversionState expectedState, Instant expectedLockedUntil, Instant now);
}
