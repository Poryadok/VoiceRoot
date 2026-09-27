package voice.backend.auth.sdkidentity;

import com.nimbusds.jose.JOSEException;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.jwk.JWK;
import com.nimbusds.jose.jwk.JWKSet;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jose.util.JSONObjectUtils;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.interfaces.RSAPrivateCrtKey;
import java.security.interfaces.RSAPrivateKey;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.PKCS8EncodedKeySpec;
import java.security.spec.RSAPublicKeySpec;
import java.time.Clock;
import java.time.Instant;
import java.util.Base64;
import java.util.Comparator;
import java.util.Date;
import java.util.List;
import java.util.UUID;

/** Dedicated RS256 workload credentials for request-bound Auth-to-User calls. */
public final class AuthUserPrincipalIssuer {
  private static final String ELIGIBILITY_RPC = "/voice.user.v1.UserService/GetSdkProfileEligibility";
  private static final String ISSUER = "auth";
  private static final String SUBJECT = "service:auth";
  private static final String AUDIENCE = "user";
  private static final int MAX_LIFETIME_SECONDS = 30;
  private final List<RSAKey> keys;
  private final RSAKey active;
  private final Clock clock;

  private AuthUserPrincipalIssuer(List<RSAKey> keys, RSAKey active, Clock clock) {
    this.keys = keys;
    this.active = active;
    this.clock = clock;
  }

  public static AuthUserPrincipalIssuer load(Path directory, String activeKid, Clock clock) {
    try {
      if (directory == null || activeKid == null || activeKid.isBlank() || clock == null
          || !Files.isDirectory(directory)) throw new IllegalArgumentException("invalid Auth principal key configuration");
      List<Path> files;
      try (var listed = Files.list(directory)) {
        files = listed.filter(path -> path.getFileName().toString().endsWith(".pem"))
            .sorted(Comparator.comparing(path -> path.getFileName().toString())).toList();
      }
      if (files.size() != 2 || !files.stream().map(path -> path.getFileName().toString())
          .collect(java.util.stream.Collectors.toSet()).equals(java.util.Set.of("current.pem", "next.pem"))) {
        throw new IllegalArgumentException("Auth principal keyset must contain current.pem and next.pem");
      }
      List<RSAKey> loaded = files.stream().map(AuthUserPrincipalIssuer::readKey).toList();
      if (loaded.get(0).getModulus().equals(loaded.get(1).getModulus())) {
        throw new IllegalArgumentException("Auth principal keys must be distinct");
      }
      RSAKey selected = loaded.stream().filter(key -> activeKid.equals(key.getKeyID())).findFirst()
          .orElseThrow(() -> new IllegalArgumentException("active Auth principal kid is not in keyset"));
      return new AuthUserPrincipalIssuer(loaded, selected, clock);
    } catch (IOException invalid) {
      throw new IllegalArgumentException("unable to load Auth principal keyset", invalid);
    }
  }

  public String issue(String rpc, String requestId, String requestHash) {
    if (!ELIGIBILITY_RPC.equals(rpc) || requestId == null || !requestId.matches(
        "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
        || requestHash == null || !requestHash.matches("sha256:[0-9a-f]{64}")) {
      throw new IllegalArgumentException("invalid request binding");
    }
    Instant now = clock.instant();
    JWTClaimsSet claims = new JWTClaimsSet.Builder().issuer(ISSUER).subject(SUBJECT).audience(AUDIENCE)
        .claim("principal_type", "service").claim("rpc", rpc).claim("request_id", requestId)
        .claim("request_hash", requestHash).issueTime(Date.from(now)).notBeforeTime(Date.from(now))
        .expirationTime(Date.from(now.plusSeconds(MAX_LIFETIME_SECONDS))).jwtID(UUID.randomUUID().toString()).build();
    SignedJWT jwt = new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).keyID(active.getKeyID()).build(), claims);
    try {
      jwt.sign(new RSASSASigner(active.toPrivateKey()));
      return jwt.serialize();
    } catch (JOSEException failure) {
      throw new IllegalStateException("unable to sign Auth principal", failure);
    }
  }

  public String jwksJson() {
    try {
      List<JWK> publicKeys = keys.stream().map(key -> (JWK) key.toPublicJWK()).toList();
      return JSONObjectUtils.toJSONString(new JWKSet(publicKeys).toJSONObject());
    } catch (RuntimeException failure) {
      throw new IllegalStateException("unable to publish Auth principal keyset", failure);
    }
  }

  private static RSAKey readKey(Path file) {
    try {
      String filename = file.getFileName().toString();
      String kid = filename.substring(0, filename.length() - ".pem".length());
      if (!kid.matches("[A-Za-z0-9_-]{1,64}")) throw new IllegalArgumentException("invalid Auth principal key id");
      String pem = Files.readString(file).trim();
      if (!pem.startsWith("-----BEGIN PRIVATE KEY-----") || !pem.endsWith("-----END PRIVATE KEY-----")) {
        throw new IllegalArgumentException("Auth principal key must be unencrypted PKCS#8 PEM");
      }
      String encoded = pem.substring("-----BEGIN PRIVATE KEY-----".length(),
          pem.length() - "-----END PRIVATE KEY-----".length()).replaceAll("\\s", "");
      byte[] der = Base64.getDecoder().decode(encoded);
      RSAPrivateKey privateKey = (RSAPrivateKey) KeyFactory.getInstance("RSA")
          .generatePrivate(new PKCS8EncodedKeySpec(der));
      if (!(privateKey instanceof RSAPrivateCrtKey crt) || privateKey.getModulus().bitLength() < 2048) {
        throw new IllegalArgumentException("Auth principal RSA keys must be at least 2048 bits");
      }
      RSAPublicKey publicKey = (RSAPublicKey) KeyFactory.getInstance("RSA")
          .generatePublic(new RSAPublicKeySpec(crt.getModulus(), crt.getPublicExponent()));
      return new RSAKey.Builder(publicKey).privateKey(privateKey)
          .keyUse(KeyUse.SIGNATURE).algorithm(JWSAlgorithm.RS256).keyID(kid).build();
    } catch (Exception invalid) {
      if (invalid instanceof IllegalArgumentException argument) throw argument;
      throw new IllegalArgumentException("invalid Auth principal PKCS#8 RSA key", invalid);
    }
  }
}
