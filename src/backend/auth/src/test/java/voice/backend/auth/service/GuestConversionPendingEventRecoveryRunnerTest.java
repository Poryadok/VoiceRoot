package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.lang.reflect.Method;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import org.springframework.scheduling.annotation.Scheduled;
import voice.backend.auth.config.GuestConversionPendingEventRecoveryProperties;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

class GuestConversionPendingEventRecoveryRunnerTest {
  private static final Instant NOW = Instant.parse("2026-09-04T10:15:30Z");
  private static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);

  @Test
  void explicitTickUsesTheConfiguredBoundedBatchAndLease() {
    RecordingOperations operations = new RecordingOperations(List.of());
    GuestConversionPendingEventRecoveryProperties properties = properties();
    properties.setBatchSize(7);
    properties.setLeaseDuration(Duration.ofSeconds(45));

    new GuestConversionPendingEventRecoveryRunner(worker(operations, ignored -> {}), properties).tick();

    assertThat(operations.claims)
        .containsExactly(new ClaimCall(GuestConversionState.PENDING_EVENT, 7, NOW, NOW.plusSeconds(45)));
  }

  @Test
  void tickProcessesPendingEventButNeverPendingUser() {
    GuestConversionOperation pendingEvent = operation(GuestConversionState.PENDING_EVENT);
    GuestConversionOperation pendingUser = operation(GuestConversionState.PENDING_USER);
    RecordingOperations operations = new RecordingOperations(List.of(pendingEvent, pendingUser));
    ArrayList<UUID> publishedAccounts = new ArrayList<>();

    new GuestConversionPendingEventRecoveryRunner(
            worker(operations, envelope -> publishedAccounts.add(UUID.fromString(
                envelope.getUserGuestConverted().getAccountId()))),
            properties())
        .tick();

    assertThat(publishedAccounts).containsExactly(pendingEvent.accountId());
    assertThat(operations.advancedOperationIds).containsExactly(pendingEvent.operationId());
    assertThat(operations.genericLeaseCalls).isZero();
  }

  @Test
  void overlappingTickFailsClosedInsteadOfPublishingTheSameWorkTwice() throws Exception {
    RecordingOperations operations = new RecordingOperations(List.of(operation(GuestConversionState.PENDING_EVENT)));
    BlockingPublisher publisher = new BlockingPublisher();
    GuestConversionPendingEventRecoveryRunner runner =
        new GuestConversionPendingEventRecoveryRunner(worker(operations, publisher), properties());
    ExecutorService executor = Executors.newFixedThreadPool(2);
    try {
      var first = executor.submit(runner::tick);
      assertThat(publisher.entered.await(1, TimeUnit.SECONDS)).isTrue();

      executor.submit(runner::tick).get(1, TimeUnit.SECONDS);

      assertThat(operations.claims).hasSize(1);
      publisher.release.countDown();
      first.get(1, TimeUnit.SECONDS);
    } finally {
      publisher.release.countDown();
      executor.shutdownNow();
    }
  }

  @Test
  void failedTickReleasesTheOverlapGuardSoTheNextTickCanRun() {
    FailOnceOperations operations = new FailOnceOperations();
    GuestConversionPendingEventRecoveryRunner runner =
        new GuestConversionPendingEventRecoveryRunner(worker(operations, ignored -> {}), properties());

    try {
      runner.tick();
    } catch (RuntimeException expected) {
      // Propagation is allowed; the next tick must still be admitted.
    }
    runner.tick();

    assertThat(operations.calls).isEqualTo(2);
  }

  @Test
  void schedulerEntryPointIsBoundToARealScheduledInvocation() throws Exception {
    Method tick = GuestConversionPendingEventRecoveryRunner.class.getMethod("tick");

    Scheduled scheduled = tick.getAnnotation(Scheduled.class);
    assertThat(scheduled).isNotNull();
    assertThat(scheduled.fixedDelayString()).contains("auth.guest-conversion.pending-event.interval");
  }

  private static GuestConversionPendingEventRecoveryProperties properties() {
    GuestConversionPendingEventRecoveryProperties properties =
        new GuestConversionPendingEventRecoveryProperties();
    properties.setBatchSize(3);
    properties.setLeaseDuration(Duration.ofMinutes(1));
    properties.setInterval(Duration.ofSeconds(10));
    return properties;
  }

  private static GuestConversionPendingEventWorker worker(
      RecordingOperations operations, PublishingProbe publishingProbe) {
    return new GuestConversionPendingEventWorker(
        operations,
        (subject, envelope, natsMessageId) -> {
          publishingProbe.published(envelope);
          return new GuestConversionPublishAck("user.events", 1);
        },
        CLOCK);
  }

  private static GuestConversionOperation operation(GuestConversionState state) {
    return new GuestConversionOperation(
        UUID.randomUUID(),
        UUID.randomUUID(),
        UUID.randomUUID(),
        state,
        0,
        NOW,
        NOW.plusSeconds(60),
        null,
        state == GuestConversionState.PENDING_EVENT ? NOW.minusSeconds(1) : null,
        state == GuestConversionState.PENDING_EVENT ? NOW.minusSeconds(1) : null,
        null,
        NOW,
        NOW);
  }

  private static class RecordingOperations implements GuestConversionOperationRepository {
    private final List<GuestConversionOperation> operations;
    private final ArrayList<ClaimCall> claims = new ArrayList<>();
    private final ArrayList<UUID> advancedOperationIds = new ArrayList<>();
    private int genericLeaseCalls;

    private RecordingOperations(List<GuestConversionOperation> operations) {
      this.operations = operations;
    }

    @Override
    public GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now) {
      throw new UnsupportedOperationException();
    }

    @Override
    public List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant leaseUntil) {
      genericLeaseCalls++;
      throw new AssertionError("event runner must use a state-aware claim");
    }

    @Override
    public List<GuestConversionOperation> leaseDue(
        GuestConversionState state, int batchSize, Instant now, Instant leaseUntil) {
      claims.add(new ClaimCall(state, batchSize, now, leaseUntil));
      return operations.stream().filter(operation -> operation.state() == state).toList();
    }

    @Override
    public GuestConversionAdvanceResult advance(
        UUID operationId,
        GuestConversionState expectedState,
        Instant expectedLockedUntil,
        Instant now) {
      advancedOperationIds.add(operationId);
      return GuestConversionAdvanceResult.APPLIED;
    }

    @Override
    public Optional<GuestConversionOperation> recordFailure(
        UUID operationId,
        Instant expectedLockedUntil,
        String errorCode,
        Instant nextAttemptAt,
        Instant now) {
      throw new AssertionError("publisher is expected to acknowledge");
    }
  }

  private static final class FailOnceOperations extends RecordingOperations {
    private int calls;

    private FailOnceOperations() {
      super(List.of());
    }

    @Override
    public List<GuestConversionOperation> leaseDue(
        GuestConversionState state, int batchSize, Instant now, Instant leaseUntil) {
      calls++;
      if (calls == 1) {
        throw new IllegalStateException("claim failure");
      }
      return List.of();
    }
  }

  private static final class BlockingPublisher implements PublishingProbe {
    private final CountDownLatch entered = new CountDownLatch(1);
    private final CountDownLatch release = new CountDownLatch(1);

    @Override
    public void published(UserStreamEvent envelope) {
      entered.countDown();
      try {
        release.await();
      } catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
        throw new IllegalStateException(interrupted);
      }
    }
  }

  @FunctionalInterface
  private interface PublishingProbe {
    void published(UserStreamEvent envelope);
  }

  private record ClaimCall(
      GuestConversionState state, int batchSize, Instant now, Instant leaseUntil) {}
}
