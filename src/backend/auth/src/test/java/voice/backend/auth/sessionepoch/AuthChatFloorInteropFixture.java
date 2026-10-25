package voice.backend.auth.sessionepoch;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Server;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import org.flywaydb.core.Flyway;
import org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory;
import org.springframework.data.redis.connection.RedisStandaloneConfiguration;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.mock.env.MockEnvironment;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.principal.AuthPrincipalConfiguration;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.AuthPrincipalServices;
import voice.backend.auth.principal.AuthPrincipalTransport;
import voice.backend.auth.repository.JdbcAccountRepository;

/** Bounded, test-only production Chat floor receiver for private Linux composition. */
public final class AuthChatFloorInteropFixture {
  static final String PREFIX = "VOICE_AUTH_CHAT_FLOOR_INTEROP_";
  private static final ObjectMapper JSON = new ObjectMapper();
  private AuthChatFloorInteropFixture() {}

  static void validateInputs(Map<String, String> environment, Path output, int lifetime) {
    Config.read(environment, output, lifetime);
  }

  public static void main(String[] args) throws Exception {
    if (args.length != 2) throw new IllegalArgumentException("expected output directory and lifetime 1..600");
    final Config config;
    try {
      config = Config.read(System.getenv(), Path.of(args[0]), Integer.parseInt(args[1]));
    } catch (RuntimeException failure) {
      throw new IllegalArgumentException("invalid Chat fixture inputs");
    }
    try (var fixture = start(config)) {
      Thread shutdown = new Thread(fixture::close, "chat-floor-fixture-shutdown");
      Runtime.getRuntime().addShutdownHook(shutdown);
      try { new CountDownLatch(1).await(config.lifetime(), TimeUnit.SECONDS); }
      finally { Runtime.getRuntime().removeShutdownHook(shutdown); }
    }
  }

  record Config(Map<String, String> env, Path output, Path root, int lifetime,
                UUID run, UUID account, UUID unknown, UUID deleted, long durable, long retained) {
    static Config read(Map<String, String> env, Path output, int lifetime) {
      try {
        if (lifetime < 1 || lifetime > 600) throw new IllegalArgumentException();
        UUID run = uuid(required(env, "RUN_ID"));
        Path root = Path.of(required(env, "INPUT_ROOT")).toRealPath();
        Path out = output.toRealPath();
        if (!Files.isDirectory(root) || !Files.isDirectory(out) || !out.startsWith(root)
            || Files.exists(out.resolve("ready.json")) || Files.exists(out.resolve("closed.json")))
          throw new IllegalArgumentException();
        for (String name : new String[] {"AUTH_SOURCE_SHA", "LAUNCHER_SOURCE_SHA"})
          if (!required(env, name).matches("[0-9a-f]{40}")) throw new IllegalArgumentException();
        for (String name : new String[] {"HOST", "TLS_SERVER_NAME", "JDBC_URL", "JDBC_USERNAME", "REDIS_HOST"})
          required(env, name);
        if (!required(env, "JDBC_URL").startsWith("jdbc:postgresql://")) throw new IllegalArgumentException();
        var jdbcUri = java.net.URI.create(required(env, "JDBC_URL").substring(5));
        if (jdbcUri.getHost() == null || jdbcUri.getRawUserInfo() != null
            || jdbcUri.getRawQuery() != null || jdbcUri.getRawFragment() != null) throw new IllegalArgumentException();
        int redisPort = Integer.parseInt(env.getOrDefault(PREFIX + "REDIS_PORT", "6379"));
        if (redisPort < 1 || redisPort > 65535) throw new IllegalArgumentException();
        for (String name : new String[] {"CA_FILE", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE", "JDBC_PASSWORD_FILE",
            "JWKS_TRUSTSTORE_FILE", "JWKS_TRUSTSTORE_PASSWORD_FILE"})
          inside(root, required(env, name));
        for (String name : new String[] {"AUTH_GRPC_TLS_CERT_FILE", "AUTH_GRPC_TLS_KEY_FILE", "AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE"})
          inside(root, raw(env, name));
        if (java.util.Arrays.equals(Files.readAllBytes(inside(root, required(env, "CLIENT_CERT_FILE"))),
            Files.readAllBytes(inside(root, raw(env, "AUTH_GRPC_TLS_CERT_FILE")))))
          throw new IllegalArgumentException();
        // Use the production configuration to validate issuer URLs/cache policy later;
        // never replace it with a fixture KeyResolver or an in-memory replay guard.
        raw(env, "S2S_JWKS_URLS_JSON");
        var jwks = JSON.readTree(raw(env, "S2S_JWKS_URLS_JSON"));
        if (!jwks.isObject() || !jwks.hasNonNull("chat")) throw new IllegalArgumentException();
        UUID account = uuid(env.getOrDefault(PREFIX + "ACCOUNT_ID", derived(run, "active").toString()));
        UUID unknown = derived(run, "unknown"), deleted = derived(run, "deleted");
        if (account.equals(unknown) || account.equals(deleted)) throw new IllegalArgumentException();
        long durable = Long.parseLong(env.getOrDefault(PREFIX + "DURABLE_EPOCH", "7"));
        long retained = Long.parseLong(env.getOrDefault(PREFIX + "RETAINED_EPOCH", "9"));
        if (durable <= 0 || retained <= 0) throw new IllegalArgumentException();
        return new Config(Map.copyOf(env), out, root, lifetime, run, account, unknown, deleted, durable, retained);
      } catch (Exception failure) { throw new IllegalArgumentException("invalid Chat fixture inputs"); }
    }

    String value(String name) { return required(env, name); }
    Path file(String name) { return inside(root, value(name)); }
    MockEnvironment production() {
      var environment = new MockEnvironment();
      env.forEach(environment::setProperty);
      environment.setProperty("auth.persistence", "jdbc");
      environment.setActiveProfiles("production");
      return environment;
    }
  }

  static Running start(Config config) throws Exception {
    var running = new Running(config);
    try {
      String password = Files.readString(config.file("JDBC_PASSWORD_FILE")).stripTrailing();
      var source = new DriverManagerDataSource(config.value("JDBC_URL"), config.value("JDBC_USERNAME"), password);
      Flyway.configure().dataSource(source).load().migrate();
      var jdbc = new NamedParameterJdbcTemplate(source);
      var accounts = new JdbcAccountRepository(jdbc);
      // A fresh, fixture-owned database is required. Never overwrite an existing identity.
      if (jdbc.getJdbcTemplate().queryForObject("SELECT count(*) FROM accounts", Long.class) != 0L)
        throw new IllegalArgumentException("Chat fixture requires a fresh owned database");
      var transaction = new org.springframework.transaction.support.TransactionTemplate(
          new org.springframework.jdbc.datasource.DataSourceTransactionManager(source));
      transaction.executeWithoutResult(status -> {
        seed(jdbc, config.account(), config.durable(), false, config.run());
        seed(jdbc, config.deleted(), config.durable(), true, config.run());
      });
      var redisConfig = new RedisStandaloneConfiguration(config.value("REDIS_HOST"),
          Integer.parseInt(config.env().getOrDefault(PREFIX + "REDIS_PORT", "6379")));
      var redisClient = org.springframework.data.redis.connection.lettuce.LettuceClientConfiguration.builder()
          .commandTimeout(Duration.ofSeconds(2)).shutdownTimeout(Duration.ofSeconds(1)).build();
      running.redis = new LettuceConnectionFactory(redisConfig, redisClient);
      running.redis.afterPropertiesSet();
      var template = new StringRedisTemplate(running.redis);
      template.afterPropertiesSet();
      try (var connection = running.redis.getConnection()) {
        if (connection.serverCommands().dbSize() != 0L)
          throw new IllegalArgumentException("Chat fixture requires a fresh owned Redis");
      }
      running.floors = new RedisSessionEpochFloorStore(new StringRedisSessionEpochCommands(template), Duration.ofSeconds(2));
      running.floors.recordAtLeast(config.account(), config.retained());
      running.floors.recordAtLeast(config.deleted(), config.retained());
      var environment = config.production();
      // This launcher runs in its own bounded JVM. Use JSSE trust configuration
      // consumed by the real production HttpClient, not a substitute resolver.
      System.setProperty("javax.net.ssl.trustStore", config.file("JWKS_TRUSTSTORE_FILE").toString());
      System.setProperty("javax.net.ssl.trustStorePassword",
          Files.readString(config.file("JWKS_TRUSTSTORE_PASSWORD_FILE")).stripTrailing());
      running.interceptor = AuthPrincipalConfiguration.fromEnvironment(environment, running.floors, template);
      var service = new AuthGrpcService(null, null);
      service.setVoiceSessionFloorSources(accounts, running.floors);
      var builder = NettyServerBuilder.forPort(0);
      AuthPrincipalTransport.configureSessionFloor(builder, environment, true);
      running.server = builder.addService(AuthPrincipalServices.sessionFloorService(
          service.bindService(), running.interceptor)).build().start();
      Instant started = Instant.now();
      var ready = new LinkedHashMap<String, Object>();
      ready.put("schema_version", 1);
      ready.put("run_id", config.run().toString());
      ready.put("auth_source_sha", config.value("AUTH_SOURCE_SHA"));
      ready.put("launcher_source_sha", config.value("LAUNCHER_SOURCE_SHA"));
      ready.put("host", config.value("HOST"));
      ready.put("port", running.server.getPort());
      ready.put("tls_server_name", config.value("TLS_SERVER_NAME"));
      ready.put("ca_file", config.file("CA_FILE").toString());
      ready.put("client_cert_file", config.file("CLIENT_CERT_FILE").toString());
      ready.put("client_key_file", config.file("CLIENT_KEY_FILE").toString());
      ready.put("rpc", AuthPrincipalServerInterceptor.CHAT_SESSION_FLOOR_RPC);
      ready.put("account_id", config.account().toString());
      ready.put("unknown_account_id", config.unknown().toString());
      ready.put("deleted_account_id", config.deleted().toString());
      ready.put("durable_epoch", config.durable());
      ready.put("retained_epoch", config.retained());
      ready.put("expected_floor", Math.max(config.durable(), config.retained()));
      ready.put("started_at", started.toString());
      ready.put("expires_at", started.plusSeconds(config.lifetime()).toString());
      atomic(config.output().resolve("ready.json"), ready);
      running.published = true;
      return running;
    } catch (Exception failure) {
      running.close();
      // Do not leak JDBC credentials, signing material or environment values.
      throw new IllegalStateException("Chat fixture setup failed before readiness");
    }
  }

  private static void seed(NamedParameterJdbcTemplate jdbc, UUID id, long epoch, boolean deleted, UUID run) {
    jdbc.update("""
        INSERT INTO accounts (id,email,password_hash,type,status,session_epoch,deleted_at)
        VALUES (:id,:email,'fixture-only','regular',:status,:epoch,:deleted)
        """, new MapSqlParameterSource().addValue("id", id)
        .addValue("email", run + "-" + id + "@example.test")
        .addValue("status", deleted ? "deleted" : "active").addValue("epoch", epoch)
        .addValue("deleted", deleted ? java.sql.Timestamp.from(Instant.now()) : null));
  }

  static final class Running implements AutoCloseable {
    final Config config;
    LettuceConnectionFactory redis;
    RedisSessionEpochFloorStore floors;
    AuthPrincipalServerInterceptor interceptor;
    Server server;
    boolean published, closed;
    Running(Config config) { this.config = config; }

    @Override public synchronized void close() {
      if (closed) return;
      closed = true;
      boolean terminated = server == null;
      if (server != null) {
        server.shutdownNow();
        try { terminated = server.awaitTermination(5, TimeUnit.SECONDS); }
        catch (InterruptedException failure) { Thread.currentThread().interrupt(); }
      }
      RuntimeException cleanupFailure = null;
      try { if (interceptor != null) interceptor.close(); }
      catch (RuntimeException failure) { cleanupFailure = failure; }
      try { if (redis != null) redis.destroy(); }
      catch (RuntimeException failure) { cleanupFailure = failure; }
      // Redis floor worker threads are virtual/idle and expire after30s; the
      // production store deliberately exposes no fixture-specific close API.
      try {
        if (published) Files.deleteIfExists(config.output().resolve("ready.json"));
        atomic(config.output().resolve("closed.json"), Map.of("schema_version", 1,
            "run_id", config.run().toString(), "server_terminated", terminated,
            "ready_removed", !Files.exists(config.output().resolve("ready.json")),
            "clients_closed", cleanupFailure == null, "closed_at", Instant.now().toString()));
      } catch (Exception failure) { throw new IllegalStateException("Chat fixture cleanup receipt failed"); }
      if (cleanupFailure != null) throw new IllegalStateException("Chat fixture client cleanup failed");
    }
  }

  private static void atomic(Path destination, Map<String, Object> value) throws Exception {
    Path pending = destination.resolveSibling(destination.getFileName() + ".pending");
    Files.write(pending, JSON.writeValueAsBytes(value), java.nio.file.StandardOpenOption.CREATE_NEW);
    try { Files.move(pending, destination, StandardCopyOption.ATOMIC_MOVE); }
    finally { Files.deleteIfExists(pending); }
  }
  private static String required(Map<String, String> env, String name) { return raw(env, PREFIX + name); }
  private static String raw(Map<String, String> env, String name) {
    String value = env.get(name);
    if (value == null || value.isBlank()) throw new IllegalArgumentException("incomplete Chat fixture inputs");
    return value;
  }
  private static UUID uuid(String value) {
    UUID id = UUID.fromString(value);
    if (!id.toString().equals(value) || id.equals(new UUID(0, 0))) throw new IllegalArgumentException();
    return id;
  }
  private static UUID derived(UUID run, String name) {
    return UUID.nameUUIDFromBytes((run + ":" + name).getBytes(java.nio.charset.StandardCharsets.UTF_8));
  }
  private static Path inside(Path root, String value) {
    try {
      Path file = Path.of(value).toRealPath();
      if (!file.startsWith(root) || !Files.isRegularFile(file)) throw new IllegalArgumentException();
      return file;
    } catch (Exception failure) { throw new IllegalArgumentException("fixture file outside declared mount or missing"); }
  }
}
