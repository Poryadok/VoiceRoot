package voice.backend.auth.service;

import java.time.Instant;
import java.util.Objects;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;

/** Keeps the Auth account promotion and its fenced durable transition in one local transaction. */
public final class TransactionalGuestConversionLocalPromotion
    implements GuestConversionLocalPromotion {
  private final TransactionTemplate transactions;
  private final AccountRepository accounts;
  private final GuestConversionOperationRepository operations;

  public TransactionalGuestConversionLocalPromotion(
      TransactionTemplate transactions,
      AccountRepository accounts,
      GuestConversionOperationRepository operations) {
    this.transactions = Objects.requireNonNull(transactions, "transactions");
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

    return Objects.requireNonNull(
        transactions.execute(
            status -> {
              Account account =
                  accounts
                      .findById(operation.accountId().toString())
                      .orElseThrow(() -> new IllegalStateException("conversion account not found"));
              boolean alreadyRegular = "regular".equals(account.type());
              if (!alreadyRegular) {
                if (!"guest".equals(account.type())) {
                  throw new IllegalStateException("conversion account is not guest or regular");
                }
                accounts.markGuestRegular(account.id());
              }

              GuestConversionAdvanceResult result =
                  operations.advance(
                      operation.operationId(),
                      GuestConversionState.PENDING_USER,
                      operation.lockedUntil(),
                      now);
              if (result == GuestConversionAdvanceResult.APPLIED) {
                return result;
              }
              if (result == GuestConversionAdvanceResult.ALREADY_APPLIED && alreadyRegular) {
                return result;
              }
              if (result == GuestConversionAdvanceResult.LEASE_LOST
                  || result == GuestConversionAdvanceResult.NOT_FOUND) {
                status.setRollbackOnly();
                return result;
              }
              throw new IllegalStateException("durable conversion was already applied without local promotion");
            }),
        "transaction result");
  }
}
