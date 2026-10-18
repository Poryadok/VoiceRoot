package voice.backend.auth.reportaudience;

import static org.assertj.core.api.Assertions.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.charset.StandardCharsets;
import java.security.*;
import java.security.interfaces.RSAPublicKey;
import java.time.*;
import java.util.*;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;

class ReportAudienceVerifierTest {
  static final String RPC="/voice.auth.v1.AuthService/ClassifyReportAudienceAccounts";
  static final String HASH="sha256:"+"0".repeat(64);
  static final ObjectMapper JSON=new ObjectMapper();
  static KeyPair pair()throws Exception {var generator=KeyPairGenerator.getInstance("RSA");generator.initialize(2048);return generator.generateKeyPair();}
  static Map<String,Object> claims(Instant now) {var claims=new LinkedHashMap<String,Object>();claims.put("iss","user");claims.put("sub","service:user");claims.put("aud","auth");claims.put("principal_type","service");claims.put("rpc",RPC);claims.put("request_id","audience-request");claims.put("request_hash",HASH);claims.put("iat",now.getEpochSecond());claims.put("nbf",now.getEpochSecond());claims.put("exp",now.plusSeconds(30).getEpochSecond());claims.put("jti",UUID.randomUUID().toString());return claims;}
  static String token(KeyPair pair,Map<String,Object> claims)throws Exception {
    var base=Base64.getUrlEncoder().withoutPadding();
    String unsigned=base.encodeToString(JSON.writeValueAsBytes(Map.of("alg","RS256","kid","current")))+"."+base.encodeToString(JSON.writeValueAsBytes(claims));
    var signature=Signature.getInstance("SHA256withRSA");signature.initSign(pair.getPrivate());signature.update(unsigned.getBytes(StandardCharsets.US_ASCII));
    return unsigned+"."+base.encodeToString(signature.sign());
  }

  @Test void acceptsOnlyExactUserServiceRequestAndRejectsReplay()throws Exception {
    var key=pair();var now=Instant.parse("2026-10-02T12:00:00Z");var replay=new HashSet<String>();
    var verifier=new ReportAudienceVerifier((issuer,kid)->(RSAPublicKey)key.getPublic(),(issuer,jti,expiry)->{if(!replay.add(jti))throw new IllegalArgumentException();},Clock.fixed(now,ZoneOffset.UTC));
    var credential=token(key,claims(now));
    var verified=verifier.verify(credential,RPC,"audience-request",HASH);
    assertThat(verified.expires()).isEqualTo(now.plusSeconds(30));
    assertThatThrownBy(()->verifier.verify(credential,RPC,"audience-request",HASH)).isInstanceOf(RuntimeException.class);
    for(String field:List.of("iss","sub","aud","principal_type","rpc","request_id","request_hash","account_id","profile_id","session_epoch")) {
      var changed=claims(now);changed.put(field,field.equals("session_epoch")?1:"wrong");
      String bad=token(key,changed);
      assertThatThrownBy(()->verifier.verify(bad,RPC,"audience-request",HASH)).isInstanceOf(RuntimeException.class);
    }
  }

  @Test void rejectsExpirationDuringReplayAndDoesNotReleaseAnExpiredRead()throws Exception {
    var key=pair();var time=new AtomicReference<>(Instant.parse("2026-10-02T12:00:00Z"));
    Clock clock=new Clock(){public ZoneId getZone(){return ZoneOffset.UTC;}public Clock withZone(ZoneId z){return this;}public Instant instant(){return time.get();}};
    var calls=new AtomicInteger();
    var verifier=new ReportAudienceVerifier((issuer,kid)->(RSAPublicKey)key.getPublic(),(issuer,jti,expiry)->{calls.incrementAndGet();time.set(expiry);},clock);
    String credential=token(key,claims(time.get()));
    assertThatThrownBy(()->verifier.verify(credential,RPC,"audience-request",HASH)).isInstanceOf(RuntimeException.class);
    assertThat(calls).hasValue(1);
    var verified=new ReportAudienceVerifier.Verified(RPC,HASH,time.get().plusSeconds(1),clock);
    verified.require(RPC,HASH);
    time.set(time.get().plusSeconds(1));
    assertThatThrownBy(()->verified.require(RPC,HASH)).isInstanceOf(RuntimeException.class);
  }
}
