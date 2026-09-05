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
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.repository.AccountDeletionOperation;
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
    InMemoryAccountDeletionOperationRepository operations = new InMemoryAccountDeletionOperationRepository();
    UUID accountId = UUID.randomUUID();
    AccountDeletionOperation created =
        operations.createOrResume(UUID.randomUUID(), accountId, 2, "hash", NOW);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(floors.recordAtLeast(accountId, 2)).thenReturn(2L);
    new AccountDeletionPendingFloorWorker(operations, floors, CLOCK).recover(1, Duration.ofSeconds(30));

    AccountDeletionEventPublisher publisher = mock(AccountDeletionEventPublisher.class);
    doThrow(new IllegalStateException("crash after PubAck boundary"))
        .doReturn(new GuestConversionPublishAck("user_events", 1L))
        .when(publisher)
        .publishAccountDeleted(any(), any(), any());
    new AccountDeletionPendingEventWorker(operations, publisher, CLOCK).recover(1, Duration.ofSeconds(30));
    assertThat(operations.findByAccountAndEpoch(accountId, 2).orElseThrow().state())
        .isEqualTo(AccountDeletionState.PENDING_EVENT);

    new AccountDeletionPendingEventWorker(
            operations, publisher, Clock.offset(CLOCK, Duration.ofSeconds(6)))
        .recover(1, Duration.ofSeconds(30));
    assertThat(operations.findByAccountAndEpoch(accountId, 2).orElseThrow().state())
        .isEqualTo(AccountDeletionState.COMPLETED);
    verify(publisher, times(2))
        .publishAccountDeleted(any(), any(), org.mockito.ArgumentMatchers.eq(created.operationId().toString()));
  }
}
