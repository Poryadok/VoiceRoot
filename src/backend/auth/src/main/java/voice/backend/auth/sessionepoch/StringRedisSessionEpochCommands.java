package voice.backend.auth.sessionepoch;

import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicReference;
import io.lettuce.core.AbstractRedisClient;
import io.lettuce.core.ConnectionFuture;
import io.lettuce.core.RedisClient;
import io.lettuce.core.RedisFuture;
import io.lettuce.core.RedisURI;
import io.lettuce.core.TransactionResult;
import io.lettuce.core.api.StatefulRedisConnection;
import io.lettuce.core.api.async.RedisAsyncCommands;
import io.lettuce.core.codec.ByteArrayCodec;
import org.springframework.data.redis.connection.RedisConnection;
import org.springframework.data.redis.connection.RedisConnectionFactory;
import org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory;
import org.springframework.data.redis.connection.lettuce.LettuceClientConfiguration;
import org.springframework.data.redis.connection.RedisStandaloneConfiguration;
import org.springframework.data.redis.core.StringRedisTemplate;

/**
 * String-only Redis CAS implementation. It deliberately avoids Lua arithmetic because Redis Lua
 * numbers cannot exactly represent every positive signed 64-bit epoch.
 */
final class StringRedisSessionEpochCommands implements RedisSessionEpochCommands {
  private final RedisClient redisClient;
  private final RedisUriFactory redisUriFactory;

  StringRedisSessionEpochCommands(StringRedisTemplate redis) {
    if (redis == null) {
      throw new IllegalArgumentException("redis template is required");
    }
    LettuceConnectionFactory connectionFactory = standaloneFactory(redis.getRequiredConnectionFactory());
    this.redisClient = standaloneClient(connectionFactory);
    this.redisUriFactory = timeout -> redisUri(connectionFactory, timeout);
  }

  StringRedisSessionEpochCommands(
      StringRedisTemplate redis, RedisClient redisClient, RedisURI redisUri) {
    if (redis == null) {
      throw new IllegalArgumentException("redis template is required");
    }
    if (redisClient == null) {
      throw new IllegalArgumentException("Redis client is required");
    }
    if (redisUri == null) {
      throw new IllegalArgumentException("Redis URI is required");
    }
    this.redisClient = redisClient;
    this.redisUriFactory = ignored -> redisUri;
  }

  @Override
  public long atomicMaxWithoutExpiry(String key, long candidate, Duration timeout) {
    long deadlineNanos = System.nanoTime() + timeout.toNanos();
    StatefulRedisConnection<byte[], byte[]> connection = connect(deadlineNanos);
    try {
      connection.setTimeout(timeout);
      RedisAsyncCommands<byte[], byte[]> commands = connection.async();
      byte[] encodedKey = key.getBytes(StandardCharsets.UTF_8);
      while (System.nanoTime() < deadlineNanos && !Thread.currentThread().isInterrupted()) {
        await(commands.watch(encodedKey), deadlineNanos, connection::closeAsync);
        boolean watched = true;
        boolean queued = false;
        try {
          byte[] currentRaw = await(commands.get(encodedKey), deadlineNanos, connection::closeAsync);
          long current = currentRaw == null ? 0L : parsePositive(new String(currentRaw, StandardCharsets.UTF_8));
          long next = Math.max(current, candidate);
          await(commands.multi(), deadlineNanos, connection::closeAsync);
          queued = true;
          await(
              commands.set(encodedKey, Long.toString(next).getBytes(StandardCharsets.UTF_8)),
              deadlineNanos,
              connection::closeAsync);
          TransactionResult result = await(commands.exec(), deadlineNanos, connection::closeAsync);
          queued = false;
          watched = false;
          if (!result.wasDiscarded() && !result.isEmpty()) {
            return next;
          }
        } finally {
          if (connection.isOpen()) {
            if (queued) {
              cancel(commands.discard());
            } else if (watched) {
              cancel(commands.unwatch());
            }
          }
        }
      }
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout");
    } finally {
      if (connection.isOpen()) {
        connection.closeAsync();
      }
    }
  }

  @Override
  public long readRequiredPositive(String key, Duration timeout) {
    long deadlineNanos = System.nanoTime() + timeout.toNanos();
    StatefulRedisConnection<byte[], byte[]> connection = connect(deadlineNanos);
    try {
      connection.setTimeout(timeout);
      RedisAsyncCommands<byte[], byte[]> commands = connection.async();
      byte[] raw =
          await(commands.get(key.getBytes(StandardCharsets.UTF_8)), deadlineNanos, connection::closeAsync);
      if (raw == null) {
        throw new SessionEpochFloorUnavailableException("session epoch floor missing");
      }
      return parsePositive(new String(raw, StandardCharsets.UTF_8));
    } finally {
      if (connection.isOpen()) {
        connection.closeAsync();
      }
    }
  }

  private StatefulRedisConnection<byte[], byte[]> connect(long deadlineNanos) {
    long remainingNanos = deadlineNanos - System.nanoTime();
    if (remainingNanos <= 0) {
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout");
    }
    ConnectionFuture<StatefulRedisConnection<byte[], byte[]>> future =
        redisClient.connectAsync(ByteArrayCodec.INSTANCE, redisUriFactory.create(Duration.ofNanos(remainingNanos)));
    AtomicBoolean abandoned = new AtomicBoolean();
    AtomicBoolean closeScheduled = new AtomicBoolean();
    AtomicReference<StatefulRedisConnection<byte[], byte[]>> connected = new AtomicReference<>();
    future.whenComplete(
        (connection, error) -> {
          if (connection != null) {
            connected.set(connection);
            if (abandoned.get()) {
              closeOnce(connection, closeScheduled);
            }
          }
        });
    try {
      return awaitConnection(
          future,
          deadlineNanos,
          () -> {
            abandoned.set(true);
            StatefulRedisConnection<byte[], byte[]> lateConnection = connected.get();
            if (lateConnection != null) {
              closeOnce(lateConnection, closeScheduled);
            }
          });
    } finally {
      if (!future.isDone()) {
        abandoned.set(true);
        StatefulRedisConnection<byte[], byte[]> lateConnection = connected.get();
        if (lateConnection != null) {
          closeOnce(lateConnection, closeScheduled);
        }
      }
    }
  }

  private static <T> T awaitConnection(
      Future<T> future, long deadlineNanos, Runnable timeoutAction) {
    long remainingNanos = deadlineNanos - System.nanoTime();
    if (remainingNanos <= 0) {
      timeoutAction.run();
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout");
    }
    try {
      return future.get(remainingNanos, TimeUnit.NANOSECONDS);
    } catch (TimeoutException ex) {
      timeoutAction.run();
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout", ex);
    } catch (InterruptedException ex) {
      timeoutAction.run();
      Thread.currentThread().interrupt();
      throw new SessionEpochFloorUnavailableException("session epoch floor command interrupted", ex);
    } catch (ExecutionException ex) {
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable", ex.getCause());
    }
  }

  private static <T> T await(
      Future<T> future, long deadlineNanos, Runnable abortAction) {
    long remainingNanos = deadlineNanos - System.nanoTime();
    if (remainingNanos <= 0) {
      abort(future, abortAction);
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout");
    }
    try {
      return future.get(remainingNanos, TimeUnit.NANOSECONDS);
    } catch (TimeoutException ex) {
      abort(future, abortAction);
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout", ex);
    } catch (InterruptedException ex) {
      abort(future, abortAction);
      Thread.currentThread().interrupt();
      throw new SessionEpochFloorUnavailableException("session epoch floor command interrupted", ex);
    } catch (ExecutionException ex) {
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable", ex.getCause());
    }
  }

  private static void cancel(RedisFuture<?> future) {
    future.cancel(true);
  }

  private static void abort(Future<?> future, Runnable abortAction) {
    abortAction.run();
    future.cancel(true);
  }

  private static void closeOnce(
      StatefulRedisConnection<byte[], byte[]> connection, AtomicBoolean closeScheduled) {
    if (closeScheduled.compareAndSet(false, true)) {
      connection.closeAsync();
    }
  }

  private static LettuceConnectionFactory standaloneFactory(RedisConnectionFactory connectionFactory) {
    if (!(connectionFactory instanceof LettuceConnectionFactory lettuceConnectionFactory)) {
      throw new IllegalArgumentException("session epoch floor requires Lettuce Redis");
    }
    return lettuceConnectionFactory;
  }

  private static RedisClient standaloneClient(LettuceConnectionFactory lettuceConnectionFactory) {
    AbstractRedisClient client = lettuceConnectionFactory.getRequiredNativeClient();
    if (!(client instanceof RedisClient redisClient)) {
      throw new IllegalArgumentException("session epoch floor requires standalone Redis");
    }
    return redisClient;
  }

  private static RedisURI redisUri(LettuceConnectionFactory connectionFactory, Duration timeout) {
    RedisStandaloneConfiguration standalone = connectionFactory.getStandaloneConfiguration();
    LettuceClientConfiguration clientConfiguration = connectionFactory.getClientConfiguration();
    RedisURI.Builder builder =
        RedisURI.Builder.redis(standalone.getHostName(), standalone.getPort())
            .withDatabase(standalone.getDatabase())
            .withTimeout(timeout)
            .withSsl(clientConfiguration.isUseSsl())
            .withStartTls(clientConfiguration.isStartTls())
            .withVerifyPeer(clientConfiguration.getVerifyMode());
    clientConfiguration.getClientName().ifPresent(builder::withClientName);
    char[] password = standalone.getPassword().toOptional().orElseGet(() -> new char[0]);
    String username = standalone.getUsername();
    if (username != null && !username.isBlank()) {
      builder.withAuthentication(username, password);
    } else if (standalone.getPassword().isPresent()) {
      builder.withPassword(password);
    }
    clientConfiguration
        .getRedisCredentialsProviderFactory()
        .map(factory -> factory.createCredentialsProvider(standalone))
        .ifPresent(builder::withAuthentication);
    return builder.build();
  }

  @FunctionalInterface
  private interface RedisUriFactory {
    RedisURI create(Duration timeout);
  }

  private static long parsePositive(String raw) {
    try {
      long parsed = Long.parseLong(raw);
      if (parsed <= 0) {
        throw new NumberFormatException("non-positive");
      }
      return parsed;
    } catch (NumberFormatException ex) {
      throw new SessionEpochFloorUnavailableException("invalid session epoch floor", ex);
    }
  }
}
