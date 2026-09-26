package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.support.SimpleTransactionStatus;
import org.springframework.transaction.support.TransactionCallback;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.sessionepoch.PreparedSessionEpoch;
import voice.backend.auth.sessionepoch.SessionEpochIssuanceGate;

/** Sequence checks only; PostgreSQL tests own evidence of atomic rollback. */
class RegistrationIntentPreparerTest {
  private final TransactionTemplate transactions = mock(TransactionTemplate.class);
  private final AccountRepository accounts = mock(AccountRepository.class);
  private final SessionEpochIssuanceGate gate = mock(SessionEpochIssuanceGate.class);
  private final RegistrationIntentBinder binder = mock(RegistrationIntentBinder.class);
  private final UUID intent = UUID.randomUUID();
  private Account account;

  @BeforeEach
  void setup() {
    account = new InMemoryAccountRepository().createRegularEmailPending("new@example.test", "hash");
    when(accounts.createRegularEmailPending("new@example.test", "hash")).thenReturn(account);
    when(gate.prepare(account.id(), account.sessionEpoch())).thenReturn(new PreparedSessionEpoch(account.id(), 7));
    when(transactions.execute(any())).thenAnswer(invocation ->
        ((TransactionCallback<?>) invocation.getArgument(0)).doInTransaction(new SimpleTransactionStatus()));
  }

  @Test
  void existingThreeArgumentConstructorAndFiveArgumentPrepareStillWorkWithoutIntent() {
    var preparer = new RegistrationSessionEpochPreparer(transactions, accounts, gate);
    var result = preparer.prepare("new@example.test", null, "hash", "guest", true);
    assertThat(result.account()).isEqualTo(account);
    assertThat(result.preparedEpoch()).isEqualTo(new PreparedSessionEpoch(account.id(), 7));
    verify(gate).prepare(account.id(), account.sessionEpoch());
  }

  @Test
  void bindReceivesNewlyCreatedAccountAfterSuccessfulEpochPreparation() {
    var preparer = new RegistrationSessionEpochPreparer(transactions, accounts, gate, binder);
    var result = preparer.prepare("new@example.test", null, "hash", "guest", true, intent);
    assertThat(result.account().id()).isEqualTo(account.id());
    var order = inOrder(accounts, gate, binder);
    order.verify(accounts).createRegularEmailPending("new@example.test", "hash");
    order.verify(gate).prepare(account.id(), account.sessionEpoch());
    order.verify(binder).bind(intent, account.id());
  }

  @Test
  void missingBinderRejectsIntentBeforeAccountCreation() {
    var preparer = new RegistrationSessionEpochPreparer(transactions, accounts, gate);
    assertThatThrownBy(() -> preparer.prepare("new@example.test", null, "hash", "guest", true, intent))
        .isInstanceOf(AuthException.class).hasMessage("auth_unavailable");
    verify(accounts, never()).createRegularEmailPending(any(), any());
    verifyNoInteractions(gate);
  }

  @Test
  void configuredBinderIsNotCalledForOrdinaryRegistration() {
    var preparer = new RegistrationSessionEpochPreparer(transactions, accounts, gate, binder);
    assertThat(preparer.prepare("new@example.test", null, "hash", "guest", true).account()).isEqualTo(account);
    verifyNoInteractions(binder);
  }

  @Test
  void epochFailureDoesNotConsumeIntent() {
    var failure = new IllegalStateException("epoch preparation unavailable");
    when(gate.prepare(account.id(), account.sessionEpoch())).thenThrow(failure);
    var preparer = new RegistrationSessionEpochPreparer(transactions, accounts, gate, binder);
    assertThatThrownBy(() -> preparer.prepare("new@example.test", null, "hash", "guest", true, intent))
        .isSameAs(failure);
    verifyNoInteractions(binder);
  }

  @Test
  void invalidIntentBinderFailurePropagatesInsteadOfReturningPreparedAccount() {
    doThrow(new AuthException("validation_failed")).when(binder).bind(intent, account.id());
    var preparer = new RegistrationSessionEpochPreparer(transactions, accounts, gate, binder);
    assertThatThrownBy(() -> preparer.prepare("new@example.test", null, "hash", "guest", true, intent))
        .isInstanceOf(AuthException.class).hasMessage("validation_failed");
    verify(binder).bind(intent, account.id());
  }
}
