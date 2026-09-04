package voice.backend.auth.service;

import java.time.Instant;
import java.util.Objects;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;

/** Concrete memory-profile counterpart for the local PENDING_USER promotion boundary. */
public final class InMemoryGuestConversionLocalPromotion implements GuestConversionLocalPromotion {
  private final InMemoryAccountRepository accounts;
  private final GuestConversionOperationRepository operations;

  public InMemoryGuestConversionLocalPromotion(
      InMemoryAccountRepository accounts, GuestConversionOperationRepository operations) {
    this.accounts = Objects.requireNonNull(accounts, "accounts");
    this.operations = Objects.requireNonNull(operations, "operations");
  }

  @Override
  public GuestConversionAdvanceResult promoteAndAdvance(GuestConversionOperation operation, Instant now) {
    Objects.requireNonNull(operation, "operation");
    Objects.requireNonNull(now, "now");
    if (operation.state() != GuestConversionState.PENDING_USER || operation.lockedUntil() == null) {
      throw new IllegalArgumentException("operation is not a leased PENDING_USER conversion");
    }
    Account account = accounts.findById(operation.accountId().toString())
        .orElseThrow(() -> new IllegalStateException("conversion account not found"));
    boolean alreadyRegular = "regular".equals(account.type());
    if (!alreadyRegular && !"guest".equals(account.type())) {
      throw new IllegalStateException("conversion account is not guest or regular");
    }
    boolean promoted = false;
    boolean durableTransitionObserved = false;
    try {
      if (!alreadyRegular) {
        accounts.markGuestRegular(account.id());
        promoted = true;
      }
      GuestConversionAdvanceResult result = operations.advance(
          operation.operationId(), GuestConversionState.PENDING_USER, operation.lockedUntil(), now);
      if (result == GuestConversionAdvanceResult.APPLIED) {
        durableTransitionObserved = true;
        return result;
      }
      if (result == GuestConversionAdvanceResult.ALREADY_APPLIED) {
        durableTransitionObserved = true;
        if (alreadyRegular) {
          return result;
        }
        throw new IllegalStateException("durable conversion was already applied without local promotion");
      }
      if (result == GuestConversionAdvanceResult.LEASE_LOST || result == GuestConversionAdvanceResult.NOT_FOUND) {
        return result;
      }
      throw new IllegalStateException("durable conversion was already applied without local promotion");
    } finally {
      if (promoted && !durableTransitionObserved) {
        accounts.restoreRegularGuest(account.id());
      }
    }
  }
}
