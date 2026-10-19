package voice.backend.auth.moderationauthority;

import static org.junit.jupiter.api.Assertions.*;
import app.voice.automod.v1.*;
import com.google.protobuf.ByteString;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jwt.SignedJWT;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.*;
import java.nio.file.*;
import java.security.*;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.util.*;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.env.MockEnvironment;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

class AuthLifecycleGrpcClientTest {
  @TempDir Path keys;
  @Test void exactBothRecipientsShareIssuerAndUseMutualTls() throws Exception {
    try(var f=new Fixture();var runtime=f.settings().connect(f.issuer)) {
      for(String recipient:List.of("messaging","moderation")) {
        f.recipient.set(recipient);
        assertEquals(7,runtime.client().send(new AuthLifecycleDeliveryStore.Delivery(recipient,f.fact)).getLifecycleRevision());
      }
      assertEquals(2,f.calls.get());
      TransactionSynchronizationManager.setActualTransactionActive(true);
      try {assertThrows(IllegalStateException.class,()->runtime.client().send(f.delivery()));}
      finally {TransactionSynchronizationManager.setActualTransactionActive(false);}
      assertEquals(2,f.calls.get());
    }
  }
  @Test void malformedReceiptAndExpiredDeadlineCannotAcknowledge() throws Exception {
    try(var f=new Fixture();var runtime=f.settings().connect(f.issuer)) {
      f.response.set(f.response().toBuilder().setRequestedEventId(UUID.randomUUID().toString()).build());
      assertThrows(AuthLifecycleDeliveryStore.RestoreDivergence.class,()->runtime.client().send(f.delivery()));
      var scheduler=java.util.concurrent.Executors.newSingleThreadScheduledExecutor();
      var expired=Context.current().withDeadlineAfter(-1,TimeUnit.MILLISECONDS,scheduler);
      try {assertEquals(Status.Code.DEADLINE_EXCEEDED,assertThrows(StatusRuntimeException.class,
          ()->expired.call(()->runtime.client().send(f.delivery()))).getStatus().getCode());}
      finally {expired.cancel(null);scheduler.shutdownNow();}
      assertEquals(1,f.calls.get());
    }
  }
  @Test void anonymousAndWrongTrustFailBeforeHandler() throws Exception {
    try(var f=new Fixture()) {
      for(boolean anonymous:List.of(true,false)) {
        var tls=GrpcSslContexts.forClient().trustManager(resource(anonymous?"server-cert.pem":"wrong-ca-cert.pem"));
        if(!anonymous)tls.keyManager(resource("server-cert.pem"),resource("server-key.pem"));
        var channel=NettyChannelBuilder.forAddress("localhost",f.server.getPort()).sslContext(tls.build()).build();
        try {assertThrows(StatusRuntimeException.class,()->AccountLifecycleReceiverServiceGrpc.newBlockingStub(channel)
            .withDeadlineAfter(2,TimeUnit.SECONDS).applyLifecycleFact(ApplyLifecycleFactRequest.newBuilder().setFact(f.fact).build()));}
        finally {channel.shutdownNow().awaitTermination(5,TimeUnit.SECONDS);}
      }
      assertEquals(0,f.calls.get());
    }
  }
  @Test void partialOrOrphanConfigurationNeverFallsBackToPlaintext() {
    assertTrue(AuthLifecycleClientSettings.load(new MockEnvironment()).isEmpty());
    assertThrows(IllegalArgumentException.class,()->AuthLifecycleClientSettings.load(new MockEnvironment()
        .withProperty("AUTH_AUTOMOD_LIFECYCLE_MODERATION_GRPC_ADDR","localhost:9090")));
    assertThrows(IllegalArgumentException.class,()->AuthLifecycleClientSettings.load(new MockEnvironment()
        .withProperty("auth.automod.lifecycle.enabled","true")));
    assertThrows(IllegalArgumentException.class,()->new AuthLifecycleClientSettings.Endpoint("http://localhost:80","localhost","ca","cert","key"));
  }
  class Fixture implements AutoCloseable {
    final AccountLifecycleFact fact=AuthLifecycleCodecTest.live();
    final AtomicReference<String> recipient=new AtomicReference<>("moderation");
    final AtomicReference<ApplyLifecycleFactResponse> response=new AtomicReference<>();
    final AtomicInteger calls=new AtomicInteger();
    final AuthUserPrincipalIssuer issuer;final Server server;final KeyPair current;
    Fixture() throws Exception {
      var generator=KeyPairGenerator.getInstance("RSA");generator.initialize(2048);current=generator.generateKeyPair();
      for(var entry:Map.of("current",current,"next",generator.generateKeyPair()).entrySet())
        Files.writeString(keys.resolve(entry.getKey()+".pem"),"-----BEGIN PRIVATE KEY-----\n"+
            Base64.getMimeEncoder(64,new byte[]{'\n'}).encodeToString(entry.getValue().getPrivate().getEncoded())+"\n-----END PRIVATE KEY-----\n");
      issuer=AuthUserPrincipalIssuer.load(keys,"current",Clock.systemUTC());
      var requestHash=Context.key("lifecycle-request-hash");
      ServerInterceptor principal=new ServerInterceptor(){
        @Override public <Q,S> ServerCall.Listener<Q> interceptCall(ServerCall<Q,S> call,Metadata headers,ServerCallHandler<Q,S> next) {
          try {
            var token=SignedJWT.parse(headers.get(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER)).substring(7));
            assertTrue(token.verify(new RSASSAVerifier((RSAPublicKey)current.getPublic())));
            var claims=token.getJWTClaimsSet();assertEquals("auth",claims.getIssuer());assertEquals("service:auth",claims.getSubject());
            assertEquals(List.of(recipient.get()),claims.getAudience());
            assertEquals("/voice.automod.v1.AccountLifecycleReceiverService/ApplyLifecycleFact",claims.getStringClaim("rpc"));
            assertEquals(headers.get(Metadata.Key.of("x-request-id",Metadata.ASCII_STRING_MARSHALLER)),claims.getStringClaim("request_id"));
            assertTrue(claims.getExpirationTime().getTime()-claims.getIssueTime().getTime()<=30000);
            assertTrue(Context.current().getDeadline().timeRemaining(TimeUnit.MILLISECONDS)<=2000);
            return Contexts.interceptCall(Context.current().withValue(requestHash,claims.getStringClaim("request_hash")),call,headers,next);
          }catch(Exception failure){throw new RuntimeException(failure);}
        }
      };
      var service=new AccountLifecycleReceiverServiceGrpc.AccountLifecycleReceiverServiceImplBase(){
        @Override public void applyLifecycleFact(ApplyLifecycleFactRequest request,io.grpc.stub.StreamObserver<ApplyLifecycleFactResponse> observer) {
          assertEquals(AuthPrincipalServerInterceptor.requestHash(request),requestHash.get());calls.incrementAndGet();
          observer.onNext(response.get()==null?response():response.get());observer.onCompleted();
        }
      };
      server=NettyServerBuilder.forPort(0).sslContext(GrpcSslContexts.forServer(resource("server-cert.pem"),resource("server-key.pem"))
          .trustManager(resource("server-cert.pem")).clientAuth(io.grpc.netty.shaded.io.netty.handler.ssl.ClientAuth.REQUIRE).build())
          .intercept(principal).addService(service).build().start();
    }
    AuthLifecycleDeliveryStore.Delivery delivery(){return new AuthLifecycleDeliveryStore.Delivery("moderation",fact);}
    ApplyLifecycleFactResponse response(){return ApplyLifecycleFactResponse.newBuilder().setProtocolVersion(1)
        .setAuthorityGeneration(fact.getAuthorityGeneration()).setSubjectId(fact.getSubjectId()).setRequestedEventId(fact.getEventId())
        .setLifecycleRevision(7).setCurrentFactSha256(ByteString.copyFrom(AuthLifecycleCodec.hash(fact))).build();}
    AuthLifecycleClientSettings settings()throws Exception {
      var endpoint=new AuthLifecycleClientSettings.Endpoint("localhost:"+server.getPort(),"localhost",resource("server-cert.pem").getPath(),
          resource("server-cert.pem").getPath(),resource("server-key.pem").getPath());
      return new AuthLifecycleClientSettings(Map.of("messaging",endpoint,"moderation",endpoint));
    }
    @Override public void close()throws Exception {server.shutdownNow().awaitTermination(5,TimeUnit.SECONDS);}
  }
  static java.io.File resource(String name)throws Exception{return new java.io.File(AuthLifecycleGrpcClientTest.class.getResource("/principal-tls/"+name).toURI());}
}
