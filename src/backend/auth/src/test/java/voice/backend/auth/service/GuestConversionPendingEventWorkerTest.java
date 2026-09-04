package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

class GuestConversionPendingEventWorkerTest {
  private static final Instant NOW = Instant.parse("2026-09-04T10:15:30Z");
  private static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);
  private static final Duration LEASE = Duration.ofMinutes(1);

  @Test
  void acknowledgedPublishUsesOperationIdentityForTheEnvelopeAndNatsDedupThenFencedCompletes() {
    GuestConversionOperation operation = operation(GuestConversionState.PENDING_EVENT);
    RecordingOperations operations = new RecordingOperations(operation);
    RecordingPublisher publisher = new RecordingPublisher();

    worker(operations, publisher).processDue(3, LEASE);

    assertThat(operations.claimedStates).containsExactly(GuestConversionState.PENDING_EVENT);
    assertThat(publisher.publishes).singleElement().satisfies(publish -> {
      assertThat(publish.subject()).isEqualTo("user.guest_converted");
      assertThat(publish.natsMessageId()).isEqualTo(operation.operationId().toString());
      assertThat(publish.envelope().getEventId()).isEqualTo(operation.operationId().toString());
      assertThat(publish.envelope().getUserGuestConverted().getAccountId())
          .isEqualTo(operation.accountId().toString());
    });
    assertThat(operations.advances)
        .containsExactly(
            new AdvanceCall(
                operation.operationId(),
                GuestConversionState.PENDING_EVENT,
                operation.lockedUntil(),
                NOW));
    assertThat(operations.failures).isEmpty();
  }

  @Test
  void publishFailureOrTimeoutRecordsTheExactLeaseAndKeepsPendingEventForRetry() {
    GuestConversionOperation operation = operation(GuestConversionState.PENDING_EVENT);
    RecordingOperations operations = new RecordingOperations(operation);
    RecordingPublisher publisher = new RecordingPublisher();
    publisher.failure = new IllegalStateException("no puback");

    worker(operations, publisher).processDue(1, LEASE);

    assertThat(operations.advances).isEmpty();
    assertThat(operations.failures)
        .singleElement()
        .satisfies(
            failure -> {
              assertThat(failure.operationId()).isEqualTo(operation.operationId());
              assertThat(failure.lockedUntil()).isEqualTo(operation.lockedUntil());
              assertThat(failure.errorCode()).isEqualTo("IllegalStateException");
              assertThat(failure.nextAttemptAt()).isAfterOrEqualTo(NOW);
              assertThat(failure.now()).isEqualTo(NOW);
            });
    assertThat(operations.state).isEqualTo(GuestConversionState.PENDING_EVENT);
  }

  @Test
  void missingDurablePubAckNeverCompletesTheOperation() {
    GuestConversionOperation operation = operation(GuestConversionState.PENDING_EVENT);
    RecordingOperations operations = new RecordingOperations(operation);
    RecordingPublisher publisher = new RecordingPublisher();
    publisher.ack = null;

    worker(operations, publisher).processDue(1, LEASE);

    assertThat(operations.advances).isEmpty();
    assertThat(operations.failures).singleElement().satisfies(failure -> {
      assertThat(failure.lockedUntil()).isEqualTo(operation.lockedUntil());
      assertThat(failure.errorCode()).isEqualTo("IllegalStateException");
    });
    assertThat(operations.state).isEqualTo(GuestConversionState.PENDING_EVENT);
  }

  @Test
  void crashAfterPubAckBeforeAdvanceRepublishesTheSameStableIdentity() {
    GuestConversionOperation operation = operation(GuestConversionState.PENDING_EVENT);
    RecordingOperations operations = new RecordingOperations(operation);
    operations.advanceFailure = new IllegalStateException("crash after puback");
    RecordingPublisher publisher = new RecordingPublisher();
    MutableClock clock = new MutableClock(NOW);
    GuestConversionPendingEventWorker worker = worker(operations, publisher, clock);

    worker.processDue(1, LEASE);
    operations.advanceFailure = null;
    clock.advance(Duration.ofSeconds(1));
    worker.processDue(1, LEASE);

    assertThat(publisher.publishes)
        .extracting(PublishCall::natsMessageId)
        .containsExactly(operation.operationId().toString(), operation.operationId().toString());
    assertThat(publisher.publishes)
        .extracting(publish -> publish.envelope().getEventId())
        .containsExactly(operation.operationId().toString(), operation.operationId().toString());
    assertThat(operations.advances).hasSize(2);
    assertThat(operations.state).isEqualTo(GuestConversionState.COMPLETED);
  }

  @Test
  void pendingUserIsNotLeasedOrPublishedByTheEventWorker() {
    RecordingOperations operations = new RecordingOperations(operation(GuestConversionState.PENDING_USER));
    RecordingPublisher publisher = new RecordingPublisher();

    worker(operations, publisher).processDue(1, LEASE);

    assertThat(operations.claimedStates).containsExactly(GuestConversionState.PENDING_EVENT);
    assertThat(publisher.publishes).isEmpty();
    assertThat(operations.advances).isEmpty();
  }

  private static GuestConversionPendingEventWorker worker(
      RecordingOperations operations, RecordingPublisher publisher) {
    return worker(operations, publisher, CLOCK);
  }

  private static GuestConversionPendingEventWorker worker(
      RecordingOperations operations, RecordingPublisher publisher, Clock clock) {
    return new GuestConversionPendingEventWorker(
        operations, publisher, clock, (operation, failure, now) -> now.plusSeconds(1));
  }

  private static GuestConversionOperation operation(GuestConversionState state) {
    return new GuestConversionOperation(
        UUID.randomUUID(),
        UUID.randomUUID(),
        UUID.randomUUID(),
        state,
        0,
        NOW,
        NOW.plus(LEASE),
        null,
        state == GuestConversionState.PENDING_EVENT ? NOW.minusSeconds(1) : null,
        state == GuestConversionState.PENDING_EVENT ? NOW.minusSeconds(1) : null,
        null,
        NOW.minusSeconds(1),
        NOW);
  }

  private static final class RecordingOperations implements GuestConversionOperationRepository {
    private GuestConversionState state;
    private final GuestConversionOperation operation;
    private final List<GuestConversionState> claimedStates = new ArrayList<>();
    private final List<AdvanceCall> advances = new ArrayList<>();
    private final List<FailureCall> failures = new ArrayList<>();
    private RuntimeException advanceFailure;
    private Instant nextAttemptAt;

    private RecordingOperations(GuestConversionOperation operation) {
      this.operation = operation;
      state = operation.state();
      nextAttemptAt = operation.nextAttemptAt();
    }

    @Override
    public GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now) {
      throw new UnsupportedOperationException();
    }

    @Override
    public List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant leaseUntil) {
      throw new AssertionError("event worker must use state-aware leasing");
    }

    @Override
    public List<GuestConversionOperation> leaseDue(
        GuestConversionState expectedState, int batchSize, Instant now, Instant leaseUntil) {
      claimedStates.add(expectedState);
      return state == expectedState && !nextAttemptAt.isAfter(now) ? List.of(operation) : List.of();
    }

    @Override
    public GuestConversionAdvanceResult advance(
        UUID operationId,
        GuestConversionState expectedState,
        Instant expectedLockedUntil,
        Instant now) {
      advances.add(new AdvanceCall(operationId, expectedState, expectedLockedUntil, now));
      if (advanceFailure != null) {
        throw advanceFailure;
      }
      state = GuestConversionState.COMPLETED;
      return GuestConversionAdvanceResult.APPLIED;
    }

    @Override
    public Optional<GuestConversionOperation> recordFailure(
        UUID operationId,
        Instant expectedLockedUntil,
        String errorCode,
        Instant nextAttemptAt,
        Instant now) {
      failures.add(new FailureCall(operationId, expectedLockedUntil, errorCode, nextAttemptAt, now));
      this.nextAttemptAt = nextAttemptAt;
      return Optional.of(operation);
    }
  }

  private static final class RecordingPublisher implements GuestConversionEventPublisher {
    private final List<PublishCall> publishes = new ArrayList<>();
    private RuntimeException failure;
    private GuestConversionPublishAck ack = new GuestConversionPublishAck("user.events", 1);

    @Override
    public GuestConversionPublishAck publishGuestConverted(
        String subject, UserStreamEvent envelope, String natsMessageId) {
      publishes.add(new PublishCall(subject, envelope, natsMessageId));
      if (failure != null) {
        throw failure;
      }
      return ack;
    }
  }

  private record PublishCall(String subject, UserStreamEvent envelope, String natsMessageId) {}

  private record AdvanceCall(
      UUID operationId, GuestConversionState state, Instant lockedUntil, Instant now) {}

  private record FailureCall(
      UUID operationId, Instant lockedUntil, String errorCode, Instant nextAttemptAt, Instant now) {}

  private static final class MutableClock extends Clock {
    private Instant instant;

    private MutableClock(Instant instant) {
      this.instant = instant;
    }

    void advance(Duration duration) {
      instant = instant.plus(duration);
    }

    @Override
    public ZoneOffset getZone() {
      return ZoneOffset.UTC;
    }

    @Override
    public Clock withZone(java.time.ZoneId zone) {
      return this;
    }

    @Override
    public Instant instant() {
      return instant;
    }
  }
}
