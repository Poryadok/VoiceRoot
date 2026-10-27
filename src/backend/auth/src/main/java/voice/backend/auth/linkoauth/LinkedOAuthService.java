package voice.backend.auth.linkoauth;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.security.*;
import java.sql.Timestamp;
import java.time.*;
import java.util.*;
import org.springframework.jdbc.core.namedparam.*;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.repository.*;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.TokenClaims;
import voice.backend.auth.userdb.ProfileSwitchValidator;

/** PostgreSQL one-time transport. External network I/O never holds an attempt lock. */
public final class LinkedOAuthService {
  public record Start(UUID attemptId, String authorizationUrl, Instant expiresAt) {
    @Override public String toString() { return "LinkedOAuthStart[redacted]"; }
  }
  public record Return(UUID attemptId, String resultHandle, String returnUri, String clientSurface) {
    @Override public String toString() { return "LinkedOAuthReturn[redacted]"; }
  }
  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate tx;
  private final LinkedOAuthSettings settings;
  private final TokenBlacklist blacklist;
  private final ProfileSwitchValidator profiles;
  private final Clock clock;
  private final LinkedOAuthProvider exchange;
  private final LinkedOAuthVault vault;
  private final LinkedIdentityRepository identities;
  private final ObjectMapper json = new ObjectMapper();
  private final SecureRandom random = new SecureRandom();

  public LinkedOAuthService(NamedParameterJdbcTemplate jdbc, TransactionTemplate tx, LinkedOAuthSettings settings,
      TokenBlacklist blacklist, ProfileSwitchValidator profiles, Clock clock, LinkedOAuthProvider exchange) {
    this.jdbc=jdbc; this.tx=tx; this.settings=settings; this.blacklist=blacklist; this.profiles=profiles;
    this.clock=clock; this.exchange=exchange; this.vault=new LinkedOAuthVault(settings.encryptionKey());
    this.identities=new JdbcLinkedIdentityRepository(jdbc);
  }
  public Start start(TokenClaims claims, String provider, String surface, String challenge, String method) {
    if (!"S256".equals(method) || !opaque(challenge)) throw invalid();
    var p = provider(provider);
    String returnUri = p.returns().get(surface);
    if (returnUri == null) throw new LinkedOAuthException("oauth_unavailable");
    return tx.execute(status -> {
      authority(claims);
      UUID id=UUID.randomUUID(); String state=randomToken(), verifier=randomToken();
      Instant expires=earlier(clock.instant().plus(settings.attemptTtl()), claims.expiresAt());
      fresh(expires);
      jdbc.update("""
          UPDATE linked_oauth_attempts SET status='superseded',provider_secret=NULL,result_secret=NULL
          WHERE account_id=:account AND profile_id=:profile AND original_jti=:jti AND provider=:provider
            AND status IN ('pending','processing','ready')
          """, Map.of("account", UUID.fromString(claims.userId()), "profile", UUID.fromString(claims.profileId()), "jti", claims.jti(), "provider", provider));
      jdbc.update("""
          INSERT INTO linked_oauth_attempts(id,account_id,profile_id,provider,original_jti,original_epoch,original_exp,
            subscription_tier,registration_revision,callback_uri,return_uri,client_surface,state_hash,return_challenge,
            provider_secret,expires_at,status)
          VALUES (:id,:account,:profile,:provider,:jti,:epoch,:exp,:tier,:revision,:callback,:returnUri,:surface,:state,
            :challenge,:secret,:expires,'pending')
          """, new MapSqlParameterSource().addValue("id",id).addValue("account",UUID.fromString(claims.userId()))
          .addValue("profile",UUID.fromString(claims.profileId())).addValue("provider",provider).addValue("jti",claims.jti())
          .addValue("epoch",claims.sessionEpoch()).addValue("exp",Timestamp.from(claims.expiresAt()))
          .addValue("tier",claims.subscriptionTier()==null?"free":claims.subscriptionTier()).addValue("revision",revision(provider,p))
          .addValue("callback",p.callbackUri()).addValue("returnUri",returnUri).addValue("surface",surface)
          .addValue("state",hash(state)).addValue("challenge",challenge)
          .addValue("secret",vault.seal(id,"provider",verifier.getBytes(StandardCharsets.US_ASCII)))
          .addValue("expires",Timestamp.from(expires)));
      String scope=provider.equals("twitch")?"user:read:email":"https://www.googleapis.com/auth/youtube.readonly";
      String url=p.authorizationUrl()+"?response_type=code&client_id="+url(p.clientId())+"&redirect_uri="+url(p.callbackUri())
          +"&scope="+url(scope)+"&state="+state+"&code_challenge_method=S256&code_challenge="+s256(verifier);
      return new Start(id,url,expires);
    });
  }
  public Return callback(String provider, String state, String code, String error) {
    if (!opaque(state) || ((code==null)==(error==null)) || (code!=null && (code.isBlank() || code.length()>4096))
        || (error!=null && (error.isBlank() || error.length()>256))) throw invalid();
    Row claimed=tx.execute(status -> {
      Row row=byState(state); lockAccount(row.account()); row=locked(row.id());
      if (!provider.equals(row.text("provider")) || !"pending".equals(row.text("status"))) throw invalid();
      valid(row); authority(row.claims()); valid(row);
      jdbc.update("UPDATE linked_oauth_attempts SET status='processing' WHERE id=:id",Map.of("id",row.id()));
      return row;
    });
    LinkedOAuthProvider.Result result;
    if (error!=null) result=new LinkedOAuthProvider.Result(error.equals("access_denied")?"cancelled":"provider_error",null,null,null,null);
    else {
      // Profile authority and the transaction commit may outlive the attempt deadline.
      fresh(claimed.time("expires_at")); fresh(claimed.time("original_exp"));
      try {
        String verifier=new String(vault.open(claimed.id(),"provider",claimed.bytes("provider_secret")),StandardCharsets.US_ASCII);
        result=exchange.exchange(provider,code,claimed.text("callback_uri"),verifier);
        if (result==null || !Set.of("success","denied","provider_error","unavailable").contains(result.outcome()))
          result=new LinkedOAuthProvider.Result("provider_error",null,null,null,null);
      } catch (RuntimeException failure) { result=new LinkedOAuthProvider.Result("provider_error",null,null,null,null); }
    }
    final var staged=result;
    return tx.execute(status -> {
      lockAccount(claimed.account()); Row row=locked(claimed.id());
      if (!"processing".equals(row.text("status"))) throw invalid();
      valid(row); authority(row.claims());
      String handle=randomToken(); Instant expires=earlier(clock.instant().plus(settings.resultTtl()),row.time("expires_at")); fresh(expires);
      byte[] secret=null;
      if (staged.outcome().equals("success")) {
        if (staged.externalId()==null || staged.externalId().isBlank() || staged.accessToken()==null || staged.accessToken().isBlank()) throw invalid();
        try { secret=vault.seal(row.id(),"result",json.writeValueAsBytes(staged)); }
        catch (Exception e) { throw invalid(); }
      }
      jdbc.update("""
          UPDATE linked_oauth_attempts SET status='ready',provider_secret=NULL,result_hash=:hash,
            result_expires_at=:expires,result_secret=:secret,outcome=:outcome WHERE id=:id
          """,new MapSqlParameterSource().addValue("hash",hash(handle)).addValue("expires",Timestamp.from(expires))
          .addValue("secret",secret).addValue("outcome",staged.outcome()).addValue("id",row.id()));
      return new Return(row.id(),handle,row.text("return_uri"),row.text("client_surface"));
    });
  }
  public String consume(TokenClaims claims, String provider, UUID id, String handle, String verifier) {
    if (id==null || !opaque(handle) || verifier==null || !verifier.matches("[A-Za-z0-9._~-]{43,128}")) throw invalid();
    return tx.execute(status -> {
      lockAccount(UUID.fromString(claims.userId())); Row row=locked(id);
      bound(row,claims,provider);
      if (!hash(handle).equals(row.text("result_hash")) || !s256(verifier).equals(row.text("return_challenge"))) throw invalid();
      if ("consumed".equals(row.text("status"))) throw new LinkedOAuthException("result_consumed");
      valid(row); fresh(row.time("result_expires_at"));
      if (!"ready".equals(row.text("status"))) throw invalid();
      authority(row.claims());
      String outcome=row.text("outcome");
      if ("success".equals(outcome)) {
        LinkedOAuthProvider.Result result;
        try { result=json.readValue(vault.open(row.id(),"result",row.bytes("result_secret")),LinkedOAuthProvider.Result.class); }
        catch (Exception e) { throw invalid(); }
        try {
          identities.linkActive(row.account(),row.profile(),provider,result.externalId(),result.login(),
              sourceToken(row,result.accessToken(),"access"),sourceToken(row,result.refreshToken(),"refresh"));
        } catch (LinkedIdentityProfileConflictException | org.springframework.dao.DuplicateKeyException conflict) { throw new LinkedOAuthException("linked_account_profile_conflict"); }
      }
      // Recheck after source/sync mutation too: any failure rolls back the entire Auth transaction.
      authority(row.claims()); valid(row); fresh(row.time("result_expires_at"));
      jdbc.update("UPDATE linked_oauth_attempts SET status='consumed',provider_secret=NULL,result_secret=NULL WHERE id=:id",Map.of("id",id));
      return outcome;
    });
  }
  public void cancel(TokenClaims claims, String provider, UUID id) {
    tx.executeWithoutResult(status -> {
      lockAccount(UUID.fromString(claims.userId())); Row row=locked(id); bound(row,claims,provider); authority(row.claims());
      if ("consumed".equals(row.text("status"))) throw new LinkedOAuthException("result_consumed");
      jdbc.update("UPDATE linked_oauth_attempts SET status='cancelled',provider_secret=NULL,result_secret=NULL WHERE id=:id",Map.of("id",id));
    });
  }
  public void cleanup() {
    new LinkedOAuthCleanup(jdbc,clock).cleanup();
  }
  private byte[] sourceToken(Row row, String token, String purpose) {
    if (token==null) return null;
    return LinkedOAuthSourceTokens.seal(vault,row.account(),row.profile(),row.text("provider"),purpose,token);
  }
  private void valid(Row row) {
    fresh(row.time("expires_at")); fresh(row.time("original_exp"));
    var p=provider(row.text("provider"));
    if (!revision(row.text("provider"),p).equals(row.text("registration_revision"))
        || !p.callbackUri().equals(row.text("callback_uri")) || !row.text("return_uri").equals(p.returns().get(row.text("client_surface")))) throw invalid();
  }
  private void bound(Row row, TokenClaims c, String provider) {
    if (!row.account().toString().equals(c.userId()) || !row.profile().toString().equals(c.profileId())
        || !row.text("original_jti").equals(c.jti()) || row.number("original_epoch")!=c.sessionEpoch()
        || !row.time("original_exp").equals(c.expiresAt()) || !row.text("provider").equals(provider)) throw invalid();
  }
  private void authority(TokenClaims c) {
    if (c==null || c.jti()==null || c.jti().isBlank() || c.sessionEpoch()<=0 || !"regular".equals(c.normalizedAccountType())) throw invalid();
    fresh(c.expiresAt()); UUID account=UUID.fromString(c.userId()), profile=UUID.fromString(c.profileId());
    Map<String,Object> a=lockAccount(account);
    try {
      if (blacklist.isRevoked(c.jti()) || !"active".equals(a.get("status")) || !"regular".equals(a.get("type"))
          || a.get("deleted_at")!=null || Boolean.TRUE.equals(a.get("regular_email_verification_pending"))
          || ((Number)a.get("session_epoch")).longValue()!=c.sessionEpoch()) throw invalid();
      profiles.validateOwnedSwitchable(account,profile,profile,c.subscriptionTier());
    } catch (RuntimeException unavailable) { throw invalid(); }
    fresh(c.expiresAt());
  }
  private Map<String,Object> lockAccount(UUID account) {
    var rows=jdbc.queryForList("SELECT status,type,deleted_at,session_epoch,regular_email_verification_pending FROM accounts WHERE id=:id FOR UPDATE",Map.of("id",account));
    if (rows.size()!=1) throw invalid(); return rows.getFirst();
  }
  private LinkedOAuthSettings.Provider provider(String provider) {
    var p=settings.providers().get(provider);
    // YouTube has no canonical membership+verified-channel evidence mapping yet.
    if (p==null || !p.enabled() || !p.s256() || "youtube".equals(provider)) throw new LinkedOAuthException("oauth_unavailable");
    return p;
  }
  private String revision(String provider, LinkedOAuthSettings.Provider p) {
    return hash(String.join("\n",settings.revision(),provider,p.authorizationUrl(),p.tokenUrl(),p.identityUrl(),p.clientId(),p.clientSecret(),
        p.callbackUri(),new TreeMap<>(p.returns()).toString(),Base64.getEncoder().encodeToString(settings.encryptionKey())));
  }
  private Row byState(String state) { return row("SELECT * FROM linked_oauth_attempts WHERE state_hash=:value",hash(state)); }
  private Row locked(UUID id) { if (id==null) throw invalid(); return row("SELECT * FROM linked_oauth_attempts WHERE id=:value FOR UPDATE",id); }
  private Row row(String sql,Object value) { var rows=jdbc.queryForList(sql,Map.of("value",value)); if(rows.size()!=1)throw invalid(); return new Row(rows.getFirst()); }
  private record Row(Map<String,Object> values) {
    String text(String k) { return (String)values.get(k); }
    byte[] bytes(String k) { return (byte[])values.get(k); }
    long number(String k) { return ((Number)values.get(k)).longValue(); }
    Instant time(String k) { Object v=values.get(k); return v==null?null:((Timestamp)v).toInstant(); }
    UUID id() { return (UUID)values.get("id"); }
    UUID account() { return (UUID)values.get("account_id"); }
    UUID profile() { return (UUID)values.get("profile_id"); }
    TokenClaims claims() { return new TokenClaims(account().toString(),profile().toString(),List.of(),text("subscription_tier"),time("original_exp"),text("original_jti"),"regular",number("original_epoch")); }
  }
  private void fresh(Instant exp) { if(exp==null || !clock.instant().isBefore(exp))throw new LinkedOAuthException("result_expired"); }
  private String randomToken() { byte[] b=new byte[32]; random.nextBytes(b); return Base64.getUrlEncoder().withoutPadding().encodeToString(b); }
  private static boolean opaque(String s) { return s!=null && s.matches("[A-Za-z0-9_-]{43}"); }
  private static Instant earlier(Instant a,Instant b) { return a.isBefore(b)?a:b; }
  static String url(String s) { return URLEncoder.encode(s,StandardCharsets.UTF_8); }
  static String s256(String s) { return Base64.getUrlEncoder().withoutPadding().encodeToString(digest(s)); }
  private static String hash(String s) { return HexFormat.of().formatHex(digest(s)); }
  private static byte[] digest(String s) { try { return MessageDigest.getInstance("SHA-256").digest(s.getBytes(StandardCharsets.UTF_8)); } catch(Exception e) { throw new IllegalStateException("SHA-256 unavailable"); } }
  private static LinkedOAuthException invalid() { return new LinkedOAuthException("invalid_link_attempt"); }
}
