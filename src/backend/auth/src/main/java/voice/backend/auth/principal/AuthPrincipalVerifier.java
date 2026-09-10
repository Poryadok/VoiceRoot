package voice.backend.auth.principal;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Status;
import java.nio.charset.StandardCharsets;
import java.security.Signature;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.time.Instant;
import java.util.Base64;
import java.util.Objects;
import java.util.Set;
import java.util.UUID;

/** Phase-0 consumer for Auth's ownership-proof methods. Never accepts client access JWTs. */
public final class AuthPrincipalVerifier {
  @FunctionalInterface public interface KeyResolver { RSAPublicKey resolve(String issuer, String kid); }
  @FunctionalInterface public interface EpochLookup { long requireFloor(UUID accountId); }
  @FunctionalInterface public interface ReplayGuard { void record(String issuer, String jti, Instant expires); }

  static final ObjectMapper JSON = new ObjectMapper()
      .enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
      .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);
  private final KeyResolver keys;
  private final EpochLookup epochs;
  private final ReplayGuard replay;
  private final Clock clock;

  public AuthPrincipalVerifier(KeyResolver keys, EpochLookup epochs, ReplayGuard replay, Clock clock) {
    this.keys = Objects.requireNonNull(keys);
    this.epochs = Objects.requireNonNull(epochs);
    this.replay = Objects.requireNonNull(replay);
    this.clock = Objects.requireNonNull(clock);
  }

  public VerifiedPrincipal verify(String token, String rpc, String requestId, String requestHash) {
    final VerifiedPrincipal principal;
    final String jti;
    final Instant expires;
    try {
      if (token == null || token.length() > 32768 || requestId == null || requestId.isBlank()
          || requestHash == null || !requestHash.matches("sha256:[0-9a-f]{64}")) throw invalid();
      String[] parts = token.split("\\.", -1);
      if (parts.length != 3) throw invalid();
      var decoder = Base64.getUrlDecoder();
      JsonNode header = JSON.readTree(decoder.decode(parts[0]));
      if (!"RS256".equals(string(header, "alg")) || header.has("crit")) throw invalid();
      String kid = string(header, "kid");
      if (!kid.matches("[A-Za-z0-9][A-Za-z0-9._-]{0,127}")) throw invalid();
      JsonNode claims = JSON.readTree(decoder.decode(parts[1]));
      String issuer = string(claims, "iss");
      if (!Set.of("gateway", "space").contains(issuer)) throw invalid();
      RSAPublicKey key = keys.resolve(issuer, kid);
      if (key == null || key.getModulus().bitLength() < 2048) throw invalid();
      var signature = Signature.getInstance("SHA256withRSA");
      signature.initVerify(key);
      signature.update((parts[0] + "." + parts[1]).getBytes(StandardCharsets.US_ASCII));
      if (!signature.verify(decoder.decode(parts[2]))) throw invalid();
      if (!"auth".equals(string(claims, "aud")) || !rpc.equals(string(claims, "rpc"))
          || !requestId.equals(string(claims, "request_id"))
          || !requestHash.equals(string(claims, "request_hash"))) throw invalid();
      Instant issued = Instant.ofEpochSecond(integer(claims, "iat"));
      Instant notBefore = Instant.ofEpochSecond(integer(claims, "nbf"));
      expires = Instant.ofEpochSecond(integer(claims, "exp"));
      Instant now = clock.instant();
      if (notBefore.isBefore(issued) || notBefore.isAfter(now.plusSeconds(5))
          || issued.isAfter(now.plusSeconds(5)) || !expires.isAfter(now)
          || !expires.isAfter(notBefore) || expires.isAfter(issued.plusSeconds(30))) throw invalid();
      jti = string(claims, "jti");
      String kind = string(claims, "principal_type");
      String subject = string(claims, "sub");
      if (VerifiedPrincipal.SERVICE.equals(kind)) {
        if (!subject.equals("service:" + issuer)) throw invalid();
        principal = new VerifiedPrincipal(kind, issuer, null, null, 0);
      } else if (VerifiedPrincipal.DELEGATED_USER.equals(kind)) {
        if (!"gateway".equals(issuer) || !subject.equals(string(claims, "account_id"))) throw invalid();
        UUID account = uuid(subject);
        UUID profile = uuid(string(claims, "profile_id"));
        long epoch = integer(claims, "session_epoch");
        long floor = epochs.requireFloor(account);
        if (floor <= 0 || epoch < floor) throw invalid();
        principal = new VerifiedPrincipal(kind, issuer, account, profile, epoch);
      } else {
        throw invalid();
      }
    } catch (Exception ex) {
      // Do not expose credentials, identities, network endpoints or parser details.
      throw Status.UNAUTHENTICATED.withDescription("invalid principal").asRuntimeException();
    }
    boolean allowed = AuthPrincipalServerInterceptor.ISSUE_RPC.equals(rpc)
        ? principal.kind().equals(VerifiedPrincipal.DELEGATED_USER) && principal.issuer().equals("gateway")
        : (AuthPrincipalServerInterceptor.CONSUME_RPC.equals(rpc) || AuthPrincipalServerInterceptor.LOOKUP_RPC.equals(rpc))
            && principal.kind().equals(VerifiedPrincipal.SERVICE) && principal.issuer().equals("space");
    if (!allowed) throw Status.PERMISSION_DENIED.withDescription("principal caller not permitted").asRuntimeException();
    try {
      if (!expires.isAfter(clock.instant())) throw invalid();
      replay.record(principal.issuer(), jti, expires);
      if (!expires.isAfter(clock.instant())) throw invalid();
    } catch (Exception ex) {
      throw Status.UNAUTHENTICATED.withDescription("invalid principal").asRuntimeException();
    }
    return principal;
  }

  static String string(JsonNode value, String field) {
    JsonNode node = value == null ? null : value.get(field);
    if (node == null || !node.isTextual() || node.textValue().isBlank()) throw invalid();
    return node.textValue();
  }

  private static long integer(JsonNode value, String field) {
    JsonNode node = value.get(field);
    if (node == null || !node.isIntegralNumber() || !node.canConvertToLong() || node.longValue() <= 0) throw invalid();
    return node.longValue();
  }

  private static UUID uuid(String value) {
    UUID id = UUID.fromString(value);
    if (!id.toString().equals(value)) throw invalid();
    return id;
  }

  private static IllegalArgumentException invalid() {
    return new IllegalArgumentException("invalid principal");
  }
}
