package voice.backend.auth.service;

import java.util.Objects;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.sessionepoch.PreparedSessionEpoch;
import voice.backend.auth.sessionepoch.SessionEpochIssuanceGate;

/** Creates an account and records its session epoch in one short local transaction. */
public final class RegistrationSessionEpochPreparer {
  private final TransactionTemplate transactions;
  private final AccountRepository accounts;
  private final SessionEpochIssuanceGate gate;

  public RegistrationSessionEpochPreparer(
      TransactionTemplate transactions, AccountRepository accounts, SessionEpochIssuanceGate gate) {
    this.transactions = Objects.requireNonNull(transactions, "transactions");
    this.accounts = Objects.requireNonNull(accounts, "accounts");
    this.gate = Objects.requireNonNull(gate, "gate");
  }

  public PreparedRegistration prepare(String email, String phone, String passwordHash, String type) {
    return Objects.requireNonNull(
        transactions.execute(
            ignored -> {
              Account account = accounts.create(email, phone, passwordHash, type);
              return new PreparedRegistration(account, gate.prepare(account.id(), account.sessionEpoch()));
            }),
        "registration transaction result");
  }

  public record PreparedRegistration(Account account, PreparedSessionEpoch preparedEpoch) {
    public PreparedRegistration {
      Objects.requireNonNull(account, "account");
      Objects.requireNonNull(preparedEpoch, "preparedEpoch");
      if (!account.id().equals(preparedEpoch.accountId())) {
        throw new IllegalArgumentException("prepared epoch account ID must match registration account");
      }
    }
  }
}
