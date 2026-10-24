package voice.backend.auth.roleidentityauthority;

import io.grpc.Server;
import java.io.IOException;
import java.lang.reflect.Proxy;
import java.net.InetSocketAddress;
import java.time.Clock;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import javax.sql.DataSource;
import org.springframework.data.redis.connection.RedisConnectionFactory;
import org.springframework.data.redis.core.StringRedisTemplate;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.AuthPrincipalVerifier;

/** Pure lifecycle/descriptor checks: no socket, Redis, SQL or TLS listener is opened. */
public final class AuthIdentityAuthorityGrpcServerTest {
  @org.junit.jupiter.api.Test
  void lifecycleAndRegistrationContract() throws Exception { main(new String[0]); }

  public static void main(String[] args) throws Exception {
    disabledDoesNotResolveDependencies();
    missingDependenciesCannotStart();
    startupFailureClosesAllOwnedResources();
    repeatedStartStopAndRestartOwnOnlyLocalResources();
    registeredDescriptorContainsBothUserMethodsOnly();
    System.out.println("PASS 5 identity registration lifecycle/source methods (no transport/native proof)");
  }

  static void disabledDoesNotResolveDependencies() {
    var lifecycle = new AuthIdentityAuthorityGrpcServer(null, () -> { throw new AssertionError(); },
        () -> { throw new AssertionError(); }, (c, s, r) -> { throw new AssertionError(); });
    lifecycle.start(); lifecycle.stop();
    check(!lifecycle.isRunning(), "disabled server running");
  }

  static void missingDependenciesCannotStart() {
    var never = (AuthIdentityAuthorityGrpcServer.Factory) (c, s, r) -> { throw new AssertionError("factory reached"); };
    for (int missing = 0; missing < 3; missing++) {
      final int selected = missing;
      var lifecycle = new AuthIdentityAuthorityGrpcServer(config(), () -> selected == 0 ? null : store(),
          () -> selected == 1 ? null : selected == 2 ? new StringRedisTemplate() : redis(), never);
      denied(lifecycle::start);
      check(!lifecycle.isRunning(), "missing dependency marked running");
    }
  }

  static void startupFailureClosesAllOwnedResources() {
    var events = new ArrayList<String>();
    var server = new FakeServer(events, true);
    var lifecycle = new AuthIdentityAuthorityGrpcServer(config(), AuthIdentityAuthorityGrpcServerTest::store,
        AuthIdentityAuthorityGrpcServerTest::redis, (c, s, r) -> new AuthIdentityAuthorityGrpcServer.Owned(
            server, () -> events.add("jwks-close"), () -> { events.add("replay-close"); throw new IOException(); }));
    denied(lifecycle::start);
    lifecycle.stop();
    check(events.equals(List.of("start", "server-close", "replay-close", "jwks-close")), "failure cleanup/order");
    check(!lifecycle.isRunning(), "failed start running");
  }

  static void repeatedStartStopAndRestartOwnOnlyLocalResources() {
    var events = new ArrayList<String>();
    var lifecycle = new AuthIdentityAuthorityGrpcServer(config(), AuthIdentityAuthorityGrpcServerTest::store,
        AuthIdentityAuthorityGrpcServerTest::redis, (c, s, r) -> new AuthIdentityAuthorityGrpcServer.Owned(
            new FakeServer(events, false), () -> events.add("jwks-close"), () -> events.add("replay-close")));
    lifecycle.start(); lifecycle.start(); check(lifecycle.isRunning(), "successful start not running");
    lifecycle.stop(); lifecycle.stop(); check(!lifecycle.isRunning(), "stopped still running");
    lifecycle.start(); lifecycle.stop();
    check(events.equals(List.of("start", "server-close", "replay-close", "jwks-close",
        "start", "server-close", "replay-close", "jwks-close")), "idempotent lifecycle/restart");
  }

  static void registeredDescriptorContainsBothUserMethodsOnly() {
    var interceptor = AuthPrincipalServerInterceptor.identityAuthority(AuthPrincipalVerifier.identityAuthority(
        (issuer, kid) -> { throw new AssertionError("unexpected key I/O"); },
        (issuer, jti, expiry) -> { throw new AssertionError("unexpected replay I/O"); }, Clock.systemUTC()));
    var service = AuthIdentityAuthorityGrpcServer.userService(store(), interceptor);
    var names = service.getMethods().stream().map(m -> m.getMethodDescriptor().getFullMethodName())
        .collect(java.util.stream.Collectors.toSet());
    String prefix = "voice.identityerasure.v1.AuthUserIdentityAuthorityService/";
    check(names.equals(Set.of(prefix + "ResolveAccountHandle", prefix + "ObserveAccountHandles")),
        "service has missing User method or extra Role/legacy route");
  }

  static IdentityAuthorityTransportConfig config() {
    return new IdentityAuthorityTransportConfig(new InetSocketAddress("127.0.0.1", 0), null, null, null,
        null, Map.of(), Duration.ofSeconds(30), Duration.ofMinutes(2), Duration.ofSeconds(5));
  }

  static AuthIdentityOwnerStore store() {
    byte[] key = new byte[32]; key[0] = 1;
    return new AuthIdentityOwnerStore(proxy(DataSource.class), new RoleIdentitySelectors(1, 1, Map.of(1, key)));
  }

  static StringRedisTemplate redis() {
    var template = new StringRedisTemplate();
    template.setConnectionFactory(proxy(RedisConnectionFactory.class));
    return template;
  }

  @SuppressWarnings("unchecked") static <T> T proxy(Class<T> type) {
    return (T) Proxy.newProxyInstance(type.getClassLoader(), new Class<?>[] { type }, (p, m, a) -> {
      throw new AssertionError("unexpected shared dependency access: " + m.getName());
    });
  }

  static void denied(Runnable action) {
    try { action.run(); } catch (IllegalStateException expected) {
      check("identity authority startup unavailable".equals(expected.getMessage()), "coarse startup error"); return;
    }
    throw new AssertionError("startup unexpectedly succeeded");
  }
  static void check(boolean condition, String message) { if (!condition) throw new AssertionError(message); }

  static final class FakeServer extends Server {
    final List<String> events;
    final boolean fail;
    FakeServer(List<String> events, boolean fail) { this.events = events; this.fail = fail; }
    @Override public Server start() throws IOException { events.add("start"); if (fail) throw new IOException(); return this; }
    @Override public Server shutdown() { events.add("server-close"); return this; }
    @Override public Server shutdownNow() { return shutdown(); }
    @Override public boolean isShutdown() { return events.contains("server-close"); }
    @Override public boolean isTerminated() { return isShutdown(); }
    @Override public boolean awaitTermination(long timeout, TimeUnit unit) { return isTerminated(); }
    @Override public void awaitTermination() { }
  }
}
