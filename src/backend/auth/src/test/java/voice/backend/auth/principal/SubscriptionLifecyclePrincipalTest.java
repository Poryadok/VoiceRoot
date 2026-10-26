package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.time.ZoneOffset;
import java.util.HashSet;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class SubscriptionLifecyclePrincipalTest {
  static final String RPC = "/voice.auth.v1.AuthSubscriptionLifecycleService/ObserveSubscriptionAccount";
  static final String HASH = "sha256:"+"0".repeat(64);
  static final String REQUEST = UUID.randomUUID().toString();
  static Map<String,Object> claims() {
    var claims = AuthPrincipalVerifierTest.claims();
    claims.remove("account_id"); claims.remove("profile_id"); claims.remove("session_epoch");
    claims.put("iss", "subscription"); claims.put("sub", "service:subscription");
    claims.put("principal_type", "service"); claims.put("rpc", RPC); claims.put("request_id", REQUEST);
    return claims;
  }
  static AuthPrincipalVerifier verifier() {
    var seen = new HashSet<String>();
    return AuthPrincipalVerifier.subscriptionLifecycle((issuer,kid) -> {
      assertEquals("subscription",issuer); assertEquals("current",kid);
      return (RSAPublicKey)AuthPrincipalVerifierTest.KEY.getPublic();
    }, (issuer,jti,expires) -> { if (!seen.add(issuer+":"+jti)) throw new IllegalStateException("fixture replay"); },
        Clock.fixed(AuthPrincipalVerifierTest.NOW,ZoneOffset.UTC));
  }
  @Test void exactSubscriptionCapabilityRejectsReplayAndLegacyMethods() {
    var verifier = verifier();
    var claims = claims();
    String token = AuthPrincipalVerifierTest.token(claims);
    assertEquals("subscription",verifier.verify(token,RPC,REQUEST,HASH).issuer());
    assertThrows(io.grpc.StatusRuntimeException.class, () -> verifier.verify(token,RPC,REQUEST,HASH));
    claims.put("jti",UUID.randomUUID().toString()); claims.put("rpc",AuthPrincipalServerInterceptor.ROLE_SESSION_FLOOR_RPC);
    assertThrows(io.grpc.StatusRuntimeException.class, () -> verifier.verify(AuthPrincipalVerifierTest.token(claims),AuthPrincipalServerInterceptor.ROLE_SESSION_FLOOR_RPC,REQUEST,HASH));
    assertThrows(io.grpc.StatusRuntimeException.class, () -> new AuthPrincipalVerifierTest().verifier.verify(token,RPC,REQUEST,HASH));
  }
  @Test void allServiceClaimsAreClosedAndBoundToCanonicalRequestAndJti() {
    var common=claims();common.put("jti",java.util.Base64.getUrlEncoder().withoutPadding().encodeToString("01234567890123456789".getBytes(java.nio.charset.StandardCharsets.US_ASCII)));
    assertEquals("subscription",verifier().verify(AuthPrincipalVerifierTest.token(common),RPC,REQUEST,HASH).issuer());
    for (String field : new String[]{"extra","account_id","profile_id","session_epoch"}) {
      var claims = claims(); claims.put(field,"untrusted");
      assertThrows(io.grpc.StatusRuntimeException.class, () -> verifier().verify(AuthPrincipalVerifierTest.token(claims),RPC,REQUEST,HASH));
    }
    for (String jti : new String[]{"not-a-uuid","00000000-0000-0000-0000-000000000000"}) {
      var claims=claims(); claims.put("jti",jti);
      assertThrows(io.grpc.StatusRuntimeException.class, () -> verifier().verify(AuthPrincipalVerifierTest.token(claims),RPC,REQUEST,HASH));
    }
    assertThrows(io.grpc.StatusRuntimeException.class, () -> verifier().verify(AuthPrincipalVerifierTest.token(claims()),RPC,UUID.randomUUID().toString(),HASH));
    assertThrows(io.grpc.StatusRuntimeException.class, () -> verifier().verify(AuthPrincipalVerifierTest.token(claims()),RPC,REQUEST,"sha256:"+"1".repeat(64)));
    var delegated=claims(); delegated.put("principal_type","delegated_user");
    assertThrows(io.grpc.StatusRuntimeException.class, () -> verifier().verify(AuthPrincipalVerifierTest.token(delegated),RPC,REQUEST,HASH));
  }
}
