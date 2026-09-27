package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.junit.jupiter.api.Assertions.fail;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jose.jwk.JWKSet;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jose.jwk.gen.RSAKeyGenerator;
import com.nimbusds.jwt.SignedJWT;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Arrays;
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

    Object issuer = load("current");
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
