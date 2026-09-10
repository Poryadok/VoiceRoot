package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.protobuf.Struct;
import com.google.protobuf.Value;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import java.nio.charset.StandardCharsets;
import java.security.*;
import java.security.interfaces.RSAPublicKey;
import java.time.*;
import java.util.*;
import org.junit.jupiter.api.Test;

class AuthPrincipalVerifierTest {
  static final Instant NOW = Instant.parse("2026-09-10T10:00:00Z");
  static final UUID ACCOUNT = UUID.randomUUID(), PROFILE = UUID.randomUUID();
  static final KeyPair KEY = key();
  static final ObjectMapper JSON = new ObjectMapper();
  final Set<String> replay = new HashSet<>();
  long floor = 2;
  final AuthPrincipalVerifier verifier = new AuthPrincipalVerifier(
      (issuer, kid) -> { if (!kid.equals("current")) throw new IllegalArgumentException(); return (RSAPublicKey) KEY.getPublic(); },
      account -> floor,
      (issuer, jti, expires) -> { if (!replay.add(issuer + ":" + jti)) throw new IllegalArgumentException(); },
      Clock.fixed(NOW, ZoneOffset.UTC));

  static KeyPair key() {
    try { var g = KeyPairGenerator.getInstance("RSA"); g.initialize(2048); return g.generateKeyPair(); }
    catch (Exception e) { throw new RuntimeException(e); }
  }
  static Map<String,Object> claims() {
    var c = new HashMap<String,Object>();
    c.put("iss", "gateway"); c.put("sub", ACCOUNT.toString()); c.put("account_id", ACCOUNT.toString());
    c.put("profile_id", PROFILE.toString()); c.put("session_epoch", 2L);
    c.put("principal_type", "delegated_user"); c.put("aud", "auth");
    c.put("rpc", AuthPrincipalServerInterceptor.ISSUE_RPC); c.put("request_id", "request-1");
    c.put("request_hash", "sha256:" + "0".repeat(64)); c.put("jti", UUID.randomUUID().toString());
    c.put("iat", NOW.getEpochSecond()); c.put("nbf", NOW.getEpochSecond()); c.put("exp", NOW.plusSeconds(30).getEpochSecond());
    return c;
  }
  static String token(Map<String,Object> claims) {
    return token(claims, Map.of("alg", "RS256", "kid", "current"), KEY.getPrivate());
  }
  static String token(Map<String,Object> claims, Map<String,Object> headerClaims, PrivateKey signingKey) {
    try {
      var encoder = Base64.getUrlEncoder().withoutPadding();
      String header = encoder.encodeToString(JSON.writeValueAsBytes(headerClaims));
      String unsigned = header + "." + encoder.encodeToString(JSON.writeValueAsBytes(claims));
      var signature = Signature.getInstance("SHA256withRSA"); signature.initSign(signingKey);
      signature.update(unsigned.getBytes(StandardCharsets.US_ASCII));
      return unsigned + "." + encoder.encodeToString(signature.sign());
    } catch (Exception e) { throw new RuntimeException(e); }
  }
  @Test void rejectsInvalidSignatureWrongKeyAndUntrustedHeaders() throws Exception {
    String signed = token(claims());
    String[] parts = signed.split("\\.");
    byte[] signature = Base64.getUrlDecoder().decode(parts[2]);
    signature[0] ^= 1;
    assertUnauthenticated(parts[0] + "." + parts[1] + "." + Base64.getUrlEncoder().withoutPadding().encodeToString(signature));
    assertUnauthenticated(token(claims(), Map.of("alg", "RS256", "kid", "current"), key().getPrivate()));
    for (Map<String,Object> header : List.<Map<String,Object>>of(
        Map.of("alg", "none", "kid", "current"),
        Map.of("alg", "HS256", "kid", "current"),
        Map.of("alg", "RS256"),
        Map.of("alg", "RS256", "kid", "unknown"))) {
      assertUnauthenticated(token(claims(), header, KEY.getPrivate()));
    }
    String noneHeader = Base64.getUrlEncoder().withoutPadding().encodeToString(
        "{\"alg\":\"none\",\"kid\":\"current\"}".getBytes(StandardCharsets.UTF_8));
    assertUnauthenticated(noneHeader + "." + parts[1] + ".");
    String hsHeader = Base64.getUrlEncoder().withoutPadding().encodeToString(
        "{\"alg\":\"HS256\",\"kid\":\"current\"}".getBytes(StandardCharsets.UTF_8));
    String hsUnsigned = hsHeader + "." + parts[1];
    var mac = javax.crypto.Mac.getInstance("HmacSHA256");
    mac.init(new javax.crypto.spec.SecretKeySpec(KEY.getPublic().getEncoded(), "HmacSHA256"));
    assertUnauthenticated(hsUnsigned + "." + Base64.getUrlEncoder().withoutPadding()
        .encodeToString(mac.doFinal(hsUnsigned.getBytes(StandardCharsets.US_ASCII))));
  }
  void assertUnauthenticated(String credential) {
    var failure = assertThrows(StatusRuntimeException.class, () -> verifier.verify(credential,
        AuthPrincipalServerInterceptor.ISSUE_RPC, "request-1", "sha256:" + "0".repeat(64)));
    assertEquals(Status.Code.UNAUTHENTICATED, Status.fromThrowable(failure).getCode());
  }
  @Test void acceptsExactlyFiveSecondsFutureClockSkew() {
    var c = claims();
    c.put("iat", NOW.plusSeconds(5).getEpochSecond());
    c.put("nbf", NOW.plusSeconds(5).getEpochSecond());
    c.put("exp", NOW.plusSeconds(35).getEpochSecond());
    assertEquals(ACCOUNT, verify(c).accountId());
  }
  @Test void epochLookupCannotReturnPrincipalThatExpiredDuringLookup() {
    var clock = new AuthPrincipalJwksResolverTest.MutableClock();
    var delayed = new AuthPrincipalVerifier((issuer, kid) -> (RSAPublicKey) KEY.getPublic(),
        account -> { clock.advance(30); return 2L; }, (issuer, jti, expires) -> {}, clock);
    var failure = assertThrows(StatusRuntimeException.class, () -> delayed.verify(token(claims()),
        AuthPrincipalServerInterceptor.ISSUE_RPC, "request-1", "sha256:" + "0".repeat(64)));
    assertEquals(Status.Code.UNAUTHENTICATED, Status.fromThrowable(failure).getCode());
  }
  @Test void replayStoreCannotReturnPrincipalThatExpiredDuringRecord() {
    var clock = new AuthPrincipalJwksResolverTest.MutableClock();
    var delayed = new AuthPrincipalVerifier((issuer, kid) -> (RSAPublicKey) KEY.getPublic(),
        account -> 2L, (issuer, jti, expires) -> clock.advance(30), clock);
    var failure = assertThrows(StatusRuntimeException.class, () -> delayed.verify(token(claims()),
        AuthPrincipalServerInterceptor.ISSUE_RPC, "request-1", "sha256:" + "0".repeat(64)));
    assertEquals(Status.Code.UNAUTHENTICATED, Status.fromThrowable(failure).getCode());
  }
  VerifiedPrincipal verify(Map<String,Object> c) {
    return verifier.verify(token(c), AuthPrincipalServerInterceptor.ISSUE_RPC, "request-1", "sha256:" + "0".repeat(64));
  }
  @Test void verifiesGatewayIdentityAndRejectsReplay() {
    var c = claims(); var p = verify(c);
    assertEquals(ACCOUNT, p.accountId()); assertEquals(PROFILE, p.profileId()); assertEquals(2, p.sessionEpoch());
    assertThrows(RuntimeException.class, () -> verify(c));
  }
  @Test void rejectsStaleMissingAndUnavailableEpoch() {
    floor = 3; assertThrows(RuntimeException.class, () -> verify(claims()));
    floor = 0; assertThrows(RuntimeException.class, () -> verify(claims()));
    var unavailable = new AuthPrincipalVerifier((i,k) -> (RSAPublicKey) KEY.getPublic(),
        a -> { throw new IllegalStateException("unavailable"); }, (i,j,e) -> {}, Clock.fixed(NOW, ZoneOffset.UTC));
    assertThrows(RuntimeException.class, () -> unavailable.verify(token(claims()), AuthPrincipalServerInterceptor.ISSUE_RPC, "request-1", "sha256:"+"0".repeat(64)));
  }
  @Test void rejectsWrongBindingsAndIdentity() {
    for (String field : List.of("iss","aud","rpc","request_id","request_hash","sub","account_id","profile_id","principal_type")) {
      var c=claims(); c.put(field, "wrong");
      assertThrows(RuntimeException.class, () -> verify(c), field);
    }
    var c=claims(); c.put("session_epoch", 2.5); assertThrows(RuntimeException.class, () -> verify(c));
  }
  @Test void rejectsTemporalViolationsWithoutExpiryGrace() {
    for (var change : List.of(Map.entry("exp", NOW.getEpochSecond()), Map.entry("exp",NOW.plusSeconds(31).getEpochSecond()),
        Map.entry("iat",NOW.plusSeconds(6).getEpochSecond()), Map.entry("nbf",NOW.plusSeconds(6).getEpochSecond()),
        Map.entry("nbf",NOW.minusSeconds(1).getEpochSecond()))) {
      var c=claims(); c.put(change.getKey(),change.getValue()); assertThrows(RuntimeException.class, () -> verify(c));
    }
    for (String missing : List.of("iat","nbf","exp","jti","session_epoch")) {
      var c=claims(); c.remove(missing); assertThrows(RuntimeException.class, () -> verify(c));
    }
  }
  @Test void acceptsOnlySpaceServiceOnConsume() {
    var c=claims(); c.put("iss","space"); c.put("sub","service:space"); c.put("principal_type","service");
    c.put("rpc",AuthPrincipalServerInterceptor.CONSUME_RPC);
    var p=verifier.verify(token(c), AuthPrincipalServerInterceptor.CONSUME_RPC,"request-1","sha256:"+"0".repeat(64));
    assertEquals("space",p.issuer()); assertNull(p.accountId());
    c.put("iss","role"); c.put("sub","service:role");
    assertThrows(RuntimeException.class, () -> verifier.verify(token(c),AuthPrincipalServerInterceptor.CONSUME_RPC,"request-1","sha256:"+"0".repeat(64)));
  }
  @Test void lookupRequiresSpaceServiceAndFreshReplayAdmissionWithoutAccountEpochLookup() {
    var c = claims();
    c.put("iss", "space"); c.put("sub", "service:space"); c.put("principal_type", "service");
    c.put("rpc", AuthPrincipalServerInterceptor.LOOKUP_RPC);
    c.remove("account_id"); c.remove("profile_id"); c.remove("session_epoch");
    var recorded = new HashSet<String>();
    var lookupVerifier = new AuthPrincipalVerifier((issuer, kid) -> (RSAPublicKey) KEY.getPublic(),
        account -> { throw new AssertionError("historical service lookup must not check account epoch"); },
        (issuer, jti, expires) -> { if (!recorded.add(issuer + ":" + jti)) throw new IllegalArgumentException("replay"); },
        Clock.fixed(NOW, ZoneOffset.UTC));
    var principal = lookupVerifier.verify(token(c), AuthPrincipalServerInterceptor.LOOKUP_RPC, "request-1", "sha256:" + "0".repeat(64));
    assertEquals("space", principal.issuer()); assertEquals("service", principal.kind());
    assertNull(principal.accountId()); assertNull(principal.profileId());
    var replay = assertThrows(StatusRuntimeException.class, () -> lookupVerifier.verify(token(c),
        AuthPrincipalServerInterceptor.LOOKUP_RPC, "request-1", "sha256:" + "0".repeat(64)));
    assertEquals(Status.Code.UNAUTHENTICATED, replay.getStatus().getCode());
    for (String kind : List.of("service", "delegated_user")) {
      var forbidden = claims(); forbidden.put("rpc", AuthPrincipalServerInterceptor.LOOKUP_RPC);
      forbidden.put("principal_type", kind);
      if (kind.equals("service")) {
        forbidden.put("sub", "service:gateway");
        forbidden.remove("account_id"); forbidden.remove("profile_id"); forbidden.remove("session_epoch");
      }
      var failure = assertThrows(StatusRuntimeException.class, () -> verifier.verify(token(forbidden),
          AuthPrincipalServerInterceptor.LOOKUP_RPC, "request-1", "sha256:" + "0".repeat(64)));
      assertEquals(Status.Code.PERMISSION_DENIED, failure.getStatus().getCode());
    }
  }
  @Test void canonicalHashIgnoresProtobufMapInsertionOrder() {
    var a=Struct.newBuilder().putFields("z",Value.newBuilder().setStringValue("last").build()).putFields("a",Value.newBuilder().setNumberValue(1).build()).build();
    var b=Struct.newBuilder().putFields("a",Value.newBuilder().setNumberValue(1).build()).putFields("z",Value.newBuilder().setStringValue("last").build()).build();
    assertEquals(AuthPrincipalServerInterceptor.requestHash(a),AuthPrincipalServerInterceptor.requestHash(b));
    assertNotEquals(AuthPrincipalServerInterceptor.requestHash(a),AuthPrincipalServerInterceptor.requestHash(Struct.getDefaultInstance()));
  }
}
