package voice.backend.auth.linkoauth;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.*;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.http.*;
import org.springframework.web.bind.annotation.*;
import org.springframework.util.MultiValueMap;
import voice.backend.auth.service.*;

@RestController
@RequestMapping("/api/v1/auth/linked-accounts")
public final class LinkedOAuthController {
  private final AuthService auth;
  private final ObjectProvider<LinkedOAuthService> services;
  private final ObjectMapper json=new ObjectMapper();
  public LinkedOAuthController(AuthService auth,ObjectProvider<LinkedOAuthService> services) { this.auth=auth; this.services=services; }
  @ModelAttribute
  void securityHeaders(jakarta.servlet.http.HttpServletResponse response) {
    response.setHeader("Cache-Control","no-store");response.setHeader("Referrer-Policy","no-referrer");
  }
  public record StartRequest(@JsonProperty("client_surface")String surface,@JsonProperty("return_challenge")String challenge,
      @JsonProperty("return_challenge_method")String method) {}
  public record ConsumeRequest(@JsonProperty("attempt_id")UUID attempt,@JsonProperty("result_handle")String handle,
      @JsonProperty("return_verifier")String verifier) {}
  @PostMapping("/{provider:twitch|youtube}/link")
  public Map<String,Object> start(@RequestHeader(value="Authorization",required=false)String bearer,
      @PathVariable String provider,@RequestBody StartRequest body) {
    TokenClaims claims=auth.validate(bearer); var s=service().start(claims,provider,body.surface(),body.challenge(),body.method());
    return Map.of("attempt_id",s.attemptId(),"authorization_url",s.authorizationUrl(),"expires_at",s.expiresAt().toString());
  }
  @PostMapping("/{provider:twitch|youtube}/results/consume")
  public Map<String,String> consume(@RequestHeader(value="Authorization",required=false)String bearer,
      @PathVariable String provider,@RequestBody ConsumeRequest body) {
    TokenClaims claims=auth.validate(bearer);
    return Map.of("outcome",service().consume(claims,provider,body.attempt(),body.handle(),body.verifier()));
  }
  // Keep the authentication result local to the request, never in controller fields.
  @PostMapping("/{provider:twitch|youtube}/attempts/{id}/cancel")
  public ResponseEntity<Void> cancel(@RequestHeader(value="Authorization",required=false)String bearer,
      @PathVariable String provider,@PathVariable UUID id) {
    TokenClaims claims=auth.validate(bearer); service().cancel(claims,provider,id); return ResponseEntity.noContent().build();
  }
  @GetMapping(value="/{provider:twitch|youtube}/oauth-callback",produces=MediaType.TEXT_HTML_VALUE)
  public ResponseEntity<String> callback(@PathVariable String provider,@RequestParam MultiValueMap<String,String> params) {
    String target=null; boolean windows=false;
    try {
      if(params.size()>3 || params.keySet().stream().anyMatch(k->!Set.of("state","code","error").contains(k))
          || params.values().stream().anyMatch(v->v.size()!=1)
          || params.getFirst("state")==null || !params.getFirst("state").matches("[A-Za-z0-9_-]{43}")
          || (params.containsKey("code")==params.containsKey("error"))
          || params.values().stream().anyMatch(v->v.getFirst().isBlank() || v.getFirst().length()>4096))throw new LinkedOAuthException("invalid_link_attempt");
      var result=service().callback(provider,params.getFirst("state"),params.getFirst("code"),params.getFirst("error"));
      target=result.returnUri()+"?attempt_id="+result.attemptId()+"&result_handle="+result.resultHandle()+"#";
      windows="windows".equals(result.clientSurface());
    } catch(RuntimeException failure) { /* neutral page, never reflect or log provider query */ }
    String nonce=UUID.randomUUID().toString().replace("-","");
    String script="const rejected=location.hash.length>1;history.replaceState(null,'',location.pathname);";
    if(target!=null) {
      try {
        String encoded=json.writeValueAsString(target).replace("<","\\u003c");
        script+=windows?"if(!rejected){const a=document.createElement('a');a.textContent='Return to Voice';a.href="+encoded+";document.body.append(a);}":"if(!rejected)location.replace("+encoded+");";
      }
      catch(Exception impossible) { throw new IllegalStateException("unable to encode safe return"); }
    }
    String html="<!doctype html><html><head><meta charset=\"utf-8\"><title>Voice</title></head><body><p>Return to Voice Settings to continue.</p><script nonce=\""+nonce+"\">"+script+"</script></body></html>";
    return ResponseEntity.ok().header("Cache-Control","no-store").header("Referrer-Policy","no-referrer")
        .header("X-Content-Type-Options","nosniff").header("Content-Security-Policy","default-src 'none'; script-src 'nonce-"+nonce+"'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
        .contentType(MediaType.TEXT_HTML).body(html);
  }
  private LinkedOAuthService service() { var s=services.getIfAvailable(); if(s==null)throw new LinkedOAuthException("oauth_unavailable"); return s; }
  @ExceptionHandler(LinkedOAuthException.class)
  ResponseEntity<Map<String,String>> error(LinkedOAuthException e) {
    int status=switch(e.getMessage()) { case "oauth_unavailable" -> 503; case "result_consumed","linked_account_profile_conflict" ->409; case "result_expired"->410; default->400; };
    return ResponseEntity.status(status).header("Cache-Control","no-store").body(Map.of("error",e.getMessage()));
  }
  @ExceptionHandler(AuthException.class)
  ResponseEntity<Map<String,String>> authError() { return ResponseEntity.status(401).body(Map.of("error","invalid_token")); }
  @ExceptionHandler({org.springframework.http.converter.HttpMessageNotReadableException.class,org.springframework.web.method.annotation.MethodArgumentTypeMismatchException.class})
  ResponseEntity<Map<String,String>> malformed() { return ResponseEntity.badRequest().body(Map.of("error","invalid_link_attempt")); }
}
