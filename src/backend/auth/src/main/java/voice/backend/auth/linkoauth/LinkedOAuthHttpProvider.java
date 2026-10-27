package voice.backend.auth.linkoauth;

import com.fasterxml.jackson.databind.*;
import java.net.URI;
import java.net.http.*;
import java.nio.charset.StandardCharsets;
import java.time.Duration;

/** Strict S256 server exchange with no mock-token or missing-configuration fallback. */
final class LinkedOAuthHttpProvider implements LinkedOAuthProvider {
  private final LinkedOAuthSettings settings;
  private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).followRedirects(HttpClient.Redirect.NEVER).build();
  private final ObjectMapper json=new ObjectMapper();
  LinkedOAuthHttpProvider(LinkedOAuthSettings settings) { this.settings=settings; }
  @Override public Result exchange(String provider,String code,String callback,String verifier) {
    if(!provider.equals("twitch"))return outcome("unavailable");
    var p=settings.providers().get(provider);
    if(p==null || !p.enabled() || !p.s256() || !p.callbackUri().equals(callback)
        || p.clientId()==null || p.clientId().isBlank() || p.clientSecret()==null || p.clientSecret().isBlank())return outcome("unavailable");
    try {
      String form="grant_type=authorization_code&client_id="+enc(p.clientId())+"&client_secret="+enc(p.clientSecret())
          +"&code="+enc(code)+"&redirect_uri="+enc(callback)+"&code_verifier="+enc(verifier);
      JsonNode tokens=send(HttpRequest.newBuilder(URI.create(p.tokenUrl())).header("Content-Type","application/x-www-form-urlencoded")
          .timeout(Duration.ofSeconds(5)).POST(HttpRequest.BodyPublishers.ofString(form)).build());
      String access=token(tokens,"access_token",true),refresh=token(tokens,"refresh_token",false);
      JsonNode users=send(HttpRequest.newBuilder(URI.create(p.identityUrl())).header("Authorization","Bearer "+access)
          .header("Client-Id",p.clientId()).timeout(Duration.ofSeconds(5)).GET().build()).path("data");
      if(!users.isArray() || users.size()!=1) return outcome("provider_error");
      JsonNode user=users.get(0);
      if(!"partner".equals(user.path("broadcaster_type").asText()))return outcome("denied");
      String id=field(user,"id",256),login=field(user,"login",256);
      return new Result("success",id,login,access,refresh);
    } catch(InterruptedException interrupted) { Thread.currentThread().interrupt(); return outcome("provider_error"); }
    catch(Exception failure) { return outcome("provider_error"); }
  }
  private JsonNode send(HttpRequest request) throws Exception {
    var response=http.send(request,HttpResponse.BodyHandlers.ofInputStream());
    try(var body=response.body()) {
      if(response.statusCode()!=200)throw new IllegalArgumentException("provider_error");
      byte[] data=body.readNBytes(65537); if(data.length>65536)throw new IllegalArgumentException("provider_error");
      return json.readTree(data);
    }
  }
  private static String token(JsonNode n,String name,boolean required) {
    if(!required && (!n.has(name)||n.get(name).isNull()))return null;
    return field(n,name,8192);
  }
  private static String field(JsonNode n,String name,int max) {
    JsonNode v=n.get(name);
    if(v==null || !v.isTextual() || v.asText().isBlank() || v.asText().length()>max || v.asText().chars().anyMatch(c->c<32 || c==127))
      throw new IllegalArgumentException("provider_error");
    return v.asText();
  }
  private static Result outcome(String outcome) { return new Result(outcome,null,null,null,null); }
  private static String enc(String value) { return LinkedOAuthService.url(value); }
}
