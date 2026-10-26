package voice.backend.auth.roleidentityauthority;

import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.netty.shaded.io.netty.handler.ssl.ClientAuth;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContextBuilder;
import java.time.Clock;
import java.time.Duration;
import java.util.Objects;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.function.Supplier;
import javax.sql.DataSource;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.context.SmartLifecycle;
import org.springframework.core.env.Environment;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;
import voice.backend.auth.principal.*;

/** Dedicated owner-input listener. Started transport is never CURRENT/source admission. */
@Component
public final class AuthSubscriptionLifecycleGrpcServer implements SmartLifecycle {
  private final SubscriptionLifecycleTransportConfig config;
  private final Supplier<DataSource> sources;
  private final Supplier<StringRedisTemplate> replaySources;
  private final Environment environment;
  private Owned owned;
  public AuthSubscriptionLifecycleGrpcServer(Environment environment, ObjectProvider<DataSource> sources, ObjectProvider<StringRedisTemplate> redis) {
    this.environment = environment;
    config = SubscriptionLifecycleTransportConfig.load(environment);
    this.sources = sources::getIfAvailable;
    replaySources = redis::getIfAvailable;
  }

  /** Real production factory, also exercised by native TLS/JWKS/Redis/SQL fixtures. */
  public static Owned build(SubscriptionLifecycleTransportConfig config, AuthSubscriptionLifecycleOwnerStore source, StringRedisTemplate redis) throws Exception {
    Objects.requireNonNull(config); Objects.requireNonNull(source); Objects.requireNonNull(redis.getConnectionFactory());
    IdentityAuthorityJwks jwks = null;
    RedisPrincipalReplayGuard replay = null;
    ScheduledExecutorService deadlines = Executors.newSingleThreadScheduledExecutor(Thread.ofPlatform().daemon().name("auth-subscription-budget").factory());
    try {
      Clock clock = Clock.systemUTC();
      jwks = IdentityAuthorityJwks.subscriptionLifecycle(config.jwks(),config.jwksCa(),clock,Duration.ofSeconds(30),Duration.ofMinutes(2),Duration.ofSeconds(5));
      replay = new RedisPrincipalReplayGuard(redis,clock);
      replay.requireAvailable();
      var principal = AuthPrincipalServerInterceptor.subscriptionLifecycle(AuthPrincipalVerifier.subscriptionLifecycle(jwks,replay,clock),config.subscriptionPeerUri());
      var signed = ServerInterceptors.intercept(new AuthSubscriptionLifecycleGrpcAdapter(source),principal);
      var service = ServerInterceptors.intercept(signed,new Budget(deadlines));
      var tls = GrpcSslContexts.configure(SslContextBuilder.forServer(config.certificate().toFile(),config.key().toFile()))
          .trustManager(config.clientCa().toFile()).clientAuth(ClientAuth.REQUIRE).protocols("TLSv1.3","TLSv1.2").build();
      Server server = NettyServerBuilder.forAddress(config.listen()).sslContext(tls).maxInboundMessageSize(1<<20).addService(service).build();
      return new Owned(server,jwks,replay,deadlines);
    } catch (Exception failure) {
      if (replay != null) replay.close();
      if (jwks != null) jwks.close();
      deadlines.shutdownNow(); throw failure;
    }
  }

  public static final class Owned implements AutoCloseable {
    public final Server server;
    private final IdentityAuthorityJwks jwks;
    private final RedisPrincipalReplayGuard replay;
    private final ScheduledExecutorService deadlines;
    private Owned(Server server,IdentityAuthorityJwks jwks,RedisPrincipalReplayGuard replay,ScheduledExecutorService deadlines) {
      this.server=server; this.jwks=jwks; this.replay=replay; this.deadlines=deadlines;
    }
    @Override public void close() { server.shutdownNow(); replay.close(); jwks.close(); deadlines.shutdownNow(); }
  }

  /** Caps authentication and SQL too; a late successful dependency cannot release a response. */
  private static final class Budget implements ServerInterceptor {
    private final ScheduledExecutorService deadlines;
    private Budget(ScheduledExecutorService deadlines) { this.deadlines=deadlines; }
    @Override public <Q,S> ServerCall.Listener<Q> interceptCall(ServerCall<Q,S> call,Metadata headers,ServerCallHandler<Q,S> next) {
      var closed = new AtomicBoolean();
      var bounded = Context.current().withDeadlineAfter(500,TimeUnit.MILLISECONDS,deadlines);
      bounded.addListener(ignored -> {
        if (closed.compareAndSet(false,true)) call.close(Status.DEADLINE_EXCEEDED.withDescription("Subscription owner budget elapsed"),new Metadata());
      },Runnable::run);
      var guarded = new ForwardingServerCall.SimpleForwardingServerCall<Q,S>(call) {
        @Override public void close(Status status,Metadata trailers) { if (closed.compareAndSet(false,true)) super.close(status,trailers); }
      };
      var delegate = Contexts.interceptCall(bounded,guarded,headers,next);
      return new ForwardingServerCallListener.SimpleForwardingServerCallListener<Q>(delegate) {
        @Override public void onCancel() { closed.set(true); bounded.cancel(null); super.onCancel(); }
        @Override public void onComplete() { closed.set(true); bounded.cancel(null); super.onComplete(); }
      };
    }
  }

  @Override public int getPhase() { return -100; }
  @Override public synchronized boolean isRunning() { return owned != null; }
  @Override public synchronized void start() {
    if (config == null || owned != null) return;
    try {
      var source = new AuthSubscriptionLifecycleOwnerStore(Objects.requireNonNull(sources.get()),AuthIdentityDatabasePolicy.load(environment));
      owned = build(config,source,Objects.requireNonNull(replaySources.get()));
      owned.server.start();
    } catch (Exception unavailable) { stop(); throw new IllegalStateException("Subscription lifecycle startup unavailable"); }
  }
  @Override public synchronized void stop() { var closing=owned; owned=null; if (closing!=null) closing.close(); }
}
