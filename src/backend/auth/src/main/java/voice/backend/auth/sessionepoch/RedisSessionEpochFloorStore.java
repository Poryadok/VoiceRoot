package voice.backend.auth.sessionepoch;

import java.time.Duration;
import java.util.UUID;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

/** Redis-backed, max-only account session-epoch floors. */
public final class RedisSessionEpochFloorStore implements SessionEpochFloorStore {
  private static final String KEY_PREFIX = "auth:session:min_epoch:";
  private static final Duration MAX_COMMAND_TIMEOUT = Duration.ofSeconds(2);
  private static final Duration WAIT_MARGIN = Duration.ofMillis(20);
  private static final ExecutorService BOUNDED_COMMANDS = Executors.newVirtualThreadPerTaskExecutor();

  private final RedisSessionEpochCommands commands;
  private final Duration commandTimeout;

  RedisSessionEpochFloorStore(RedisSessionEpochCommands commands, Duration commandTimeout) {
    if (commands == null) {
      throw new IllegalArgumentException("redis commands are required");
    }
    if (commandTimeout == null || commandTimeout.isZero() || commandTimeout.isNegative()) {
      throw new IllegalArgumentException("command timeout must be positive");
    }
    if (commandTimeout.compareTo(MAX_COMMAND_TIMEOUT) > 0) {
      throw new IllegalArgumentException("session epoch command timeout must not exceed two seconds");
    }
    this.commands = commands;
    this.commandTimeout = commandTimeout;
  }

  @Override
  public long recordAtLeast(UUID accountId, long epoch) {
    requireAccountId(accountId);
    requirePositiveEpoch(epoch);
    return bounded(
        () -> {
          long recorded = commands.atomicMaxWithoutExpiry(keyFor(accountId), epoch, commandTimeout);
          return requirePositiveFloor(recorded);
        });
  }

  @Override
  public long requireFloor(UUID accountId) {
    requireAccountId(accountId);
    return bounded(() -> requirePositiveFloor(commands.readRequiredPositive(keyFor(accountId), commandTimeout)));
  }

  String keyFor(UUID accountId) {
    requireAccountId(accountId);
    return KEY_PREFIX + accountId;
  }

  private long bounded(Callable<Long> command) {
    Future<Long> future = BOUNDED_COMMANDS.submit(command);
    try {
      long waitNanos = Math.max(1L, commandTimeout.minus(WAIT_MARGIN).toNanos());
      return future.get(waitNanos, TimeUnit.NANOSECONDS);
    } catch (TimeoutException ex) {
      future.cancel(true);
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout", ex);
    } catch (InterruptedException ex) {
      future.cancel(true);
      Thread.currentThread().interrupt();
      throw new SessionEpochFloorUnavailableException("session epoch floor command interrupted", ex);
    } catch (ExecutionException ex) {
      Throwable cause = ex.getCause();
      if (cause instanceof SessionEpochFloorUnavailableException unavailable) {
        throw unavailable;
      }
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable", cause);
    }
  }

  private static void requireAccountId(UUID accountId) {
    if (accountId == null) {
      throw new IllegalArgumentException("account id is required");
    }
  }

  private static void requirePositiveEpoch(long epoch) {
    if (epoch <= 0) {
      throw new IllegalArgumentException("session epoch must be positive");
    }
  }

  private static long requirePositiveFloor(long floor) {
    if (floor <= 0) {
      throw new SessionEpochFloorUnavailableException("invalid session epoch floor");
    }
    return floor;
  }
}
