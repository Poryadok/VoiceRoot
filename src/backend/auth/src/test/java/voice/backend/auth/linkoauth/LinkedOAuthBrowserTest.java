package voice.backend.auth.linkoauth;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;
import com.sun.net.httpserver.HttpServer;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;
import org.junit.jupiter.api.*;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.util.LinkedMultiValueMap;
import voice.backend.auth.service.AuthService;

/** Optional real Chromium boundary. CI without a local browser still runs controller/crypto/store suites. */
class LinkedOAuthBrowserTest {
  @Test void realBrowserRejectsAndScrubsCredentialFragmentsBeforeWebOrWindowsHandoff() throws Exception {
    Path chrome=Path.of("C:/Program Files/Google/Chrome/Application/chrome.exe");
    Assumptions.assumeTrue(Files.isExecutable(chrome),"local Chromium fixture unavailable");
    var service=mock(LinkedOAuthService.class);
    @SuppressWarnings("unchecked") ObjectProvider<LinkedOAuthService> provider=mock(ObjectProvider.class);
    when(provider.getIfAvailable()).thenReturn(service);
    var controller=new LinkedOAuthController(mock(AuthService.class),provider);
    var server=HttpServer.create(new InetSocketAddress("127.0.0.1",0),0);
    var requests=new CopyOnWriteArrayList<String>();var returned=new AtomicInteger();
    String base="http://127.0.0.1:"+server.getAddress().getPort();
    UUID id=UUID.randomUUID();String handle="h".repeat(43),state="s".repeat(43);
    server.createContext("/callback",x->{
      requests.add(x.getRequestURI().toASCIIString());
      var params=new LinkedMultiValueMap<String,String>();params.add("state",state);params.add("code","provider-secret-code");
      var response=controller.callback("twitch",params);
      String html=response.getBody().replace("</script>","document.body.dataset.url=location.href;</script>");
      response.getHeaders().forEach((key,values)->x.getResponseHeaders().put(key,values));
      byte[] data=html.getBytes(StandardCharsets.UTF_8);x.sendResponseHeaders(200,data.length);try(var out=x.getResponseBody()){out.write(data);}
    });
    server.createContext("/return",x->{
      returned.incrementAndGet();requests.add(x.getRequestURI().toASCIIString());
      byte[] data="<!doctype html><p>RETURNED</p><script>document.body.dataset.url=location.href;</script>".getBytes(StandardCharsets.UTF_8);
      x.sendResponseHeaders(200,data.length);try(var out=x.getResponseBody()){out.write(data);}
    });server.start();
    try {
      when(service.callback("twitch",state,"provider-secret-code",null)).thenReturn(new LinkedOAuthService.Return(id,handle,base+"/return","web"));
      String rejected=browse(chrome,base+"/callback?state="+state+"&code=provider-secret-code#credential-fragment");
      assertThat(rejected).contains("data-url=\""+base+"/callback\"").doesNotContain("credential-fragment","provider-secret-code",state);
      assertThat(returned).hasValue(0);
      String success=browse(chrome,base+"/callback?state="+state+"&code=provider-secret-code");
      assertThat(success).contains("RETURNED","result_handle="+handle).doesNotContain("provider-secret-code",state);
      assertThat(returned).hasValue(1);
      when(service.callback("twitch",state,"provider-secret-code",null)).thenReturn(new LinkedOAuthService.Return(id,handle,"voice://settings/oauth-return","windows"));
      String nativeRejected=browse(chrome,base+"/callback#credential-fragment");
      assertThat(nativeRejected).doesNotContain("<a ","credential-fragment");
      String nativeReady=browse(chrome,base+"/callback");
      assertThat(nativeReady).contains("<a href=\"voice://settings/oauth-return?attempt_id="+id,"result_handle="+handle+"#\"");
      assertThat(requests).noneMatch(uri->uri.contains("#") || uri.contains("credential-fragment"));
    } finally {server.stop(0);}
  }
  private String browse(Path chrome,String url) throws Exception {
    Path directory=Path.of("../../../tmp/anton110/browser").toAbsolutePath().normalize();Files.createDirectories(directory);
    Path profile=Files.createTempDirectory(directory,"chromium-");Path output=profile.resolve("stdout.txt");Path error=profile.resolve("stderr.txt");
    Process p=new ProcessBuilder(chrome.toString(),"--headless=new","--no-sandbox","--disable-gpu","--no-first-run","--no-default-browser-check",
        "--disable-background-networking","--disable-extensions","--user-data-dir="+profile,"--virtual-time-budget=1500","--dump-dom",url)
        .redirectOutput(output.toFile()).redirectError(error.toFile()).start();
    try {assertThat(p.waitFor(20,TimeUnit.SECONDS)).isTrue();assertThat(p.exitValue()).isZero();return Files.readString(output);}
    finally {if(p.isAlive()){p.destroyForcibly();p.waitFor(10,TimeUnit.SECONDS);}}
  }
}
