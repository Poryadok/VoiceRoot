package voice.backend.auth.roleidentityauthority;

import io.grpc.Server;
import io.grpc.ServerInterceptors;
import io.grpc.ServerServiceDefinition;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.time.Clock;
import java.util.Objects;
import java.util.function.Supplier;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.SmartLifecycle;
import org.springframework.core.env.Environment;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.AuthPrincipalVerifier;
import voice.backend.auth.principal.IdentityAuthorityJwks;
import voice.backend.auth.principal.RedisPrincipalReplayGuard;

/** Dedicated User-purpose serving set. Running means transport started, not owner-source readiness. */
@Component
public final class AuthIdentityAuthorityGrpcServer implements SmartLifecycle {
  private final IdentityAuthorityTransportConfig config;
  private final Supplier<AuthIdentityOwnerStore> stores;
  private final Supplier<StringRedisTemplate> redis;
  private final Factory factory;
  private Owned pending;
  private boolean running;

  @Autowired
  public AuthIdentityAuthorityGrpcServer(Environment environment,
      ObjectProvider<AuthIdentityOwnerStore> stores, ObjectProvider<StringRedisTemplate> redis) {
    this(IdentityAuthorityTransportConfig.load(environment), stores::getIfAvailable,
        redis::getIfAvailable, AuthIdentityAuthorityGrpcServer::production);
  }

  /** Package-only lifecycle seam; production always uses the guarded factory above. */
  AuthIdentityAuthorityGrpcServer(IdentityAuthorityTransportConfig config,
      Supplier<AuthIdentityOwnerStore> stores, Supplier<StringRedisTemplate> redis, Factory factory) {
    this.config = config;
    this.stores = Objects.requireNonNull(stores);
    this.redis = Objects.requireNonNull(redis);
    this.factory = Objects.requireNonNull(factory);
  }

  @FunctionalInterface interface Factory {
    Owned create(IdentityAuthorityTransportConfig config, AuthIdentityOwnerStore store,
        StringRedisTemplate redis) throws Exception;
  }

  static final class Owned implements AutoCloseable {
    final Server server;
    private final AutoCloseable jwks;
    private final AutoCloseable replay;
    private boolean closed;

    Owned(Server server, AutoCloseable jwks, AutoCloseable replay) {
      this.server = Objects.requireNonNull(server);
      this.jwks = Objects.requireNonNull(jwks);
      this.replay = Objects.requireNonNull(replay);
    }

    @Override public synchronized void close() {
      if (closed) return;
      closed = true;
      // Attempt every owned cleanup even if an earlier close fails. Shared Redis is never closed.
      try { server.shutdownNow(); }
      finally { try { replay.close(); } catch (Exception ignored) { }
        finally { try { jwks.close(); } catch (Exception ignored) { } } }
    }
  }

  static ServerServiceDefinition userService(AuthIdentityOwnerStore store,
      AuthPrincipalServerInterceptor interceptor) {
    return ServerInterceptors.intercept(new AuthParentIdentityGrpcAdapter(store), interceptor);
  }

  private static Owned production(IdentityAuthorityTransportConfig config,
      AuthIdentityOwnerStore store, StringRedisTemplate redis) throws Exception {
    Clock clock = Clock.systemUTC();
    IdentityAuthorityJwks jwks = null;
    RedisPrincipalReplayGuard replay = null;
    try {
      jwks = new IdentityAuthorityJwks(config.jwks(), config.jwksCa(), clock,
          config.refresh(), config.hardExpiry(), config.cooldown());
      replay = new RedisPrincipalReplayGuard(redis, clock);
      var interceptor = AuthPrincipalServerInterceptor.identityAuthority(
          AuthPrincipalVerifier.identityAuthority(jwks, replay, clock));
      var builder = NettyServerBuilder.forAddress(config.listen());
      IdentityAuthorityTransport.configure(builder, config);
      return new Owned(builder.addService(userService(store, interceptor)).build(), jwks, replay);
    } catch (Exception failure) {
      if (replay != null) replay.close();
      if (jwks != null) jwks.close();
      throw failure;
    }
  }

  @Override public int getPhase() { return -100; }
  @Override public synchronized boolean isRunning() { return running; }

  @Override public synchronized void start() {
    if (config == null || running) return;
    try {
      var store = Objects.requireNonNull(stores.get(), "identity owner store required");
      var template = Objects.requireNonNull(redis.get(), "identity replay Redis required");
      Objects.requireNonNull(template.getConnectionFactory(), "identity replay connection factory required");
      pending = Objects.requireNonNull(factory.create(config, store, template));
      pending.server.start();
      running = true;
    } catch (Exception failure) {
      stop();
      // Do not leak endpoint, key paths, credentials or database diagnostics.
      throw new IllegalStateException("identity authority startup unavailable");
    }
  }

  @Override public synchronized void stop() {
    running = false;
    Owned closing = pending;
    pending = null;
    if (closing != null) closing.close();
  }
}
