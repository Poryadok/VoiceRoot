package voice.backend.auth.events;

import static org.assertj.core.api.Assertions.assertThat;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import org.junit.jupiter.api.Test;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

class AuthObservationDispatcherTest {
  private final AtomicLong ticks = new AtomicLong();
  private final SimpleMeterRegistry meters = new SimpleMeterRegistry();
  private final Clock clock = Clock.fixed(Instant.parse("2026-05-01T10:00:00.123Z"), ZoneOffset.UTC);
  private final UUID id = UUID.randomUUID();

  @Test
  void fourEnvelopesCarryExactSubjectArmAndUnknownIp() throws Exception {
    List<String> subjects = new ArrayList<>();
    List<UserStreamEvent> events = new ArrayList<>();
    try (var dispatcher = dispatcher((subject, bytes, eventId) -> {
      subjects.add(subject);
      var event = UserStreamEvent.parseFrom(bytes);
      assertThat(event.getEventId()).isEqualTo(eventId);
      UUID.fromString(eventId);
      assertThat(event.getOccurredAt().getSeconds()).isEqualTo(clock.instant().getEpochSecond());
      assertThat(event.getOccurredAt().getNanos()).isEqualTo(123000000);
      events.add(event);
      return true;
    })) {
      dispatcher.registered(id, "guest", "email");
      dispatcher.loggedIn(id, "{\"ip\":\"forged\"}");
      dispatcher.loggedOut(id, null);
      dispatcher.twoFaEnabled(id);
      assertThat(subjects).isEmpty();
      while (dispatcher.processReady()) {}
      assertThat(subjects).containsExactlyInAnyOrder("user.registered", "user.logged_in", "user.logged_out", "user.2fa_enabled");
      assertThat(events.stream().filter(UserStreamEvent::hasUserRegistered).findFirst().orElseThrow()
          .getUserRegistered().getType()).isEqualTo("guest");
      assertThat(events.stream().filter(UserStreamEvent::hasUserLoggedIn).findFirst().orElseThrow()
          .getUserLoggedIn().getIp()).isEmpty();
      assertThat(events.stream().filter(UserStreamEvent::hasUserLoggedOut).findFirst().orElseThrow()
          .getUserLoggedOut().getDeviceInfoJson()).isEqualTo("{}");
      assertThat(events.stream().filter(UserStreamEvent::hasUser2FaEnabled).findFirst().orElseThrow()
          .getUser2FaEnabled().getAccountId()).isEqualTo(id.toString());
      assertThat(dispatcher.pending()).isZero();
      assertThat(meters.counter("auth.observations.published").count()).isEqualTo(4);
    }
  }

  @Test
  void retryReusesBytesIdTimeAndStopsAfterPositiveAck() {
    List<byte[]> attempts = new ArrayList<>();
    try (var dispatcher = dispatcher((subject, bytes, eventId) -> {
      attempts.add(bytes.clone());
      bytes[0] = 0; // A transport cannot corrupt the retained retry envelope.
      return attempts.size() == 2;
    })) {
      dispatcher.loggedIn(id, "{}");
      assertThat(dispatcher.processReady()).isTrue();
      assertThat(dispatcher.processReady()).isFalse();
      assertThat(dispatcher.pending()).isEqualTo(1);
      ticks.addAndGet(100_000_000L);
      assertThat(dispatcher.processReady()).isTrue();
      ticks.addAndGet(10_000_000_000L);
      assertThat(dispatcher.processReady()).isFalse();
      assertThat(attempts).hasSize(2);
      assertThat(attempts.get(0)).containsExactly(attempts.get(1));
      assertThat(dispatcher.pending()).isZero();
    }
  }

  @Test
  void attemptsExpiryCapacityOversizeAndShutdownAreBounded() {
    try (var dispatcher = dispatcher((subject, bytes, eventId) -> false)) {
      dispatcher.twoFaEnabled(id);
      dispatcher.processReady();
      ticks.addAndGet(100_000_000L);
      dispatcher.processReady();
      ticks.addAndGet(1_000_000_000L);
      dispatcher.processReady();
      assertThat(dispatcher.pending()).isZero();
      assertDropped("attempts_exhausted", 1);
      dispatcher.twoFaEnabled(id);
      ticks.addAndGet(30_000_000_000L);
      dispatcher.processReady();
      assertDropped("expired", 1);
      dispatcher.loggedIn(id, "x".repeat(16384));
      assertDropped("oversize", 1);
      for (int i = 0; i < 1025; i++) dispatcher.twoFaEnabled(id);
      assertThat(dispatcher.pending()).isEqualTo(1024);
      assertDropped("capacity", 1);
    }
    assertDropped("shutdown", 1024);
    assertThat(meters.find("auth.observations.pending").gauge().value()).isZero();
  }

  @Test
  void totalByteBudgetIncludesQueuedRetries() {
    try (var dispatcher = dispatcher((subject, bytes, eventId) -> false)) {
      for (int i = 0; i < 600; i++) dispatcher.loggedIn(id, "x".repeat(16000));
      assertThat(dispatcher.pending()).isBetween(500, 524);
      assertThat(meters.counter("auth.observations.dropped", "reason", "capacity").count()).isPositive();
    }
  }

  @Test
  void publisherRunsOnDedicatedThreadAndInflightCountsAgainstCapacity() throws Exception {
    Thread requester = Thread.currentThread();
    CountDownLatch entered = new CountDownLatch(1);
    CountDownLatch finish = new CountDownLatch(1);
    try (var dispatcher = new AuthObservationDispatcher((subject, bytes, eventId) -> {
      assertThat(Thread.currentThread()).isNotEqualTo(requester);
      entered.countDown();
      return finish.await(2, TimeUnit.SECONDS);
    }, clock, meters)) {
      dispatcher.twoFaEnabled(id);
      assertThat(entered.await(2, TimeUnit.SECONDS)).isTrue();
      for (int i = 0; i < 1024; i++) dispatcher.twoFaEnabled(id);
      assertThat(dispatcher.pending()).isEqualTo(1024);
      assertDropped("capacity", 1);
      finish.countDown();
    } finally { finish.countDown(); }
  }

  @Test
  void unavailableAndSerializationAreObservableAndNeverThrow() {
    try (var dispatcher = dispatcher(null)) {
      dispatcher.twoFaEnabled(id);
      dispatcher.twoFaEnabled(null);
      assertDropped("unavailable", 1);
      assertDropped("serialization", 1);
      assertThat(dispatcher.pending()).isZero();
    }
  }

  private AuthObservationDispatcher dispatcher(AuthObservationDispatcher.Publisher publisher) {
    return new AuthObservationDispatcher(publisher, clock, ticks::get, meters, false);
  }
  private void assertDropped(String reason, int expected) {
    assertThat(meters.counter("auth.observations.dropped", "reason", reason).count()).isEqualTo(expected);
  }
}
