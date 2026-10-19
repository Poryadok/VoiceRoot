package voice.backend.auth.principal;

import com.sun.net.httpserver.HttpsConfigurator;
import com.sun.net.httpserver.HttpsExchange;
import com.sun.net.httpserver.HttpsParameters;
import com.sun.net.httpserver.HttpsServer;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.Objects;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import javax.net.ssl.SSLPeerUnverifiedException;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.SmartLifecycle;
import org.springframework.core.env.Environment;
import org.springframework.stereotype.Component;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

/** Certificate-only local key publication, independent of all peer readiness. */
@Component
@ConditionalOnProperty(name="AUTH_PRINCIPAL_SIGNING_KEYS_DIR")
public final class AuthPrincipalJwksServer implements SmartLifecycle {
  private static final String PATH="/internal/principals/jwks";
  private final AuthUserPrincipalIssuer issuer;
  private final Environment environment;
  private HttpsServer server;
  private ExecutorService executor;
  private volatile boolean running;

  public AuthPrincipalJwksServer(AuthUserPrincipalIssuer issuer,Environment environment) {
    this.issuer=Objects.requireNonNull(issuer);this.environment=Objects.requireNonNull(environment);
  }

  @Override public synchronized void start() {
    if(running)return;
    try {
      String host=environment.getProperty("AUTH_PRINCIPAL_JWKS_LISTEN_HOST","0.0.0.0");
      if(host.isBlank())throw new IllegalArgumentException();
      int port=Integer.parseInt(environment.getProperty("AUTH_PRINCIPAL_JWKS_PORT","8443"));
      if(port<0 || port>65535 || port==0 && !AuthPrincipalTransport.local(environment))throw new IllegalArgumentException();
      var context=AuthPrincipalJwksTls.loadFiles(environment.getProperty("AUTH_GRPC_TLS_CERT_FILE"),
          environment.getProperty("AUTH_GRPC_TLS_KEY_FILE"),environment.getProperty("AUTH_PRINCIPAL_JWKS_CLIENT_CA_FILE"));
      // Validate immutable public material before opening any socket.
      byte[] keys=issuer.jwksJson().getBytes(StandardCharsets.UTF_8);
      if(keys.length==0 || keys.length>65536)throw new IllegalArgumentException();
      server=HttpsServer.create(new InetSocketAddress(host,port),16);
      server.setHttpsConfigurator(new HttpsConfigurator(context){
        @Override public void configure(HttpsParameters parameters) {
          var tls=getSSLContext().getDefaultSSLParameters();
          tls.setProtocols(new String[]{"TLSv1.3","TLSv1.2"});tls.setNeedClientAuth(true);
          parameters.setSSLParameters(tls);
        }
      });
      executor=Executors.newFixedThreadPool(2,Thread.ofPlatform().name("auth-private-jwks-",0).daemon().factory());
      server.setExecutor(executor);
      server.createContext(PATH,exchange->{
        try {
          if(!PATH.equals(exchange.getRequestURI().getRawPath()) || exchange.getRequestURI().getRawQuery()!=null){exchange.sendResponseHeaders(404,-1);return;}
          if(!"GET".equals(exchange.getRequestMethod())){exchange.sendResponseHeaders(405,-1);return;}
          try {
            if(!(exchange instanceof HttpsExchange tls) || tls.getSSLSession().getPeerCertificates().length==0)throw new SSLPeerUnverifiedException("workload certificate required");
          }catch(SSLPeerUnverifiedException denied){exchange.sendResponseHeaders(403,-1);return;}
          exchange.getResponseHeaders().set("Content-Type","application/jwk-set+json");
          exchange.getResponseHeaders().set("Cache-Control","no-store");
          exchange.sendResponseHeaders(200,keys.length);
          exchange.getResponseBody().write(keys);
        }finally{exchange.close();}
      });
      server.start();running=true;
    }catch(Exception invalid){stop();throw new IllegalArgumentException("valid private Auth JWKS listener and workload TLS configuration are required",invalid);}
  }

  @Override public synchronized void stop() {
    if(server!=null){server.stop(0);server=null;}
    if(executor!=null){executor.shutdownNow();executor=null;}
    running=false;
  }
  @Override public boolean isRunning(){return running;}
  @Override public int getPhase(){return Integer.MIN_VALUE;}
  int port(){return server==null?-1:server.getAddress().getPort();}
}
