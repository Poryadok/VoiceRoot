package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.crypto.MACSigner;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jose.jwk.gen.RSAKeyGenerator;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.PlainJWT;
import com.nimbusds.jwt.SignedJWT;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Date;
import java.util.List;
import java.util.function.Consumer;
import java.util.stream.Stream;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestFactory;

/** Independent provider proof: developer credentials cannot authenticate a player. */
class GoogleOidcProofVerifierTest {
  private static final Instant NOW = Instant.parse("2026-09-26T12:00:00Z");
  private static final String ISSUER = "https://accounts.google.com";
  private static final String AUDIENCE = "voice-sdk.apps.googleusercontent.com";
  private static final String NONCE = "server-issued-device-bound-nonce";
  private static RSAKey trustedKey;
  private static RSAKey attackerKey;

  @BeforeAll
  static void generateSigningKeys() throws Exception {
    trustedKey = new RSAKeyGenerator(2048).keyID("google-trusted-key").generate();
    attackerKey = new RSAKeyGenerator(2048).keyID("developer-key").generate();
  }

  private GoogleOidcProofVerifier verifier() {
    return new GoogleOidcProofVerifier(Clock.fixed(NOW, ZoneOffset.UTC),
        kid -> trustedKey.getKeyID().equals(kid) ? trustedKey.toPublicJWK() : null);
  }

  private JWTClaimsSet.Builder claims() {
    return new JWTClaimsSet.Builder().issuer(ISSUER).subject("provider-subject-123")
        .audience(AUDIENCE).claim("nonce", NONCE)
        .issueTime(Date.from(NOW.minusSeconds(60)))
        .expirationTime(Date.from(NOW.plusSeconds(300)));
  }

  private String signed(JWTClaimsSet.Builder claims) throws Exception {
    return signed(claims.build(), new JWSHeader.Builder(JWSAlgorithm.RS256)
        .keyID(trustedKey.getKeyID()).build(), trustedKey);
  }

  private String signed(JWTClaimsSet claims, JWSHeader header, RSAKey key) throws Exception {
    SignedJWT jwt = new SignedJWT(header, claims);
    jwt.sign(new RSASSASigner(key));
    return jwt.serialize();
  }

  private void denied(String token) {
    assertThatThrownBy(() -> verifier().verify(token, AUDIENCE, NONCE))
        .isInstanceOf(SdkIdentityDeniedException.class)
        .hasMessage("invalid_sdk_identity");
  }

  @Test
  void acceptsTrustedGoogleProofAndReturnsOnlyVerifiedIssuerAndSubject() throws Exception {
    var subject = verifier().verify(signed(claims().claim("email", "untrusted@example.com")
        .claim("email_verified", false)), AUDIENCE, NONCE);
    assertThat(subject.issuer()).isEqualTo(ISSUER);
    assertThat(subject.subject()).isEqualTo("provider-subject-123");
  }

  @Test
  void acceptsMatchingAuthorizedPartyWhenPresent() throws Exception {
    var subject = verifier().verify(signed(claims().claim("azp", AUDIENCE)), AUDIENCE, NONCE);
    assertThat(subject.subject()).isEqualTo("provider-subject-123");
  }

  @Test
  void subjectMustFitTheDurableIdentityBoundary() throws Exception {
    assertThat(verifier().verify(signed(claims().subject("s".repeat(255))), AUDIENCE, NONCE).subject())
        .hasSize(255);
    denied(signed(claims().subject("s".repeat(256))));
  }

  @Test
  void acceptsFreshnessAndFutureClockSkewBoundaries() throws Exception {
    for (Instant issuedAt : List.of(NOW.minusSeconds(300), NOW.plusSeconds(30))) {
      assertThat(verifier().verify(signed(claims().issueTime(Date.from(issuedAt))),
          AUDIENCE, NONCE).subject()).isEqualTo("provider-subject-123");
    }
  }

  private record InvalidClaim(String name, Consumer<JWTClaimsSet.Builder> change) {}

  @TestFactory
  Stream<DynamicTest> rejectsInvalidOrMissingSecurityClaims() {
    return Stream.of(
        new InvalidClaim("developer issuer", c -> c.issuer("https://game.example.com")),
        new InvalidClaim("issuer lookalike", c -> c.issuer(ISSUER + ".attacker.example")),
        new InvalidClaim("noncanonical Google issuer", c -> c.issuer("accounts.google.com")),
        new InvalidClaim("missing issuer", c -> c.issuer(null)),
        new InvalidClaim("wrong audience", c -> c.audience("developer-service")),
        new InvalidClaim("extra audience", c -> c.audience(List.of(AUDIENCE, "other-client"))),
        new InvalidClaim("missing audience", c -> c.audience((String) null)),
        new InvalidClaim("empty audience", c -> c.audience(List.of())),
        new InvalidClaim("wrong azp", c -> c.claim("azp", "other-client")),
        new InvalidClaim("empty azp", c -> c.claim("azp", "")),
        new InvalidClaim("nonstring azp", c -> c.claim("azp", 42)),
        new InvalidClaim("wrong nonce", c -> c.claim("nonce", "other-device-nonce")),
        new InvalidClaim("missing nonce", c -> c.claim("nonce", null)),
        new InvalidClaim("empty nonce", c -> c.claim("nonce", "")),
        new InvalidClaim("nonstring nonce", c -> c.claim("nonce", 42)),
        new InvalidClaim("missing subject", c -> c.subject(null)),
        new InvalidClaim("empty subject", c -> c.subject("")),
        new InvalidClaim("blank subject", c -> c.subject(" \t ")),
        new InvalidClaim("missing expiry", c -> c.expirationTime(null)),
        new InvalidClaim("expired", c -> c.expirationTime(Date.from(NOW.minusSeconds(1)))),
        new InvalidClaim("expiry at current instant", c -> c.expirationTime(Date.from(NOW))),
        new InvalidClaim("missing issued time", c -> c.issueTime(null)),
        new InvalidClaim("stale issued time", c -> c.issueTime(Date.from(NOW.minusSeconds(301)))),
        new InvalidClaim("future issued time", c -> c.issueTime(Date.from(NOW.plusSeconds(31)))))
        .map(example -> DynamicTest.dynamicTest(example.name(), () -> {
          var builder = claims();
          example.change().accept(builder);
          denied(signed(builder));
        }));
  }

  @Test
  void rejectsMissingOrEmptyProofAndExpectedBindings() throws Exception {
    String valid = signed(claims());
    for (String missing : new String[] {null, "", " \t "}) {
      denied(missing);
      assertThatThrownBy(() -> verifier().verify(valid, missing, NONCE))
          .isInstanceOf(SdkIdentityDeniedException.class);
      assertThatThrownBy(() -> verifier().verify(valid, AUDIENCE, missing))
          .isInstanceOf(SdkIdentityDeniedException.class);
    }
    denied("not-a-token");
    denied("service-credential-from-game-developer");
  }

  @Test
  void rejectsDeveloperSignatureEvenWhenClaimingTrustedKeyId() throws Exception {
    denied(signed(claims().build(), new JWSHeader.Builder(JWSAlgorithm.RS256)
        .keyID(trustedKey.getKeyID()).build(), attackerKey));
  }

  @Test
  void rejectsUnknownAndMissingSigningKeyIdentifiers() throws Exception {
    for (String kid : new String[] {null, "", "unknown-provider-key"}) {
      denied(signed(claims().build(), new JWSHeader.Builder(JWSAlgorithm.RS256)
          .keyID(kid).build(), trustedKey));
    }
  }

  @Test
  void embeddedDeveloperPublicKeyCannotReplaceTrustedRegistryKey() throws Exception {
    denied(signed(claims().build(), new JWSHeader.Builder(JWSAlgorithm.RS256)
        .keyID(trustedKey.getKeyID()).jwk(attackerKey.toPublicJWK()).build(), attackerKey));
  }

  @Test
  void rejectsOtherRsaAlgorithmsEvenWithValidSignature() throws Exception {
    for (JWSAlgorithm algorithm : List.of(JWSAlgorithm.RS384, JWSAlgorithm.PS256)) {
      denied(signed(claims().build(), new JWSHeader.Builder(algorithm)
          .keyID(trustedKey.getKeyID()).build(), trustedKey));
    }
  }

  @Test
  void rejectsUnsignedAndSymmetricProofs() throws Exception {
    denied(new PlainJWT(claims().build()).serialize());
    SignedJWT symmetric = new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.HS256)
        .keyID(trustedKey.getKeyID()).build(), claims().build());
    symmetric.sign(new MACSigner(new byte[32]));
    denied(symmetric.serialize());
  }
}
