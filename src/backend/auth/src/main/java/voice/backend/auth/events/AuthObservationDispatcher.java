package voice.backend.auth.events;

import com.google.protobuf.Timestamp;
import io.micrometer.core.instrument.MeterRegistry;
import java.time.Clock;
import java.util.Comparator;
import java.util.PriorityQueue;
import java.util.UUID;
import java.util.function.LongSupplier;
import voice.events.v1.JetstreamEvents.*;

/** Bounded, deliberately lossy observations. All network work belongs to one dedicated worker. */
public final class AuthObservationDispatcher implements AuthObservations, AutoCloseable {
  @FunctionalInterface
  public interface Publisher {
    boolean publish(String subject, byte[] bytes, String eventId) throws Exception;
  }

  private static final int MAX_COUNT = 1024;
  private static final int MAX_BYTES = 8 * 1024 * 1024;
  private static final int MAX_EVENT_BYTES = 16 * 1024;
  private static final long LIFETIME_NANOS = 30_000_000_000L;
  private final Publisher publisher;
  private final Clock clock;
  private final LongSupplier ticks;
  private final MeterRegistry meters;
  private final PriorityQueue<Entry> queue = new PriorityQueue<>(Comparator.comparingLong(e -> e.ready));
  private final Thread worker;
  private int pending;
  private long pendingBytes;
  private boolean accepting = true;
  private boolean stopped;

  public AuthObservationDispatcher(Publisher publisher, Clock clock, MeterRegistry meters) {
    this(publisher, clock, System::nanoTime, meters, true);
  }

  AuthObservationDispatcher(Publisher publisher, Clock clock, LongSupplier ticks,
      MeterRegistry meters, boolean startWorker) {
    this.publisher = publisher;
    this.clock = java.util.Objects.requireNonNull(clock);
    this.ticks = java.util.Objects.requireNonNull(ticks);
    this.meters = java.util.Objects.requireNonNull(meters);
    meters.gauge("auth.observations.pending", this, value -> value.pending());
    worker = startWorker ? new Thread(this::run, "auth-observations") : null;
    if (worker != null) {
      worker.setDaemon(true);
      worker.start();
    }
  }

  @Override
  public void registered(UUID id, String type, String method) {
    observe("user.registered", () -> UserStreamEvent.newBuilder().setUserRegistered(
        UserRegistered.newBuilder().setAccountId(id.toString()).setType(type).setMethod(method)));
  }

  @Override
  public void loggedIn(UUID id, String device) {
    observe("user.logged_in", () -> UserStreamEvent.newBuilder().setUserLoggedIn(
        UserLoggedIn.newBuilder().setAccountId(id.toString()).setDeviceInfoJson(device(device)).setIp("")));
  }

  @Override
  public void loggedOut(UUID id, String device) {
    observe("user.logged_out", () -> UserStreamEvent.newBuilder().setUserLoggedOut(
        UserLoggedOut.newBuilder().setAccountId(id.toString()).setDeviceInfoJson(device(device))));
  }

  @Override
  public void twoFaEnabled(UUID id) {
    observe("user.2fa_enabled", () -> UserStreamEvent.newBuilder().setUser2FaEnabled(
        UserTwoFaEnabled.newBuilder().setAccountId(id.toString())));
  }

  private static String device(String value) {
    return value == null || value.isBlank() ? "{}" : value;
  }

  private void observe(String subject, java.util.function.Supplier<UserStreamEvent.Builder> build) {
    byte[] bytes;
    String id;
    try {
      var now = clock.instant();
      id = UUID.randomUUID().toString();
      bytes = build.get().setEventId(id).setOccurredAt(Timestamp.newBuilder()
          .setSeconds(now.getEpochSecond()).setNanos(now.getNano())).build().toByteArray();
    } catch (RuntimeException failure) {
      drop("serialization");
      return;
    }
    synchronized (this) {
      if (!accepting) { drop("shutdown"); return; }
      if (publisher == null) { drop("unavailable"); return; }
      if (bytes.length > MAX_EVENT_BYTES) { drop("oversize"); return; }
      if (pending >= MAX_COUNT || pendingBytes + bytes.length > MAX_BYTES) { drop("capacity"); return; }
      long now = ticks.getAsLong();
      queue.add(new Entry(subject, bytes, id, now));
      pending++;
      pendingBytes += bytes.length;
      count("accepted");
      notifyAll();
    }
  }

  synchronized int pending() { return pending; }

  /** One due attempt; a deterministic clock seam for bounds/retry tests. Single consumer only. */
  boolean processReady() {
    Entry entry;
    synchronized (this) {
      if (stopped || queue.isEmpty() || queue.peek().ready > ticks.getAsLong()) return false;
      entry = queue.remove();
      if (ticks.getAsLong() - entry.created >= LIFETIME_NANOS) {
        finish(entry, "expired");
        return true;
      }
    }
    boolean confirmed = false;
    try {
      confirmed = publisher.publish(entry.subject, entry.bytes.clone(), entry.id);
    } catch (Exception failure) {
      // Never log the exception or payload: transport messages may contain sensitive data.
    }
    synchronized (this) {
      entry.attempts++;
      if (confirmed) { count("published"); finish(entry, null); }
      else if (stopped) finish(entry, "shutdown");
      else if (ticks.getAsLong() - entry.created >= LIFETIME_NANOS) finish(entry, "expired");
      else if (entry.attempts >= 3) finish(entry, "attempts_exhausted");
      else {
        entry.ready = ticks.getAsLong() + (entry.attempts == 1 ? 100_000_000L : 1_000_000_000L);
        queue.add(entry);
        count("retried");
      }
      notifyAll();
    }
    return true;
  }

  private void run() {
    while (true) {
      synchronized (this) {
        if (stopped || (!accepting && pending == 0)) return;
      }
      if (processReady()) continue;
      synchronized (this) {
        if (stopped) return;
        try { wait(25); } catch (InterruptedException interrupted) { if (stopped) return; }
      }
    }
  }

  private void count(String result) { meters.counter("auth.observations." + result).increment(); }
  private void drop(String reason) { meters.counter("auth.observations.dropped", "reason", reason).increment(); }
  private void finish(Entry entry, String reason) {
    pending--;
    pendingBytes -= entry.bytes.length;
    if (reason != null) drop(reason);
  }

  @Override
  public void close() {
    synchronized (this) { accepting = false; notifyAll(); }
    if (worker != null) {
      try { worker.join(2000); } catch (InterruptedException interrupted) { Thread.currentThread().interrupt(); }
    }
    synchronized (this) {
      stopped = true;
      while (!queue.isEmpty()) finish(queue.remove(), "shutdown");
      notifyAll();
    }
    if (worker != null) worker.interrupt();
  }

  private static final class Entry {
    final String subject;
    final byte[] bytes;
    final String id;
    final long created;
    long ready;
    int attempts;
    Entry(String subject, byte[] bytes, String id, long created) {
      this.subject = subject; this.bytes = bytes; this.id = id;
      this.created = created; this.ready = created;
    }
  }
}
