package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionAdvanceResult;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountDeletionState;
import voice.backend.auth.repository.InMemoryAccountDeletionOperationRepository;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

class AccountDeletionPendingWorkersTest {
  private static final Instant NOW = Instant.parse("2026-09-05T12:00:00Z");
  private static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);

  @Test
  void floorFailureLeavesDurablePendingWorkForASecondInstance() {
    InMemoryAccountDeletionOperationRepository operations = new InMemoryAccountDeletionOperationRepository();
    UUID accountId = UUID.randomUUID();
    AccountDeletionOperation operation =
        operations.createOrResume(UUID.randomUUID(), accountId, 2, "hash", NOW);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    doThrow(new IllegalStateException("redis down")).when(floors).recordAtLeast(any(), anyLong());

    new AccountDeletionPendingFloorWorker(operations, floors, CLOCK).recover(1, Duration.ofSeconds(30));
    assertThat(operations.findByAccountAndEpoch(accountId, 2).orElseThrow().state())
        .isEqualTo(AccountDeletionState.PENDING_FLOOR);

    org.mockito.Mockito.doReturn(2L).when(floors).recordAtLeast(accountId, 2);
    new AccountDeletionPendingFloorWorker(
            operations, floors, Clock.offset(CLOCK, Duration.ofSeconds(6)))
        .recover(1, Duration.ofSeconds(30));
    assertThat(operations.findByAccountAndEpoch(accountId, 2).orElseThrow().state())
        .isEqualTo(AccountDeletionState.PENDING_EVENT);
    verify(floors, times(2)).recordAtLeast(accountId, 2);
  }

  @Test
  void eventPublishBeforeFinalizeCanBeReclaimedWithTheSameStableIdentity() {
    InMemoryAccountDeletionOperationRepository persisted = new InMemoryAccountDeletionOperationRepository();
    UUID accountId = UUID.randomUUID();
    AccountDeletionOperation created =
        persisted.createOrResume(UUID.randomUUID(), accountId, 2, "hash", NOW);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(floors.recordAtLeast(accountId, 2)).thenReturn(2L);
    new AccountDeletionPendingFloorWorker(persisted, floors, CLOCK).recover(1, Duration.ofSeconds(30));

    AccountDeletionEventPublisher publisher = mock(AccountDeletionEventPublisher.class);
    when(publisher.publishAccountDeleted(any(), any(), any()))
        .thenReturn(new GuestConversionPublishAck("user_events", 1L));
    AccountDeletionOperationRepository crashAfterPubAck =
        new CrashOnceAfterPubAckRepository(persisted);
    new AccountDeletionPendingEventWorker(crashAfterPubAck, publisher, CLOCK)
        .recover(1, Duration.ofSeconds(30));
    assertThat(persisted.findByAccountAndEpoch(accountId, 2).orElseThrow().state())
        .isEqualTo(AccountDeletionState.PENDING_EVENT);

    new AccountDeletionPendingEventWorker(
            persisted, publisher, Clock.offset(CLOCK, Duration.ofSeconds(6)))
        .recover(1, Duration.ofSeconds(30));
    assertThat(persisted.findByAccountAndEpoch(accountId, 2).orElseThrow().state())
        .isEqualTo(AccountDeletionState.COMPLETED);
    verify(publisher, times(2))
        .publishAccountDeleted(any(), any(), org.mockito.ArgumentMatchers.eq(created.operationId().toString()));
  }

  @Test
  void missingPubAckNeverCompletesTheDeletionOutbox() {
    InMemoryAccountDeletionOperationRepository operations = new InMemoryAccountDeletionOperationRepository();
    UUID accountId = UUID.randomUUID();
    operations.createOrResume(UUID.randomUUID(), accountId, 2, "hash", NOW);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(floors.recordAtLeast(accountId, 2)).thenReturn(2L);
    new AccountDeletionPendingFloorWorker(operations, floors, CLOCK).recover(1, Duration.ofSeconds(30));

    AccountDeletionEventPublisher publisher = mock(AccountDeletionEventPublisher.class);
    new AccountDeletionPendingEventWorker(operations, publisher, CLOCK).recover(1, Duration.ofSeconds(30));

    AccountDeletionOperation pending = operations.findByAccountAndEpoch(accountId, 2).orElseThrow();
    assertThat(pending.state()).isEqualTo(AccountDeletionState.PENDING_EVENT);
    assertThat(pending.attemptCount()).isEqualTo(1);
  }

  /** Simulates process death after a broker PubAck but before the state transition is persisted. */
  private static final class CrashOnceAfterPubAckRepository
      implements AccountDeletionOperationRepository {
    private final AccountDeletionOperationRepository delegate;
    private boolean crash = true;

    private CrashOnceAfterPubAckRepository(AccountDeletionOperationRepository delegate) {
      this.delegate = delegate;
    }

    @Override
    public AccountDeletionOperation createOrResume(
        UUID operationId, UUID accountId, long epoch, String tokenHash, Instant now) {
      return delegate.createOrResume(operationId, accountId, epoch, tokenHash, now);
    }

    @Override
    public Optional<AccountDeletionOperation> findByAccountAndEpoch(UUID accountId, long epoch) {
      return delegate.findByAccountAndEpoch(accountId, epoch);
    }

    @Override
    public List<AccountDeletionOperation> leaseDue(
        AccountDeletionState state, int batchSize, Instant now, Instant leaseUntil) {
      return delegate.leaseDue(state, batchSize, now, leaseUntil);
    }

    @Override
    public Optional<AccountDeletionOperation> lease(
        UUID operationId, AccountDeletionState state, Instant now, Instant leaseUntil) {
      return delegate.lease(operationId, state, now, leaseUntil);
    }

    @Override
    public AccountDeletionAdvanceResult markFloorRecorded(
        UUID operationId, Instant expectedLockedUntil, Instant now) {
      return delegate.markFloorRecorded(operationId, expectedLockedUntil, now);
    }

    @Override
    public AccountDeletionAdvanceResult markEventPublished(
        UUID operationId, Instant expectedLockedUntil, Instant now) {
      if (crash) {
        crash = false;
        throw new IllegalStateException("simulated crash after broker PubAck");
      }
      return delegate.markEventPublished(operationId, expectedLockedUntil, now);
    }

    @Override
    public Optional<AccountDeletionOperation> recordFailure(
        UUID operationId,
        Instant expectedLockedUntil,
        String errorCode,
        Instant nextAttemptAt,
        Instant now) {
      return delegate.recordFailure(operationId, expectedLockedUntil, errorCode, nextAttemptAt, now);
    }
  }
}
