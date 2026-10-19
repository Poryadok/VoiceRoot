package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;

import io.grpc.StatusRuntimeException;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.time.ZoneOffset;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class IdentityAuthorityPrincipalTest {
  static final String ROLE = "/voice.identityerasure.v1.AuthRoleIdentityAuthorityService/ObserveAccounts";
  static final String PARENT = "/voice.identityerasure.v1.AuthUserIdentityAuthorityService/ResolveAccountHandle";
  static final String HANDLES = "/voice.identityerasure.v1.AuthUserIdentityAuthorityService/ObserveAccountHandles";
  static final String HASH = "sha256:" + "0".repeat(64);

  AuthPrincipalVerifier verifier() {
    var seen = new HashSet<String>();
    return AuthPrincipalVerifier.identityAuthority(
        (issuer, kid) -> (RSAPublicKey) AuthPrincipalVerifierTest.KEY.getPublic(),
        (issuer, jti, expires) -> { if (!seen.add(issuer + ":" + jti)) throw new IllegalArgumentException(); },
        Clock.fixed(AuthPrincipalVerifierTest.NOW, ZoneOffset.UTC));
  }

  Map<String, Object> claims(String issuer, String rpc) {
    var claims = new HashMap<>(AuthPrincipalVerifierTest.claims());
    claims.put("iss", issuer); claims.put("sub", "service:" + issuer);
    claims.put("principal_type", "service"); claims.put("rpc", rpc);
    claims.remove("account_id"); claims.remove("profile_id"); claims.remove("session_epoch");
    return claims;
  }

  VerifiedPrincipal verify(AuthPrincipalVerifier verifier, Map<String, Object> claims, String rpc) {
    return verifier.verify(AuthPrincipalVerifierTest.token(claims), rpc, "request-1", HASH);
  }

  @Test void acceptsOnlySeparateRoleAndUserPurposesAndRejectsReplay() {
    for (String rpc : List.of(ROLE, PARENT, HANDLES)) {
      String issuer = rpc.equals(ROLE) ? "role" : "user";
      var verifier = verifier(); var claims = claims(issuer, rpc);
      assertEquals(issuer, verify(verifier, claims, rpc).issuer());
      assertThrows(StatusRuntimeException.class, () -> verify(verifier, claims, rpc));
      for (String other : List.of("role", "user", "space", "gateway", "voice", "moderation")) {
        if (other.equals(issuer)) continue;
        assertThrows(StatusRuntimeException.class, () -> verify(verifier, claims(other, rpc), rpc), other);
      }
    }
  }

  @Test void identitySurfaceCannotBorrowExistingFloorOrOwnershipGrants() {
    for (String rpc : List.of(AuthPrincipalServerInterceptor.ROLE_SESSION_FLOOR_RPC,
        AuthPrincipalServerInterceptor.SPACE_SESSION_FLOOR_RPC,
        AuthPrincipalServerInterceptor.CONSUME_RPC, AuthPrincipalServerInterceptor.ISSUE_RPC,
        AuthPrincipalServerInterceptor.SPACE_ADMISSION_RPC)) {
      for (String issuer : List.of("role", "user")) {
        assertThrows(StatusRuntimeException.class, () -> verify(verifier(), claims(issuer, rpc), rpc));
      }
    }
    var legacy = new AuthPrincipalVerifier(
        (issuer, kid) -> (RSAPublicKey) AuthPrincipalVerifierTest.KEY.getPublic(),
        account -> 2L, (issuer, jti, expires) -> {},
        Clock.fixed(AuthPrincipalVerifierTest.NOW, ZoneOffset.UTC));
    for (String rpc : List.of(ROLE, PARENT, HANDLES)) {
      assertThrows(StatusRuntimeException.class, () -> verify(legacy, claims(rpc.equals(ROLE) ? "role" : "user", rpc), rpc));
    }
  }

  @Test void rejectsIdentityClaimsAndChangedBindingsWithoutConsumingAnotherAuthority() {
    for (String field : List.of("account_id", "profile_id", "session_epoch")) {
      var claims = claims("user", PARENT); claims.put(field, field.equals("session_epoch") ? 2L : UUID.randomUUID().toString());
      assertThrows(StatusRuntimeException.class, () -> verify(verifier(), claims, PARENT), field);
    }
    for (String field : List.of("aud", "sub", "request_id", "request_hash", "rpc", "principal_type")) {
      var claims = claims("user", PARENT); claims.put(field, "wrong");
      assertThrows(StatusRuntimeException.class, () -> verify(verifier(), claims, PARENT), field);
    }
  }

  @Test void closedListenerRejectsUnregisteredAndLegacyMethodsBeforeHandler() {
    var interceptor = AuthPrincipalServerInterceptor.identityAuthority(verifier());
    for (String rpc : List.of("/voice.auth.v1.AuthService/Login",
        AuthPrincipalServerInterceptor.ROLE_SESSION_FLOOR_RPC,
        "/voice.identityerasure.v1.AuthUserIdentityAuthorityService/Unknown")) {
      var call = new AuthPrincipalServerInterceptorTest.RecordingCall(rpc);
      var handlers = new java.util.concurrent.atomic.AtomicInteger();
      interceptor.interceptCall(call, new io.grpc.Metadata(), (c, h) -> {
        handlers.incrementAndGet(); return new io.grpc.ServerCall.Listener<com.google.protobuf.Struct>() {};
      });
      assertNotNull(call.closed); assertEquals(0, handlers.get(), rpc);
    }
  }

  @Test void closedListenerVerifiesBoundBodyBeforeCreatingIdentityHandler() {
    var interceptor = AuthPrincipalServerInterceptor.identityAuthority(verifier());
    var fixture = new AuthPrincipalServerInterceptorTest();
    var call = new AuthPrincipalServerInterceptorTest.RecordingCall(PARENT);
    var handlers = new java.util.concurrent.atomic.AtomicInteger();
    var listener = interceptor.interceptCall(call, fixture.headers(claims("user", PARENT)), (c, h) -> {
      assertEquals("user", VerifiedPrincipal.current().issuer());
      handlers.incrementAndGet(); return new io.grpc.ServerCall.Listener<com.google.protobuf.Struct>() {};
    });
    assertEquals(0, handlers.get()); listener.onMessage(fixture.request); assertEquals(0, handlers.get());
    listener.onHalfClose(); assertNull(call.closed); assertEquals(1, handlers.get());
    assertNull(VerifiedPrincipal.current());
    fixture.assertDenied(interceptor, fixture.headers(claims("user", PARENT)),
        com.google.protobuf.Struct.getDefaultInstance(), PARENT, io.grpc.Status.Code.UNAUTHENTICATED);
  }
}
