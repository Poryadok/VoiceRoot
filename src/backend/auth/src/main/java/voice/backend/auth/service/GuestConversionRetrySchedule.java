package voice.backend.auth.service;

import java.time.Instant;
import voice.backend.auth.repository.GuestConversionOperation;

/** Injectable retry scheduling policy for durable guest conversion workers. */
@FunctionalInterface
public interface GuestConversionRetrySchedule {
  Instant nextAttemptAt(GuestConversionOperation operation, RuntimeException failure, Instant now);
}
