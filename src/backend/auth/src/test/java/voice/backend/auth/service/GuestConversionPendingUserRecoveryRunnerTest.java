package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.lang.reflect.Method;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import org.springframework.scheduling.annotation.Scheduled;
import voice.backend.auth.config.GuestConversionPendingUserRecoveryProperties;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

class GuestConversionPendingUserRecoveryRunnerTest {
  private static final Instant NOW = Instant.parse("2026-09-04T10:15:30Z");
  private static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);

  @Test
  void explicitTickUsesTheConfiguredBoundedBatchAndLease() {
    RecordingOperations operations = new RecordingOperations(List.of());
    GuestConversionPendingUserRecoveryProperties properties = properties();
    properties.setBatchSize(7);
    properties.setLeaseDuration(Duration.ofSeconds(45));

    new GuestConversionPendingUserRecoveryRunner(worker(operations), properties).tick();

    assertThat(operations.leaseCalls)
        .containsExactly(new LeaseCall(7, NOW, NOW.plusSeconds(45)));
  }

  @Test
  void tickProcessesPendingUserButNeverPendingEvent() {
    GuestConversionOperation pendingUser = operation(GuestConversionState.PENDING_USER);
    GuestConversionOperation pendingEvent = operation(GuestConversionState.PENDING_EVENT);
    RecordingOperations operations = new RecordingOperations(List.of(pendingUser, pendingEvent));
    RecordingUser user = new RecordingUser();
    RecordingPromotion promotion = new RecordingPromotion();

    new GuestConversionPendingUserRecoveryRunner(
            new GuestConversionPendingUserWorker(operations, user, promotion, CLOCK), properties())
        .tick();

    assertThat(user.accountIds).containsExactly(pendingUser.accountId());
    assertThat(promotion.operations).containsExactly(pendingUser);
  }

  @Test
  void overlappingTickFailsClosedInsteadOfLeasingTheSameWorkTwice() throws Exception {
    GuestConversionOperation pendingUser = operation(GuestConversionState.PENDING_USER);
    RecordingOperations operations = new RecordingOperations(List.of(pendingUser));
    BlockingUser user = new BlockingUser();
    GuestConversionPendingUserRecoveryRunner runner =
        new GuestConversionPendingUserRecoveryRunner(
            new GuestConversionPendingUserWorker(operations, user, new RecordingPromotion(), CLOCK),
            properties());
    ExecutorService executor = Executors.newFixedThreadPool(2);
    try {
      var first = executor.submit(runner::tick);
      assertThat(user.entered.await(1, TimeUnit.SECONDS)).isTrue();

      var second = executor.submit(runner::tick);

      second.get(1, TimeUnit.SECONDS);
      assertThat(operations.leaseCalls).hasSize(1);
      user.release.countDown();
      first.get(1, TimeUnit.SECONDS);
    } finally {
      user.release.countDown();
      executor.shutdownNow();
    }
  }

  @Test
  void schedulerEntryPointIsBoundToARealScheduledInvocation() throws Exception {
    Method tick = GuestConversionPendingUserRecoveryRunner.class.getMethod("tick");

    Scheduled scheduled = tick.getAnnotation(Scheduled.class);
    assertThat(scheduled).isNotNull();
    assertThat(scheduled.fixedDelayString()).contains("auth.guest-conversion.pending-user.interval");
  }

  private static GuestConversionPendingUserRecoveryProperties properties() {
    GuestConversionPendingUserRecoveryProperties properties =
        new GuestConversionPendingUserRecoveryProperties();
    properties.setBatchSize(3);
    properties.setLeaseDuration(Duration.ofMinutes(1));
    properties.setInterval(Duration.ofSeconds(10));
    return properties;
  }

  private static GuestConversionPendingUserWorker worker(RecordingOperations operations) {
    return new GuestConversionPendingUserWorker(
        operations, new RecordingUser(), new RecordingPromotion(), CLOCK);
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
        null,
        null,
        null,
        NOW,
        NOW);
  }

  private static final class RecordingOperations implements GuestConversionOperationRepository {
    private final List<GuestConversionOperation> leased;
    private final java.util.ArrayList<LeaseCall> leaseCalls = new java.util.ArrayList<>();

    private RecordingOperations(List<GuestConversionOperation> leased) {
      this.leased = leased;
    }

    @Override
    public GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now) {
      throw new UnsupportedOperationException();
    }

    @Override
    public List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant leaseUntil) {
      leaseCalls.add(new LeaseCall(batchSize, now, leaseUntil));
      return leased;
    }

    @Override
    public GuestConversionAdvanceResult advance(
        UUID operationId,
        GuestConversionState expectedState,
        Instant expectedLockedUntil,
        Instant now) {
      throw new UnsupportedOperationException();
    }

    @Override
    public Optional<GuestConversionOperation> recordFailure(
        UUID operationId,
        Instant expectedLockedUntil,
        String errorCode,
        Instant nextAttemptAt,
        Instant now) {
      throw new UnsupportedOperationException();
    }
  }

  private static class RecordingUser implements PrimaryProfileProvisioner {
    private final java.util.ArrayList<UUID> accountIds = new java.util.ArrayList<>();

    @Override
    public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void clearGuestAccountFlag(UUID accountId) {
      accountIds.add(accountId);
    }
  }

  private static final class RecordingPromotion implements GuestConversionLocalPromotion {
    private final java.util.ArrayList<GuestConversionOperation> operations = new java.util.ArrayList<>();

    @Override
    public GuestConversionAdvanceResult promoteAndAdvance(GuestConversionOperation operation, Instant now) {
      operations.add(operation);
      return GuestConversionAdvanceResult.APPLIED;
    }
  }

  private static final class BlockingUser extends RecordingUser {
    private final CountDownLatch entered = new CountDownLatch(1);
    private final CountDownLatch release = new CountDownLatch(1);

    @Override
    public void clearGuestAccountFlag(UUID accountId) {
      super.clearGuestAccountFlag(accountId);
      entered.countDown();
      try {
        release.await();
      } catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
        throw new IllegalStateException(interrupted);
      }
    }
  }

  private record LeaseCall(int batchSize, Instant now, Instant leaseUntil) {}
}
