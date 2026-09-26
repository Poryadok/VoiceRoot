package voice.backend.auth.sdkidentity;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jose.jwk.KeyOperation;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import java.time.Clock;
import java.time.Instant;
import java.util.Objects;
import java.util.function.Function;

/** Verifies independent Google identity; no developer-controlled key discovery is permitted. */
public final class GoogleOidcProofVerifier {
  private static final String ISSUER = "https://accounts.google.com";
  private static final int MAX_TOKEN_LENGTH = 16 * 1024;
  private final Clock clock;
  private final Function<String, RSAKey> keys;

  public GoogleOidcProofVerifier(Clock clock, Function<String, RSAKey> keys) {
    this.clock = Objects.requireNonNull(clock);
    this.keys = Objects.requireNonNull(keys);
  }

  public VerifiedProviderSubject verify(String token, String audience, String nonce) {
    try {
      require(token != null && token.length() <= MAX_TOKEN_LENGTH && !token.isBlank());
      require(audience != null && !audience.isBlank() && nonce != null && !nonce.isBlank());
      SignedJWT jwt = SignedJWT.parse(token);
      JWSHeader header = jwt.getHeader();
      require(JWSAlgorithm.RS256.equals(header.getAlgorithm()));
      require(header.getKeyID() != null && !header.getKeyID().isBlank()
          && header.getKeyID().length() <= 256);
      require(header.getCriticalParams() == null || header.getCriticalParams().isEmpty());
      require(header.isBase64URLEncodePayload());
      require(header.getJWK() == null && header.getJWKURL() == null
          && header.getX509CertURL() == null && header.getX509CertChain() == null);

      RSAKey key = keys.apply(header.getKeyID());
      require(key != null && header.getKeyID().equals(key.getKeyID()) && key.size() >= 2048);
      require(key.getAlgorithm() == null || JWSAlgorithm.RS256.equals(key.getAlgorithm()));
      require(key.getKeyUse() == null || KeyUse.SIGNATURE.equals(key.getKeyUse()));
      require(key.getKeyOperations() == null || key.getKeyOperations().contains(KeyOperation.VERIFY));
      require(jwt.verify(new RSASSAVerifier(key.toRSAPublicKey())));

      JWTClaimsSet claims = jwt.getJWTClaimsSet();
      require(ISSUER.equals(claims.getIssuer()));
      require(claims.getAudience() != null && claims.getAudience().size() == 1
          && audience.equals(claims.getAudience().getFirst()));
      require(!claims.getClaims().containsKey("azp") || audience.equals(claims.getClaim("azp")));
      require(nonce.equals(claims.getClaim("nonce")));
      require(claims.getSubject() != null && !claims.getSubject().isBlank() && claims.getSubject().length() <= 255);
      require(claims.getIssueTime() != null && claims.getExpirationTime() != null);
      Instant now = clock.instant();
      Instant issued = claims.getIssueTime().toInstant();
      require(!issued.isBefore(now.minusSeconds(300)) && !issued.isAfter(now.plusSeconds(30)));
      require(claims.getExpirationTime().toInstant().isAfter(now));
      return new VerifiedProviderSubject(ISSUER, claims.getSubject());
    } catch (Exception ignored) {
      // Never expose provider tokens, claims, key resolution failures or parsing details.
      throw new SdkIdentityDeniedException();
    }
  }

  private static void require(boolean condition) {
    if (!condition) throw new SdkIdentityDeniedException();
  }
}
