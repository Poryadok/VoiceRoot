package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.junit.jupiter.api.Assertions.fail;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jose.jwk.JWKSet;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jose.jwk.Curve;
import com.nimbusds.jose.jwk.ECKey;
import com.nimbusds.jose.jwk.gen.ECKeyGenerator;
import com.nimbusds.jose.jwk.gen.RSAKeyGenerator;
import com.nimbusds.jwt.SignedJWT;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Arrays;
import java.util.List;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Objects;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import voice.backend.auth.rest.AuthRestController;

/** Contract tests for Auth's dedicated User service-principal signing keyset. */
class AuthUserPrincipalIssuerContractTest {
  private static final Instant NOW = Instant.parse("2026-09-27T12:00:00Z");
  private static final String RPC = "/voice.user.v1.UserService/GetSdkProfileEligibility";
  private static final String REQUEST_ID = "d5d5e77a-4725-4528-954e-145d1edfa33f";
  private static final String REQUEST_HASH = "sha256:" + "a".repeat(64);

  @TempDir Path keys;

  @Test
  void signsOnlyTheRequestBoundAuthServicePrincipalWithThirtySecondMaximumLifetime() throws Exception {
    RSAKey current = key("current");
    RSAKey next = key("next");
    write(current);
    write(next);

    AuthUserPrincipalIssuer issuer = (AuthUserPrincipalIssuer) load("current");
    SignedJWT token = SignedJWT.parse((String) invoke(issuer, "issue",
        new Class<?>[] {String.class, String.class, String.class}, RPC, REQUEST_ID, REQUEST_HASH));
    var claims = token.getJWTClaimsSet();

    assertThat(token.getHeader().getAlgorithm()).isEqualTo(JWSAlgorithm.RS256);
    assertThat(token.getHeader().getKeyID()).isEqualTo("current");
    assertThat(token.verify(new RSASSAVerifier(current.toPublicJWK()))).isTrue();
    assertThat(claims.getIssuer()).isEqualTo("auth");
    assertThat(claims.getSubject()).isEqualTo("service:auth");
    assertThat(claims.getAudience()).containsExactly("user");
    assertThat(claims.getClaim("principal_type")).isEqualTo("service");
    assertThat(claims.getClaim("rpc")).isEqualTo(RPC);
    assertThat(claims.getClaim("request_id")).isEqualTo(REQUEST_ID);
    assertThat(claims.getClaim("request_hash")).isEqualTo(REQUEST_HASH);
    assertThat(claims.getIssueTime().toInstant()).isEqualTo(NOW);
    assertThat(claims.getNotBeforeTime().toInstant()).isEqualTo(NOW);
    assertThat(claims.getExpirationTime().toInstant()).isEqualTo(NOW.plusSeconds(30));
    assertThat(claims.getJWTID()).isNotBlank();
    assertThat(claims.getClaims()).doesNotContainKeys("account_id", "profile_id", "session_epoch");
  }

  @Test
  void signsCanonicalDeviceStatusAssertionsWithFourSecondMaximumAndExactClaimSet() throws Exception {
    RSAKey current = key("current");
    write(current);
    write(key("next"));
    AuthUserPrincipalIssuer issuer = (AuthUserPrincipalIssuer) load("current");
    ECKey deviceKey = new ECKeyGenerator(Curve.P_256).generate();
    Map<String, Object> claims = new LinkedHashMap<>();
    claims.put("version", 1L);
    claims.put("iss", "auth");
    claims.put("aud", "voice.game-message");
    claims.put("jti", "d5d5e77a-4725-4528-954e-145d1edfa33f");
    claims.put("application_id", "11111111-1111-4111-8111-111111111111");
    claims.put("environment_id", "22222222-2222-4222-8222-222222222222");
    claims.put("account_id", "33333333-3333-4333-8333-333333333333");
    claims.put("actor_id", "44444444-4444-4444-8444-444444444444");
    claims.put("binding_id", "55555555-5555-4555-8555-555555555555");
    claims.put("device_id", "66666666-6666-4666-8666-666666666666");
    claims.put("key_id", "77777777-7777-4777-8777-777777777777");
    claims.put("public_jwk", deviceKey.toPublicJWK().toJSONObject());
    claims.put("key_thumbprint", deviceKey.computeThumbprint().toString());
    claims.put("device_generation", 1L);
    claims.put("authority_revision", 2L);
    claims.put("status", "active");
    claims.put("not_after", NOW.plusSeconds(90L * 24 * 60 * 60).toEpochMilli());
    claims.put("iat", NOW.toEpochMilli());
    claims.put("exp", NOW.plusSeconds(4).toEpochMilli());

    String compact = issuer.issueDeviceStatus(claims);
    SignedJWT token = SignedJWT.parse(compact);

    assertThat(token.getHeader().getAlgorithm()).isEqualTo(JWSAlgorithm.RS256);
    assertThat(token.getHeader().getKeyID()).isEqualTo("current");
    assertThat(token.getHeader().getType().toString()).isEqualTo("voice.game-device-status+jwt");
    assertThat(token.verify(new RSASSAVerifier(current.toPublicJWK()))).isTrue();
    String payload = new String(token.getPayload().toBytes(), java.nio.charset.StandardCharsets.UTF_8);
    assertThat(com.nimbusds.jose.util.JSONObjectUtils.parse(payload)).containsAllEntriesOf(claims);
    assertThat(payload).startsWith("{\"account_id\":");
    assertThat(issuer.verifyDeviceStatusAssertion(compact).jti()).isEqualTo(java.util.UUID.fromString(
        "d5d5e77a-4725-4528-954e-145d1edfa33f"));
    String[] segments = compact.split("\\.");
    String tampered = segments[0] + "." + segments[1] + "." + (segments[2].charAt(0) == 'A' ? "B" : "A")
        + segments[2].substring(1);
    assertThatThrownBy(() -> issuer.verifyDeviceStatusAssertion(tampered))
        .isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void publishesOnlyTheTwoPublicCurrentAndNextKeysInTheDedicatedPrincipalJwks() throws Exception {
    write(key("current"));
    write(key("next"));
    Object issuer = load("next");

    JWKSet jwks = JWKSet.parse((String) invoke(issuer, "jwksJson", new Class<?>[0]));

    assertThat(jwks.getKeys()).hasSize(2);
    assertThat(jwks.getKeys()).extracting("keyID").containsExactly("current", "next");
    assertThat(jwks.getKeys()).allSatisfy(jwk -> {
      assertThat(jwk.getKeyType().getValue()).isEqualTo("RSA");
      assertThat(jwk.getKeyUse().getValue()).isEqualTo("sig");
      assertThat(jwk.getAlgorithm()).isEqualTo(JWSAlgorithm.RS256);
      assertThat(jwk.isPrivate()).isFalse();
    });
  }

  @Test
  void signsAndVerifiesStrictThirtySecondGameBindingHandoffOnDedicatedPrincipalKeyset() throws Exception {
    RSAKey current = key("current");
    write(current);
    write(key("next"));
    AuthUserPrincipalIssuer issuer = (AuthUserPrincipalIssuer) load("current");
    var handoff = new AuthUserPrincipalIssuer.GameBindingHandoff(
        java.util.UUID.fromString("e3eaa96a-0fef-44f0-9a0d-f04d74a1fd71"),
        java.util.UUID.fromString("671cf221-245f-40bf-adb0-28ed38f0c932"),
        java.util.UUID.fromString("acfa26aa-3637-4be8-98d2-304cfcbac1bc"), "n".repeat(43),
        java.util.UUID.fromString("3119a95d-d7e7-40a7-8b2d-a58b6b0e77d2"),
        java.util.UUID.fromString("2baea249-e4a4-4944-a513-1d9f0895e421"), "a".repeat(64), "p".repeat(43),
        java.util.UUID.fromString("d33f1445-4a1b-45ee-b718-58d75b1ea8c7"), "k".repeat(43), "google",
        "hmac-sha256-v1:k1:" + "b".repeat(64),
        java.util.UUID.fromString("0b258c6c-5d2b-4e50-a2e4-74df88f0015f"),
        java.util.UUID.fromString("4d1e6aca-a5f0-470d-8c3c-8466ed2462e0"),
        java.util.UUID.fromString("d33f1445-4a1b-45ee-b718-58d75b1ea8c7"), 3,
        java.util.UUID.fromString("758c5f92-018d-459a-bb73-c335615f7886"),
        java.util.UUID.fromString("69d141ba-b84d-46ba-922a-a91c54192504"), 12, 4, 7,
        List.of("game.chat.send"), NOW.plusSeconds(30));

    String compact = issuer.issueGameBindingHandoff(handoff);
    SignedJWT token = SignedJWT.parse(compact);
    var claims = token.getJWTClaimsSet();

    assertThat(token.getHeader().getAlgorithm()).isEqualTo(JWSAlgorithm.RS256);
    assertThat(token.getHeader().getKeyID()).isEqualTo("current");
    assertThat(token.getHeader().getType().getType()).isEqualTo("voice.game-binding-handoff+jwt");
    assertThat(token.verify(new RSASSAVerifier(current.toPublicJWK()))).isTrue();
    assertThat(claims.getIssuer()).isEqualTo("auth");
    assertThat(claims.getAudience()).containsExactly("voice.game-binding");
    assertThat(claims.getExpirationTime().toInstant()).isEqualTo(NOW.plusSeconds(30));
    assertThat(claims.getClaim("provider_subject_digest")).isEqualTo(handoff.providerSubjectDigest());
    assertThat(claims.getClaim("scopes")).isEqualTo(List.of("game.chat.send"));
    assertThat(claims.getClaims().keySet()).containsExactlyInAnyOrderElementsOf(
        List.of("iss", "sub", "aud", "iat", "nbf", "exp", "jti", "version", "authorization_request_id",
            "operation_id", "challenge_id", "challenge_nonce", "application_id", "environment_id",
            "redirect_uri_sha256", "pkce_challenge", "device_key_id", "device_key_thumbprint", "provider",
            "provider_subject_digest", "source_account_id", "source_actor_id", "source_device_id", "device_generation",
            "target_account_id", "target_profile_id", "profile_revision", "consent_revision", "policy_revision", "scopes"));
    assertThat(issuer.verifyGameBindingHandoff(compact).operationId()).isEqualTo(handoff.operationId());
    AuthUserPrincipalIssuer laterIssuer = AuthUserPrincipalIssuer.load(keys, "current",
        Clock.fixed(NOW.plusSeconds(61), ZoneOffset.UTC));
    assertThatThrownBy(() -> laterIssuer.verifyGameBindingHandoff(compact)).isInstanceOf(IllegalArgumentException.class);
    assertThat(laterIssuer.verifyGameBindingHandoffForExactReplay(compact).operationId()).isEqualTo(handoff.operationId());
    String[] segments = compact.split("\\.");
    String tampered = segments[0] + "." + segments[1] + "." + (segments[2].charAt(0) == 'A' ? "B" : "A")
        + segments[2].substring(1);
    assertThatThrownBy(() -> issuer.verifyGameBindingHandoff(tampered))
        .isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void signsOnlyTheFrozenShortLivedGameMessageExecutionPermitClaims() throws Exception {
    RSAKey current = key("current");
    write(current);
    write(key("next"));
    AuthUserPrincipalIssuer issuer = (AuthUserPrincipalIssuer) load("current");
    Map<String, Object> claims = new LinkedHashMap<>();
    claims.put("version", 1L);
    claims.put("iss", "auth");
    claims.put("aud", "voice.game-message");
    claims.put("jti", "9d9d59df-5416-48de-9ef8-c18f7f076121");
    claims.put("operation", "message.send");
    claims.put("scope", "game.chat.send");
    claims.put("operation_id", "29db4ec1-05a1-4f6c-87f9-93fc47d7b897");
    claims.put("request_sha256", "a".repeat(64));
    claims.put("application_id", "3119a95d-d7e7-40a7-8b2d-a58b6b0e77d2");
    claims.put("environment_id", "2baea249-e4a4-4944-a513-1d9f0895e421");
    claims.put("account_id", "0b258c6c-5d2b-4e50-a2e4-74df88f0015f");
    claims.put("actor_id", "4d1e6aca-a5f0-470d-8c3c-8466ed2462e0");
    claims.put("binding_id", "758c5f92-018d-459a-bb73-c335615f7886");
    claims.put("profile_id", "69d141ba-b84d-46ba-922a-a91c54192504");
    claims.put("device_id", "d33f1445-4a1b-45ee-b718-58d75b1ea8c7");
    claims.put("key_id", "e3eaa96a-0fef-44f0-9a0d-f04d74a1fd71");
    claims.put("device_generation", 3L);
    claims.put("authority_revision", 4L);
    claims.put("gis_permit_id", "acfa26aa-3637-4be8-98d2-304cfcbac1bc");
    claims.put("binding_revision", 8L);
    claims.put("assertion_jti", "671cf221-245f-40bf-adb0-28ed38f0c932");
    claims.put("iat_ms", NOW.toEpochMilli());
    claims.put("expires_at_ms", NOW.plusMillis(3750).toEpochMilli());
    claims.put("exp", NOW.plusMillis(3750).getEpochSecond());

    String compact = issuer.issueGameMessageExecutionPermit(claims);
    SignedJWT jwt = SignedJWT.parse(compact);
    assertThat(jwt.getHeader().getAlgorithm()).isEqualTo(JWSAlgorithm.RS256);
    assertThat(jwt.getHeader().getKeyID()).isEqualTo("current");
    assertThat(jwt.getHeader().getType().getType()).isEqualTo("voice.game-message-execution-permit+jwt");
    assertThat(jwt.verify(new RSASSAVerifier(current.toPublicJWK()))).isTrue();
    assertThat(jwt.getJWTClaimsSet().getClaims().keySet()).containsExactlyInAnyOrderElementsOf(claims.keySet());
    assertThat(jwt.getJWTClaimsSet().getIssuer()).isEqualTo("auth");
    assertThat(jwt.getJWTClaimsSet().getAudience()).containsExactly("voice.game-message");
    for (String name : claims.keySet()) {
      if (!List.of("aud", "exp").contains(name)) {
        assertThat(jwt.getJWTClaimsSet().getClaim(name)).isEqualTo(claims.get(name));
      }
    }
    assertThat(jwt.getJWTClaimsSet().getExpirationTime().toInstant().getEpochSecond())
        .isEqualTo(NOW.plusMillis(3750).getEpochSecond());
    claims.put("unreviewed_scope", "game.chat.send");
    assertThatThrownBy(() -> issuer.issueGameMessageExecutionPermit(claims))
        .isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void principalJwksRouteIsDistinctFromExistingClientJwtJwksRoute() throws Exception {
    String clientRoute = mappedGetRoute(AuthRestController.class, "jwks");
    Class<?> principalController = issuerType().getClassLoader()
        .loadClass("voice.backend.auth.sdkidentity.AuthUserPrincipalJwksController");
    String principalRoute = mappedGetRoute(principalController, "jwks");

    assertThat(clientRoute).isEqualTo("/api/v1/auth/.well-known/jwks.json");
    assertThat(principalRoute).isEqualTo("/api/v1/auth/.well-known/principal-jwks.json")
        .isNotEqualTo(clientRoute);
  }

  @Test
  void rejectsPartialMalformedAndUnselectedKeysets() throws Exception {
    issuerType();
    write(key("current"));
    assertThatThrownBy(() -> load("current")).isInstanceOf(IllegalArgumentException.class);

    Files.writeString(keys.resolve("next.pem"), "not a PKCS#8 private key");
    assertThatThrownBy(() -> load("current")).isInstanceOf(IllegalArgumentException.class);

    Files.writeString(keys.resolve("next.pem"), pem(key("next").toPrivateKey().getEncoded()));
    assertThatThrownBy(() -> load("absent")).isInstanceOf(IllegalArgumentException.class);

    Files.write(keys.resolve("next.pem"), Files.readAllBytes(keys.resolve("current.pem")));
    assertThatThrownBy(() -> load("current")).isInstanceOf(IllegalArgumentException.class);

    var weak = java.security.KeyPairGenerator.getInstance("RSA");
    weak.initialize(1024);
    Files.writeString(keys.resolve("next.pem"), pem(weak.generateKeyPair().getPrivate().getEncoded()));
    assertThatThrownBy(() -> load("current")).isInstanceOf(IllegalArgumentException.class);
  }

  private RSAKey key(String kid) throws Exception {
    return new RSAKeyGenerator(2048).keyID(kid).generate();
  }

  private void write(RSAKey key) throws Exception {
    Files.writeString(keys.resolve(key.getKeyID() + ".pem"), pem(key.toPrivateKey().getEncoded()));
  }

  private static String pem(byte[] der) {
    return "-----BEGIN PRIVATE KEY-----\n"
        + java.util.Base64.getMimeEncoder(64, new byte[] {'\n'}).encodeToString(der)
        + "\n-----END PRIVATE KEY-----\n";
  }

  private Object load(String activeKid) throws Exception {
    Class<?> type = issuerType();
    try {
      return type.getMethod("load", Path.class, String.class, Clock.class)
          .invoke(null, keys, activeKid, Clock.fixed(NOW, ZoneOffset.UTC));
    } catch (java.lang.reflect.InvocationTargetException wrapped) {
      if (wrapped.getCause() instanceof RuntimeException cause) throw cause;
      throw wrapped;
    }
  }

  private static Class<?> issuerType() {
    try {
      return Class.forName("voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer");
    } catch (ClassNotFoundException absentImplementation) {
      return fail("AuthUserPrincipalIssuer is not implemented yet", absentImplementation);
    }
  }

  private static String mappedGetRoute(Class<?> controller, String methodName) throws Exception {
    RequestMapping root = controller.getAnnotation(RequestMapping.class);
    var method = controller.getDeclaredMethod(methodName);
    GetMapping mapping = method.getAnnotation(GetMapping.class);
    if (root == null || mapping == null) throw new AssertionError("missing Spring GET route mapping");
    String rootPath = Arrays.stream(root.value()).filter(Objects::nonNull).findFirst().orElse("");
    String methodPath = Arrays.stream(mapping.value()).filter(Objects::nonNull).findFirst().orElse("");
    return rootPath + methodPath;
  }

  private static Object invoke(Object target, String name, Class<?>[] parameterTypes, Object... args)
      throws Exception {
    return target.getClass().getMethod(name, parameterTypes).invoke(target, args);
  }
}
