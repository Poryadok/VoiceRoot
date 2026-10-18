package voice.backend.auth.service;

import java.time.Clock;
import java.time.Duration;
import java.util.ArrayDeque;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/** In-memory OTP throttle exclusively for explicit Auth memory/test mode. */
public class InMemoryOtpThrottle implements OtpThrottle {
  private static final Duration SEND_WINDOW = Duration.ofMinutes(1);
  private static final Duration VERIFY_WINDOW = Duration.ofMinutes(10);
  private static final int MAX_VERIFY_ATTEMPTS = 3;

  private final Map<String, Long> lastSendAt = new ConcurrentHashMap<>();
  private final Map<String, ArrayDeque<Long>> verifyAttempts = new ConcurrentHashMap<>();
  private final Clock clock;

  public InMemoryOtpThrottle() {
    this(Clock.systemUTC());
  }

  public InMemoryOtpThrottle(Clock clock) {
    this.clock = java.util.Objects.requireNonNull(clock, "clock");
  }

  @Override
  public void reserveSend(String key) {
    long now = clock.millis();
    lastSendAt.compute(
        key,
        (ignored, previous) -> {
          if (previous != null && now - previous < SEND_WINDOW.toMillis()) {
            throw new AuthException("otp_rate_limited");
          }
          return now;
        });
  }

  @Override
  public void admitVerify(String key) {
    long now = clock.millis();
    verifyAttempts.compute(
        key,
        (ignored, attempts) -> {
          ArrayDeque<Long> window = attempts == null ? new ArrayDeque<>() : attempts;
          while (!window.isEmpty() && now - window.peekFirst() >= VERIFY_WINDOW.toMillis()) {
            window.removeFirst();
          }
          if (window.size() >= MAX_VERIFY_ATTEMPTS) {
            throw new AuthException("otp_rate_limited");
          }
          window.addLast(now);
          return window;
        });
  }
}
