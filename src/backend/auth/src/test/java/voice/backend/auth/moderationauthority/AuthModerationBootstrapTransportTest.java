package voice.backend.auth.moderationauthority;

import static org.junit.jupiter.api.Assertions.*;
import app.voice.automod.v1.*;
import com.nimbusds.jose.*;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jwt.*;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.*;
import io.grpc.stub.MetadataUtils;
import java.nio.file.*;
import java.nio.file.attribute.PosixFilePermissions;
import java.security.*;
import java.security.interfaces.RSAPublicKey;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.mock.env.MockEnvironment;
import voice.backend.auth.principal.*;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

class AuthModerationBootstrapTransportTest {
  @TempDir Path directory;
  @Test void privateAuthorityIsReachableButClosedBeforeVerifiedP1() throws Exception {
    try(var f=AuthLifecycleDeliveryIntegrationTest.fixture()) {
      UUID account=f.account();var boot=new AuthModerationBootAuthority();var manager=new DataSourceTransactionManager(f.source);
      var named=new NamedParameterJdbcTemplate(f.source);var store=new AuthModerationConfiguration().authModerationStore(named,manager,boot);
      var pair=key();var seen=ConcurrentHashMap.<String>newKeySet();
      var verifier=new AuthPrincipalVerifier((issuer,kid)->{
        if(!issuer.equals("messaging")||!kid.equals("current"))throw new IllegalArgumentException();return (RSAPublicKey)pair.getPublic();
      },ignored->{throw new AssertionError("service bootstrap cannot depend on delegated-user floor");},
          (issuer,jti,expires)->{if(!seen.add(issuer+":"+jti))throw new IllegalArgumentException();},Clock.systemUTC());
      var server=NettyServerBuilder.forPort(0).sslContext(GrpcSslContexts.forServer(resource("server-cert.pem"),resource("server-key.pem"))
          .trustManager(resource("server-cert.pem")).clientAuth(io.grpc.netty.shaded.io.netty.handler.ssl.ClientAuth.REQUIRE).build())
          .addService(ServerInterceptors.intercept(new AuthModerationService(store),new AuthPrincipalServerInterceptor(verifier))).build().start();
      var channel=NettyChannelBuilder.forAddress("localhost",server.getPort()).sslContext(GrpcSslContexts.forClient().trustManager(resource("server-cert.pem"))
          .keyManager(resource("server-cert.pem"),resource("server-key.pem")).build()).build();
      try(var runtime=new AuthModerationStartupRuntime(named,manager,boot,settings(f),null)) {
        var request=ResolveModerationSubjectRequest.newBuilder().setAccountId(account.toString()).build();
        assertEquals(Status.Code.UNAVAILABLE,assertThrows(StatusRuntimeException.class,()->resolve(channel,pair,request)).getStatus().getCode());
        runtime.localListenersStarted();assertEquals("AUTHORITY_P1",runtime.state());
        assertEquals(account.toString(),resolve(channel,pair,request).getSubject().getAccountId());
        boot.close();assertEquals(Status.Code.UNAVAILABLE,assertThrows(StatusRuntimeException.class,()->resolve(channel,pair,request)).getStatus().getCode());
      }finally{channel.shutdownNow().awaitTermination(5,TimeUnit.SECONDS);server.shutdownNow().awaitTermination(5,TimeUnit.SECONDS);}
    }
  }
  @Test void peerRpcWaitDoesNotOccupyTerminalDeadlineWorker() throws Exception {
    try(var f=AuthLifecycleDeliveryIntegrationTest.fixture()) {
      f.account();var boot=new AuthModerationBootAuthority();var manager=new DataSourceTransactionManager(f.source);var named=new NamedParameterJdbcTemplate(f.source);
      var entered=new CountDownLatch(1);var active=new AtomicReference<Context>();
      var receiver=new AccountLifecycleReceiverServiceGrpc.AccountLifecycleReceiverServiceImplBase(){
        @Override public void applyLifecycleFact(ApplyLifecycleFactRequest request,io.grpc.stub.StreamObserver<ApplyLifecycleFactResponse> observer){active.set(Context.current());entered.countDown();}
      };
      var server=NettyServerBuilder.forPort(0).sslContext(GrpcSslContexts.forServer(resource("server-cert.pem"),resource("server-key.pem"))
          .trustManager(resource("server-cert.pem")).clientAuth(io.grpc.netty.shaded.io.netty.handler.ssl.ClientAuth.REQUIRE).build())
          .addService(receiver).build().start();
      var settings=settings(f).withProperty("auth.automod.lifecycle.enabled","true").withProperty("AUTH_AUTOMOD_LIFECYCLE_BATCH_SIZE","1");
      for(String peer:List.of("MESSAGING","MODERATION")) {
        String prefix="AUTH_AUTOMOD_LIFECYCLE_"+peer+"_";
        settings.withProperty(prefix+"GRPC_ADDR","localhost:"+server.getPort()).withProperty(prefix+"TLS_SERVER_NAME","localhost")
            .withProperty(prefix+"TLS_CA_FILE",resource("server-cert.pem").getPath()).withProperty(prefix+"TLS_CERT_FILE",resource("server-cert.pem").getPath())
            .withProperty(prefix+"TLS_KEY_FILE",resource("server-key.pem").getPath());
      }
      Path keyDirectory=Files.createDirectory(directory.resolve("keys"));
      for(String kid:List.of("current","next"))Files.writeString(keyDirectory.resolve(kid+".pem"),"-----BEGIN PRIVATE KEY-----\n"+
          Base64.getMimeEncoder(64,new byte[]{'\n'}).encodeToString(key().getPrivate().getEncoded())+"\n-----END PRIVATE KEY-----\n");
      try(var runtime=new AuthModerationStartupRuntime(named,manager,boot,settings,AuthUserPrincipalIssuer.load(keyDirectory,"current",Clock.systemUTC()))) {
        runtime.localListenersStarted();runtime.publish();assertTrue(entered.await(3,TimeUnit.SECONDS));
        UUID due=UUID.randomUUID();f.jdbc.update("INSERT INTO accounts(id,password_hash,type,status,deleted_at) VALUES (?,'fixture','regular','deleted',clock_timestamp()-interval '31 days')",due);
        var store=new AuthModerationConfiguration().authModerationStore(named,manager,boot);
        new AuthModerationConfiguration.TerminalDeadlineWorker(store,boot).purge();
        assertEquals(1,f.jdbc.queryForObject("SELECT count(*) FROM moderation_terminal_subjects",Integer.class));
        assertFalse(active.get().isCancelled(),"terminal cleanup finishes while the peer RPC still waits");
      }finally{server.shutdownNow().awaitTermination(5,TimeUnit.SECONDS);}
    }
  }
  ResolveModerationSubjectResponse resolve(ManagedChannel channel,KeyPair pair,ResolveModerationSubjectRequest request)throws Exception {
    String id=UUID.randomUUID().toString();Instant now=Instant.now().truncatedTo(java.time.temporal.ChronoUnit.SECONDS);
    var claims=new JWTClaimsSet.Builder().issuer("messaging").subject("service:messaging").audience("auth")
        .claim("principal_type","service").claim("rpc",AuthPrincipalServerInterceptor.MODERATION_SUBJECT_RPC)
        .claim("request_id",id).claim("request_hash",AuthPrincipalServerInterceptor.requestHash(request))
        .jwtID(UUID.randomUUID().toString()).issueTime(Date.from(now)).notBeforeTime(Date.from(now)).expirationTime(Date.from(now.plusSeconds(30))).build();
    var token=new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).keyID("current").build(),claims);token.sign(new RSASSASigner(pair.getPrivate()));
    var headers=new Metadata();headers.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),"Bearer "+token.serialize());
    headers.put(Metadata.Key.of("x-request-id",Metadata.ASCII_STRING_MARSHALLER),id);
    return AuthModerationAuthorityServiceGrpc.newBlockingStub(channel).withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
        .withDeadlineAfter(3,TimeUnit.SECONDS).resolveModerationSubject(request);
  }
  MockEnvironment settings(AuthLifecycleDeliveryIntegrationTest.Fixture fixture)throws Exception {
    UUID deployment=UUID.randomUUID();Path file=directory.resolve(UUID.randomUUID()+".json");
    Files.writeString(file,new com.fasterxml.jackson.databind.ObjectMapper().writeValueAsString(Map.of("protocol_version",1,"mode","LIVE_CURRENT","environment","test",
        "deployment_generation",deployment.toString(),"store_incarnation",fixture.jdbc.queryForObject("SELECT incarnation FROM moderation_startup_identity",UUID.class).toString(),
        "authority_generation",fixture.jdbc.queryForObject("SELECT generation FROM moderation_authority_generation",UUID.class).toString())));
    Files.setPosixFilePermissions(file,PosixFilePermissions.fromString("r--------"));
    return new MockEnvironment().withProperty("AUTH_AUTOMOD_START_CONTEXT_FILE",file.toString()).withProperty("AUTH_AUTOMOD_ENVIRONMENT","test")
        .withProperty("AUTH_AUTOMOD_DEPLOYMENT_GENERATION",deployment.toString());
  }
  static KeyPair key()throws Exception{var generator=KeyPairGenerator.getInstance("RSA");generator.initialize(2048);return generator.generateKeyPair();}
  static java.io.File resource(String name)throws Exception{return new java.io.File(AuthModerationBootstrapTransportTest.class.getResource("/principal-tls/"+name).toURI());}
}
