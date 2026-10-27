package voice.backend.auth.linkoauth;

import static org.assertj.core.api.Assertions.*;
import com.sun.net.httpserver.HttpServer;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.atomic.*;
import org.junit.jupiter.api.Test;

class LinkedOAuthHttpProviderTest {
  @Test void stubRequiresIndependentProviderS256AndStoredCallbackWithoutMockFallback() throws Exception {
    var server=HttpServer.create(new InetSocketAddress("127.0.0.1",0),0);
    var expected=new AtomicReference<>("v".repeat(43)); var tokenCalls=new AtomicInteger();
    server.createContext("/token",x->{
      tokenCalls.incrementAndGet();
      String body=new String(x.getRequestBody().readAllBytes(),StandardCharsets.UTF_8);
      Map<String,String> form=new HashMap<>();for(String part:body.split("&")){String[] kv=part.split("=",2);form.put(kv[0],URLDecoder.decode(kv[1],StandardCharsets.UTF_8));}
      boolean valid=LinkedOAuthService.s256(form.getOrDefault("code_verifier","")).equals(LinkedOAuthService.s256(expected.get()))
          && "https://api.example.test/api/v1/auth/linked-accounts/twitch/oauth-callback".equals(form.get("redirect_uri"))
          && "fixture-client".equals(form.get("client_id")) && "fixture-secret".equals(form.get("client_secret"));
      byte[] response=(valid?"{\"access_token\":\"stub-access\",\"refresh_token\":\"stub-refresh\"}":"{\"error_description\":\"SECRET-MUST-NOT-ESCAPE\"}").getBytes(StandardCharsets.UTF_8);
      x.sendResponseHeaders(valid?200:400,response.length);try(var out=x.getResponseBody()){out.write(response);}
    });
    server.createContext("/identity",x->{
      assertThat(x.getRequestHeaders().getFirst("Authorization")).isEqualTo("Bearer stub-access");
      byte[] body="{\"data\":[{\"id\":\"partner\",\"login\":\"streamer\",\"broadcaster_type\":\"partner\"}]}".getBytes(StandardCharsets.UTF_8);
      x.sendResponseHeaders(200,body.length);try(var out=x.getResponseBody()){out.write(body);}
    });
    server.start();
    try {
      String base="http://127.0.0.1:"+server.getAddress().getPort();
      var p=new LinkedOAuthSettings.Provider(true,true,"https://id.twitch.tv/oauth2/authorize",base+"/token",base+"/identity","fixture-client","fixture-secret",
          "https://api.example.test/api/v1/auth/linked-accounts/twitch/oauth-callback",Map.of("web","https://app.example.test/settings/oauth-return"));
      var settings=new LinkedOAuthSettings("fixture",Duration.ofSeconds(300),Duration.ofSeconds(60),"0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.US_ASCII),Map.of("twitch",p));
      var exchange=new LinkedOAuthHttpProvider(settings);
      assertThat(exchange.exchange("twitch","mock-code",p.callbackUri(),expected.get()).outcome()).isEqualTo("success");
      assertThat(exchange.exchange("twitch","real-code",p.callbackUri(),"wrong".repeat(10)).outcome()).isEqualTo("provider_error");
      assertThat(exchange.exchange("twitch","code","https://attacker.test/callback",expected.get()).outcome()).isEqualTo("unavailable");
      assertThat(exchange.exchange("youtube","code",p.callbackUri(),expected.get()).outcome()).isEqualTo("unavailable");
      assertThat(tokenCalls).hasValue(2);
    } finally {server.stop(0);}
  }
}
