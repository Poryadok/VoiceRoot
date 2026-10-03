package voice.backend.auth.authoritysource;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Context;
import io.grpc.Status;
import java.nio.charset.StandardCharsets;
import java.security.Signature;
import java.time.Clock;
import java.time.Instant;
import java.util.Base64;
import java.util.Set;
import voice.backend.auth.principal.AuthPrincipalVerifier;

/** This service principal is never an Auth user or ownership-proof principal. */
public final class AuthSourceVerifier {
  static final ObjectMapper JSON=new ObjectMapper().enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION).enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);
  static final String SNAPSHOT="/voice.authority.v1.AuthoritySourceService/ReadSnapshot";
  static final String REVISION="/voice.authority.v1.AuthoritySourceService/ReadRevision";
  public record Verified(String rpc,String hash,Instant expires,Clock clock) {
    static final Context.Key<Verified> CONTEXT=Context.key("auth-source-principal");
    public void require(String method,String requestHash) {
      if(!rpc.equals(method) || !hash.equals(requestHash) || !expires.isAfter(clock.instant()) || Context.current().isCancelled()) throw invalid();
    }
  }
  private final AuthPrincipalVerifier.KeyResolver keys;
  private final AuthPrincipalVerifier.ReplayGuard replay;
  private final Clock clock;
  public AuthSourceVerifier(AuthPrincipalVerifier.KeyResolver keys,AuthPrincipalVerifier.ReplayGuard replay,Clock clock) {this.keys=java.util.Objects.requireNonNull(keys);this.replay=java.util.Objects.requireNonNull(replay);this.clock=java.util.Objects.requireNonNull(clock);}

  public Verified verify(String token,String rpc,String requestId,String hash) {
    try {
      if(token==null || token.length()>32768 || requestId==null || requestId.isBlank() || hash==null || !hash.matches("sha256:[0-9a-f]{64}") || !Set.of(SNAPSHOT,REVISION).contains(rpc)) throw invalid();
      String[] parts=token.split("\\.",-1);if(parts.length!=3)throw invalid();
      var decoder=Base64.getUrlDecoder();var header=JSON.readTree(decoder.decode(parts[0]));var claims=JSON.readTree(decoder.decode(parts[1]));
      if(!text(header,"alg").equals("RS256") || header.has("crit") || !text(header,"kid").matches("[A-Za-z0-9][A-Za-z0-9._-]{0,127}"))throw invalid();
      if(!text(claims,"iss").equals("federation") || !text(claims,"sub").equals("service:federation") || !text(claims,"aud").equals("auth") || !text(claims,"principal_type").equals("service") || !text(claims,"rpc").equals(rpc) || !text(claims,"request_id").equals(requestId) || !text(claims,"request_hash").equals(hash)) throw invalid();
      for(String field:Set.of("account_id","profile_id")) if(claims.has(field) && (!claims.get(field).isTextual() || !claims.get(field).asText().isEmpty()))throw invalid();
      if(claims.has("session_epoch") && (!claims.get("session_epoch").isIntegralNumber() || !claims.get("session_epoch").canConvertToLong() || claims.get("session_epoch").longValue()!=0))throw invalid();
      var key=keys.resolve("federation",text(header,"kid"));if(key==null || key.getModulus().bitLength()<2048)throw invalid();
      var signature=Signature.getInstance("SHA256withRSA");signature.initVerify(key);signature.update((parts[0]+"."+parts[1]).getBytes(StandardCharsets.US_ASCII));if(!signature.verify(decoder.decode(parts[2])))throw invalid();
      Instant issued=Instant.ofEpochSecond(integer(claims,"iat")),notBefore=Instant.ofEpochSecond(integer(claims,"nbf")),expires=Instant.ofEpochSecond(integer(claims,"exp")),now=clock.instant();
      if(notBefore.isBefore(issued) || issued.isAfter(now.plusSeconds(5)) || notBefore.isAfter(now.plusSeconds(5)) || !expires.isAfter(notBefore) || !expires.isAfter(now) || expires.isAfter(issued.plusSeconds(30)))throw invalid();
      String jti=text(claims,"jti");var verified=new Verified(rpc,hash,expires,clock);
      verified.require(rpc,hash);replay.record("federation",jti,expires);verified.require(rpc,hash);
      return verified;
    } catch(Exception ex) {throw invalid();}
  }
  static String text(JsonNode node,String field) {var value=node==null?null:node.get(field);if(value==null || !value.isTextual() || value.asText().isBlank())throw invalid();return value.asText();}
  private static long integer(JsonNode node,String field) {var value=node.get(field);if(value==null || !value.isIntegralNumber() || !value.canConvertToLong() || value.longValue()<=0)throw invalid();return value.longValue();}
  static RuntimeException invalid(){return Status.UNAUTHENTICATED.withDescription("Auth source principal rejected").asRuntimeException();}
}
