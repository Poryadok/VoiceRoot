package voice.backend.auth.authoritysource;

import io.grpc.Server;
import io.grpc.netty.shaded.io.grpc.netty.*;
import io.grpc.netty.shaded.io.netty.handler.ssl.*;
import io.lettuce.core.ClientOptions;
import io.lettuce.core.SocketOptions;
import java.net.http.*;
import java.nio.file.Files;
import java.security.KeyStore;
import java.security.cert.CertificateFactory;
import java.time.*;
import java.util.concurrent.TimeUnit;
import javax.net.ssl.*;
import javax.sql.DataSource;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.SmartLifecycle;
import org.springframework.core.env.Environment;
import org.springframework.data.redis.connection.*;
import org.springframework.data.redis.connection.lettuce.*;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;
import voice.backend.auth.principal.*;

/** Independent serving set, validated before either legacy Auth surface starts. */
@Component
public final class AuthAuthorityGrpcServer implements SmartLifecycle {
  private final AuthSourceConfig config;
  private final ObjectProvider<DataSource> dataSources;
  private Server server;
  private HttpClient http;
  private LettuceConnectionFactory redis;
  private RedisPrincipalReplayGuard replay;
  private AuthAuthorityReader reader;
  private boolean running;

  @Autowired
  public AuthAuthorityGrpcServer(Environment environment,ObjectProvider<DataSource> dataSources) {
    this.config=AuthSourceConfig.load(environment);this.dataSources=dataSources;
  }
  @Override public int getPhase(){return -100;}
  @Override public boolean isRunning(){return running;}
  public int port(){return server==null?-1:server.getPort();}

  @Override public synchronized void start() {
    if(config==null || running)return;
    try {
      reader=new AuthAuthorityReader(dataSources.getIfAvailable());reader.checkSchema();
      var tls=GrpcSslContexts.configure(SslContextBuilder.forServer(config.certificate().toFile(),config.key().toFile()))
          .trustManager(config.clientCA().toFile()).clientAuth(ClientAuth.REQUIRE).protocols("TLSv1.3","TLSv1.2").build();
      var httpBuilder=HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).followRedirects(HttpClient.Redirect.NEVER);
      if(config.jwksCA()!=null)httpBuilder.sslContext(jwksTrust(config));
      http=httpBuilder.build();
      var standalone=new RedisStandaloneConfiguration(config.redisHost(),config.redisPort());
      if(!config.redisPassword().isEmpty())standalone.setPassword(RedisPassword.of(config.redisPassword()));
      var client=LettuceClientConfiguration.builder().commandTimeout(Duration.ofSeconds(2)).shutdownTimeout(Duration.ofSeconds(1))
          .clientOptions(ClientOptions.builder().socketOptions(SocketOptions.builder().connectTimeout(Duration.ofSeconds(2)).build()).build()).build();
      redis=new LettuceConnectionFactory(standalone,client);redis.afterPropertiesSet();redis.start();
      try(var connection=redis.getConnection()){if(!"PONG".equals(connection.ping()))throw new IllegalStateException();}
      var template=new StringRedisTemplate(redis);template.afterPropertiesSet();
      Clock clock=Clock.systemUTC();replay=new RedisPrincipalReplayGuard(template,clock);
      var resolver=new AuthPrincipalJwksResolver(issuer->{if(!issuer.equals("federation"))throw new IllegalArgumentException();return fetch();},clock,config.refresh(),config.hardExpiry(),config.cooldown());
      var verifier=new AuthSourceVerifier(resolver::resolve,replay,clock);
      // Lazy HTTPS JWKS avoids a Federation/Auth boot cycle; no request is
      // accepted until its fixed trusted endpoint supplies a complete key set.
      server=NettyServerBuilder.forAddress(config.listen()).sslContext(tls).maxInboundMessageSize(1<<20)
          .addService(io.grpc.ServerInterceptors.intercept(new AuthAuthorityService(reader),new AuthSourceInterceptor(verifier))).build();
      server.start();running=true;
    } catch(Exception ex){stop();throw new IllegalStateException("start Auth authority source");}
  }
  private byte[] fetch() {
    var request=HttpRequest.newBuilder(config.jwks()).timeout(Duration.ofSeconds(2)).GET().build();
    var pending=http.sendAsync(request,HttpResponse.BodyHandlers.limiting(HttpResponse.BodyHandlers.ofByteArray(),65536));
    try {var response=pending.get(2,TimeUnit.SECONDS);if(response.statusCode()!=200)throw new IllegalStateException();return response.body();}
    catch(Exception ex){pending.cancel(true);if(ex instanceof InterruptedException)Thread.currentThread().interrupt();throw new IllegalStateException("Auth source JWKS unavailable");}
  }
  private static SSLContext jwksTrust(AuthSourceConfig config)throws Exception {
    var trust=KeyStore.getInstance("PKCS12");trust.load(null,null);
    try(var input=Files.newInputStream(config.jwksCA())) {
      int index=0;for(var certificate:CertificateFactory.getInstance("X.509").generateCertificates(input))trust.setCertificateEntry("source-ca-"+(index++),certificate);
      if(index==0)throw new IllegalArgumentException();
    }
    var managers=TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());managers.init(trust);
    var context=SSLContext.getInstance("TLS");context.init(null,managers.getTrustManagers(),null);return context;
  }
  @Override public synchronized void stop() {
    if(server!=null){server.shutdownNow();server=null;}
    if(reader!=null){reader.close();reader=null;}
    if(replay!=null){replay.close();replay=null;}
    if(redis!=null){redis.destroy();redis=null;}
    if(http!=null){http.shutdownNow();http=null;}
    running=false;
  }
}
