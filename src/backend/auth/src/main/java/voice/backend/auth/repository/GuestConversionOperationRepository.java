package voice.backend.auth.repository;

import java.time.Instant;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.UUID;

/** Auth-owned durable guest-to-regular conversion operation store. */
public interface GuestConversionOperationRepository {
  GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now);

  List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant leaseUntil);

  /**
   * Claims due work for one durable stage. Implementations must fence the claim
   * to {@code expectedState}; the compatibility default is only for narrow test
   * doubles that expose the older generic method.
   */
  default List<GuestConversionOperation> leaseDue(
      GuestConversionState expectedState, int batchSize, Instant now, Instant leaseUntil) {
    Objects.requireNonNull(expectedState, "expectedState");
    return leaseDue(batchSize, now, leaseUntil).stream()
        .filter(operation -> operation.state() == expectedState)
        .toList();
  }

  /** Finds the one durable conversion operation for an account, if it exists. */
  default Optional<GuestConversionOperation> findByAccountId(UUID accountId) {
    Objects.requireNonNull(accountId, "accountId");
    return Optional.empty();
  }

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

  Optional<GuestConversionOperation> recordFailure(
      UUID operationId,
      Instant expectedLockedUntil,
      String errorCode,
      Instant nextAttemptAt,
      Instant now);
}
