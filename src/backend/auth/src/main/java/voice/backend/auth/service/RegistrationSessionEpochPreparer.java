package voice.backend.auth.service;

import java.util.Objects;
import java.util.UUID;
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
  private final RegistrationIntentBinder intentBinder;

  public RegistrationSessionEpochPreparer(
      TransactionTemplate transactions, AccountRepository accounts, SessionEpochIssuanceGate gate) {
    this(transactions, accounts, gate, null);
  }

  public RegistrationSessionEpochPreparer(TransactionTemplate transactions, AccountRepository accounts,
      SessionEpochIssuanceGate gate, RegistrationIntentBinder intentBinder) {
    this.transactions = Objects.requireNonNull(transactions, "transactions");
    this.accounts = Objects.requireNonNull(accounts, "accounts");
    this.gate = Objects.requireNonNull(gate, "gate");
    this.intentBinder = intentBinder;
  }

  public PreparedRegistration prepare(
      String email, String phone, String passwordHash, String type, boolean regularEmailVerificationPending) {
    return prepare(email, phone, passwordHash, type, regularEmailVerificationPending, null);
  }

  public PreparedRegistration prepare(String email, String phone, String passwordHash, String type,
      boolean regularEmailVerificationPending, UUID registrationIntentId) {
    if (registrationIntentId != null && intentBinder == null) {
      throw new AuthException("auth_unavailable");
    }
    return Objects.requireNonNull(
        transactions.execute(
            ignored -> {
              Account account =
                  regularEmailVerificationPending
                      ? accounts.createRegularEmailPending(email, passwordHash)
                      : accounts.create(email, phone, passwordHash, type);
              PreparedSessionEpoch prepared = gate.prepare(account.id(), account.sessionEpoch());
              if (registrationIntentId != null) intentBinder.bind(registrationIntentId, account.id());
              return new PreparedRegistration(account, prepared);
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
