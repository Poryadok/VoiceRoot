package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;

import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.time.ZoneOffset;
import java.util.*;
import org.junit.jupiter.api.Test;

public class AuthChatFloorPrincipalTest {
  static final String RPC = "/voice.auth.v1.AuthService/GetChatSessionEpochFloor";
  static final String REQUEST_ID = "5a2980a4-fddc-4b24-aae1-7ef5152e84bd";
  static final String HASH = "sha256:" + "0".repeat(64);
  public static java.security.KeyPair signingKey() { return AuthPrincipalVerifierTest.KEY; }

  /** Shared test-only HTTPS source; no production visibility is changed for fixtures. */
  public static final class HttpsFixture implements AutoCloseable {
    private final AuthPrincipalJwksHttpTest https = new AuthPrincipalJwksHttpTest();
    public HttpsFixture() throws Exception { https.startHttps(); }
    public int requests() { return https.requests.get(); }
    public AuthPrincipalVerifier.KeyResolver keys() { return resolver("/jwks")::resolve; }
    public AuthPrincipalVerifier.KeyResolver outageKeys() {
      https.server.createContext("/outage", exchange -> { exchange.sendResponseHeaders(503, -1); exchange.close(); });
      return resolver("/outage")::resolve;
    }
    private AuthPrincipalJwksResolver resolver(String path) {
      return new AuthPrincipalJwksResolver(issuer -> {
        assertEquals("chat", issuer);
        return AuthPrincipalConfiguration.fetch(https.trusted, https.endpoint(path));
      }, Clock.systemUTC(), java.time.Duration.ofSeconds(30), java.time.Duration.ofMinutes(2), java.time.Duration.ofSeconds(5));
    }
    @Override public void close() { https.stopHttps(); }
  }
  final Set<String> seen = new HashSet<>();
  final AuthPrincipalVerifier verifier = new AuthPrincipalVerifier(
      (issuer, kid) -> (RSAPublicKey) AuthPrincipalVerifierTest.KEY.getPublic(),
      account -> { throw new AssertionError("Chat floor cannot borrow delegated-user epoch lookup"); },
      (issuer, jti, expiry) -> { if (!seen.add(issuer + ":" + jti)) throw new IllegalArgumentException(); },
      Clock.fixed(AuthPrincipalVerifierTest.NOW, ZoneOffset.UTC));

  static Map<String,Object> claims() {
    var claims = AuthPrincipalVerifierTest.claims();
    claims.put("iss", "chat"); claims.put("sub", "service:chat");
    claims.put("principal_type", "service"); claims.put("rpc", RPC);
    claims.put("request_id", REQUEST_ID);
    claims.remove("account_id"); claims.remove("profile_id"); claims.remove("session_epoch");
    return claims;
  }

  @Test void exactChatServiceIsAcceptedOnceAndCannotBorrowAnyOldMethod() {
    var claims = claims();
    String token = AuthPrincipalVerifierTest.token(claims);
    assertEquals("chat", verifier.verify(token, RPC, REQUEST_ID, HASH).issuer());
    assertCode(Status.Code.UNAUTHENTICATED, () -> verifier.verify(token, RPC, REQUEST_ID, HASH));
    for (String other : List.of(AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC,
        AuthPrincipalServerInterceptor.ROLE_SESSION_FLOOR_RPC, AuthPrincipalServerInterceptor.SPACE_SESSION_FLOOR_RPC,
        AuthPrincipalServerInterceptor.BOT_SESSION_FLOOR_RPC, AuthPrincipalServerInterceptor.SPACE_ADMISSION_RPC,
        AuthPrincipalServerInterceptor.ISSUE_RPC, AuthPrincipalServerInterceptor.CONSUME_RPC,
        AuthPrincipalServerInterceptor.LOOKUP_RPC, AuthPrincipalServerInterceptor.SPACE_DELETE_ACK_RPC)) {
      var changed = claims(); changed.put("rpc", other);
      assertCode(Status.Code.PERMISSION_DENIED,
          () -> verifier.verify(AuthPrincipalVerifierTest.token(changed), other, REQUEST_ID, HASH));
    }
  }

  @Test void otherServicesAndDelegatedUsersCannotReadChatFloor() {
    for (String issuer : List.of("role", "voice", "space", "bot", "gateway")) {
      var changed = claims(); changed.put("iss", issuer); changed.put("sub", "service:" + issuer);
      assertCode(Status.Code.PERMISSION_DENIED,
          () -> verifier.verify(AuthPrincipalVerifierTest.token(changed), RPC, REQUEST_ID, HASH));
    }
    var delegated = AuthPrincipalVerifierTest.claims(); delegated.put("rpc", RPC); delegated.put("request_id", REQUEST_ID);
    assertCode(Status.Code.UNAUTHENTICATED,
        () -> verifier.verify(AuthPrincipalVerifierTest.token(delegated), RPC, REQUEST_ID, HASH));
  }

  @Test void extraClaimsAndMalformedRequestIdentityAreRejected() {
    for (String field : List.of("account_id", "profile_id", "session_epoch", "account_type",
        "actor_profile_id", "bot_id", "installation_id", "unknown")) {
      for (Object value : new Object[] {null, "", 0L}) {
        var changed = claims(); changed.put(field, value);
        assertCode(Status.Code.UNAUTHENTICATED,
            () -> verifier.verify(AuthPrincipalVerifierTest.token(changed), RPC, REQUEST_ID, HASH));
      }
    }
    for (String id : List.of("request-1", REQUEST_ID.toUpperCase(), "00000000-0000-0000-0000-000000000000")) {
      var changed = claims(); changed.put("request_id", id);
      assertCode(Status.Code.UNAUTHENTICATED,
          () -> verifier.verify(AuthPrincipalVerifierTest.token(changed), RPC, id, HASH));
    }
    for (String field : List.of("aud", "rpc", "request_id", "request_hash", "sub")) {
      var changed = claims(); changed.put(field, "wrong");
      assertCode(Status.Code.UNAUTHENTICATED,
          () -> verifier.verify(AuthPrincipalVerifierTest.token(changed), RPC, REQUEST_ID, HASH));
    }
  }

  @Test void signatureAndTemporalFailuresDenyThisExactRead() {
    var claims = claims();
    String wrongSignature = AuthPrincipalVerifierTest.token(claims,
        Map.of("alg", "RS256", "kid", "current"), AuthPrincipalVerifierTest.key().getPrivate());
    assertCode(Status.Code.UNAUTHENTICATED, () -> verifier.verify(wrongSignature, RPC, REQUEST_ID, HASH));
    for (String field : List.of("iat", "nbf", "exp")) {
      var changed = claims();
      changed.put(field, field.equals("exp") ? AuthPrincipalVerifierTest.NOW.getEpochSecond()
          : AuthPrincipalVerifierTest.NOW.plusSeconds(6).getEpochSecond());
      assertCode(Status.Code.UNAUTHENTICATED,
          () -> verifier.verify(AuthPrincipalVerifierTest.token(changed), RPC, REQUEST_ID, HASH));
    }
    claims.put("exp", AuthPrincipalVerifierTest.NOW.plusSeconds(31).getEpochSecond());
    assertCode(Status.Code.UNAUTHENTICATED,
        () -> verifier.verify(AuthPrincipalVerifierTest.token(claims), RPC, REQUEST_ID, HASH));
  }

  @Test void unavailableJwksAndReplayAreNotCredentialRejectionsOrAuthorization() {
    for (boolean keyFailure : List.of(true, false)) {
      var unavailable = new AuthPrincipalVerifier(
          (issuer, kid) -> { if (keyFailure) throw new PrincipalDependencyUnavailableException(); return (RSAPublicKey) AuthPrincipalVerifierTest.KEY.getPublic(); },
          account -> { throw new AssertionError(); },
          (issuer, jti, expiry) -> { throw new PrincipalDependencyUnavailableException(); },
          Clock.fixed(AuthPrincipalVerifierTest.NOW, ZoneOffset.UTC));
      assertCode(Status.Code.UNAVAILABLE,
          () -> unavailable.verify(AuthPrincipalVerifierTest.token(claims()), RPC, REQUEST_ID, HASH));
    }
  }

  @Test void interceptorPreservesUnavailableAndNeverCreatesAHandler() {
    var unavailable = new AuthPrincipalVerifier(
        (issuer, kid) -> { throw new PrincipalDependencyUnavailableException(); },
        account -> { throw new AssertionError(); }, (issuer, jti, expires) -> {},
        Clock.fixed(AuthPrincipalVerifierTest.NOW, ZoneOffset.UTC));
    var subject = new AuthPrincipalServerInterceptor(unavailable);
    var request = com.google.protobuf.Struct.getDefaultInstance();
    var claims = claims(); claims.put("request_hash", AuthPrincipalServerInterceptor.requestHash(request));
    var headers = new io.grpc.Metadata();
    headers.put(AuthPrincipalServerInterceptorTest.AUTH, "Bearer " + AuthPrincipalVerifierTest.token(claims));
    headers.put(AuthPrincipalServerInterceptorTest.REQUEST_ID, REQUEST_ID);
    var call = new AuthPrincipalServerInterceptorTest.RecordingCall(RPC);
    var handlers = new java.util.concurrent.atomic.AtomicInteger();
    var listener = subject.interceptCall(call, headers, (c, h) -> {
      handlers.incrementAndGet(); return new io.grpc.ServerCall.Listener<com.google.protobuf.Struct>() {};
    });
    listener.onMessage(request); listener.onHalfClose();
    assertEquals(Status.Code.UNAVAILABLE, call.closed.getCode());
    assertEquals(0, handlers.get());
  }

  static void assertCode(Status.Code expected, org.junit.jupiter.api.function.Executable action) {
    assertEquals(expected, assertThrows(StatusRuntimeException.class, action).getStatus().getCode());
  }
}
