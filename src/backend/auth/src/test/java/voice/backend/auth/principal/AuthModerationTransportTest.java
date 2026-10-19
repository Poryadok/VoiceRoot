package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;
import com.google.protobuf.Struct;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.stub.ClientCalls;
import io.grpc.stub.MetadataUtils;
import io.grpc.stub.ServerCalls;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

class AuthModerationTransportTest {
  @Test void authorityRequiresClientCertificateTrustEvenWithValidServerTlsAndLocalProfile() throws Exception {
    var environment=new MockEnvironment()
        .withProperty("AUTH_GRPC_TLS_CERT_FILE",AuthPrincipalServicesTest.resource("server-cert.pem").getAbsolutePath())
        .withProperty("AUTH_GRPC_TLS_KEY_FILE",AuthPrincipalServicesTest.resource("server-key.pem").getAbsolutePath());
    environment.setActiveProfiles("local");
    assertDoesNotThrow(()->AuthPrincipalTransport.configure(NettyServerBuilder.forPort(0),environment,true));
    assertThrows(IllegalArgumentException.class,()->AuthPrincipalTransport.configureModerationAuthority(NettyServerBuilder.forPort(0),environment),"authority must require a trusted client CA independently of the existing proof listener");
  }
  @Test void completeMutualTlsAndSeparatePortAreRequired() throws Exception {
    var certificate=AuthPrincipalServicesTest.resource("server-cert.pem").getAbsolutePath();
    var environment=new MockEnvironment().withProperty("AUTH_GRPC_TLS_CERT_FILE",certificate)
        .withProperty("AUTH_GRPC_TLS_KEY_FILE",AuthPrincipalServicesTest.resource("server-key.pem").getAbsolutePath())
        .withProperty("AUTH_AUTOMOD_TLS_CLIENT_CA_FILE",certificate);
    environment.setActiveProfiles("production");
    assertDoesNotThrow(()->AuthPrincipalTransport.configureModerationAuthority(NettyServerBuilder.forPort(0),environment));
    assertEquals(9093,AuthPrincipalTransport.moderationAuthorityPort(environment,9090,9091,9092));
    for(String port:java.util.List.of("9090","9091","9092","0","-1","65536","invalid")) {
      environment.withProperty("AUTH_AUTOMOD_GRPC_PORT",port);
      assertThrows(IllegalArgumentException.class,()->AuthPrincipalTransport.moderationAuthorityPort(environment,9090,9091,9092));
    }
    environment.setActiveProfiles("test");environment.withProperty("AUTH_AUTOMOD_GRPC_PORT","0");
    assertEquals(0,AuthPrincipalTransport.moderationAuthorityPort(environment,9090,9091,9092));
    for(String setting:java.util.List.of("AUTH_GRPC_TLS_CERT_FILE","AUTH_GRPC_TLS_KEY_FILE","AUTH_AUTOMOD_TLS_CLIENT_CA_FILE")) {
      var partial=new MockEnvironment().withProperty(setting,"");partial.setActiveProfiles("local");
      assertThrows(IllegalArgumentException.class,()->AuthPrincipalTransport.configureModerationAuthority(NettyServerBuilder.forPort(0),partial));
    }
  }

  @Test void actualAuthorityTlsDeniesAnonymousWrongCaAndWrongPrincipalBeforeDispatch() throws Exception {
    var fixture=new AuthPrincipalVerifierTest();
    var calls=new AtomicInteger();
    var method=AuthPrincipalServicesTest.method(AuthPrincipalServerInterceptor.MODERATION_TERMINAL_RPC);
    var definition=ServerServiceDefinition.builder("voice.automod.v1.AuthModerationAuthorityService")
        .addMethod(method,ServerCalls.asyncUnaryCall((Struct request,io.grpc.stub.StreamObserver<Struct> observer)->{
          calls.incrementAndGet();observer.onNext(request);observer.onCompleted();
        })).build();
    var environment=new MockEnvironment()
        .withProperty("AUTH_GRPC_TLS_CERT_FILE",AuthPrincipalServicesTest.resource("server-cert.pem").getAbsolutePath())
        .withProperty("AUTH_GRPC_TLS_KEY_FILE",AuthPrincipalServicesTest.resource("server-key.pem").getAbsolutePath())
        .withProperty("AUTH_AUTOMOD_TLS_CLIENT_CA_FILE",AuthPrincipalServicesTest.resource("server-cert.pem").getAbsolutePath());
    var builder=NettyServerBuilder.forPort(0);AuthPrincipalTransport.configureModerationAuthority(builder,environment);
    var server=builder.addService(ServerInterceptors.intercept(definition,new AuthPrincipalServerInterceptor(fixture.verifier))).build().start();
    try {
      for(String defect:java.util.List.of("no-client","wrong-ca","wrong-principal","valid")) {
        var channel=NettyChannelBuilder.forAddress("localhost",server.getPort())
            .sslContext(AuthPrincipalServicesTest.tlsClientContext(!defect.equals("no-client"),defect.equals("wrong-ca")?"wrong-ca-cert.pem":"server-cert.pem")).build();
        try {
          var claims=AuthPrincipalVerifierTest.claims();
          String issuer=defect.equals("wrong-principal")?"gateway":"messaging";
          claims.put("iss",issuer);claims.put("sub","service:"+issuer);claims.put("principal_type","service");
          claims.remove("account_id");claims.remove("profile_id");claims.remove("session_epoch");
          claims.put("rpc",AuthPrincipalServerInterceptor.MODERATION_TERMINAL_RPC);
          claims.put("request_hash",AuthPrincipalServerInterceptor.requestHash(Struct.getDefaultInstance()));
          var headers=new Metadata();headers.put(AuthPrincipalServerInterceptorTest.AUTH,"Bearer "+AuthPrincipalVerifierTest.token(claims));
          headers.put(AuthPrincipalServerInterceptorTest.REQUEST_ID,"request-1");
          var signed=ClientInterceptors.intercept(channel,MetadataUtils.newAttachHeadersInterceptor(headers));
          java.util.function.Supplier<Struct> invoke=()->ClientCalls.blockingUnaryCall(signed,method,CallOptions.DEFAULT.withDeadlineAfter(3,TimeUnit.SECONDS),Struct.getDefaultInstance());
          if(defect.equals("valid")){assertEquals(Struct.getDefaultInstance(),invoke.get());assertEquals(1,calls.get());}
          else {
            var error=assertThrows(StatusRuntimeException.class,invoke::get);
            assertEquals(defect.equals("wrong-principal")?Status.Code.PERMISSION_DENIED:Status.Code.UNAVAILABLE,error.getStatus().getCode());
            assertEquals(0,calls.get(),"transport/principal rejection must precede domain dispatch");
          }
        }finally{channel.shutdownNow();assertTrue(channel.awaitTermination(5,TimeUnit.SECONDS));}
      }
    }finally{server.shutdownNow();assertTrue(server.awaitTermination(5,TimeUnit.SECONDS));}
  }
}
