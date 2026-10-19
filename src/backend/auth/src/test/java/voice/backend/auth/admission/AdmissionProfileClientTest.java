package voice.backend.auth.admission;

import static org.junit.jupiter.api.Assertions.*;
import app.voice.user.v1.*;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jwt.SignedJWT;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.*;
import java.nio.file.*;
import java.security.*;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

class AdmissionProfileClientTest {
  @TempDir Path keys;
  final AtomicReference<GetSpaceAdmissionProfileEligibilityResponse> override=new AtomicReference<>();
  @Test void signedExactAdmissionMethodOverMutualTlsAndBoundedNestedDeadline() throws Exception {
    try(var f=new Fixture()) {
      var result=f.client.inspect(f.request);
      assertEquals(4,result.revision());assertEquals(f.request.requestHash(),result.binding());
      assertThrows(IllegalArgumentException.class,()->f.issuer.issue("/voice.user.v1.UserService/GetSpaceAdmissionProfileEligibility",UUID.randomUUID().toString(),f.request.requestHash()));
    }
  }
  @Test void malformedPositiveResponseNeverBecomesAuthority() throws Exception {
    try(var f=new Fixture()) {
      for(var response:new GetSpaceAdmissionProfileEligibilityResponse[]{
          f.response().toBuilder().setAccountId(UUID.randomUUID().toString()).build(),
          f.response().toBuilder().setObservationBindingSha256(com.google.protobuf.ByteString.copyFrom(new byte[31])).build(),
          f.response().toBuilder().setProfileRevision(0).build(),
          f.response().toBuilder().setUnknownFields(com.google.protobuf.UnknownFieldSet.newBuilder().addField(99,com.google.protobuf.UnknownFieldSet.Field.newBuilder().addVarint(1).build()).build()).build()}) {
        override.set(response);
        assertEquals(Status.Code.UNAVAILABLE,assertThrows(StatusRuntimeException.class,()->f.client.inspect(f.request)).getStatus().getCode());
      }
    }
  }
  class Fixture implements AutoCloseable {
    final AuthUserPrincipalIssuer issuer;final AdmissionProfileClient client;final Server server;final ManagedChannel channel;
    final AdmissionObservation.Request request=new AdmissionObservation.Request(AdmissionObservation.Phase.READ,AdmissionObservation.Role.APPLICANT,
        UUID.randomUUID(),UUID.randomUUID(),UUID.randomUUID(),"ab".repeat(32),"sha256:"+"cd".repeat(32),1L);
    final KeyPair current;
    Fixture() throws Exception {
      var generator=KeyPairGenerator.getInstance("RSA");generator.initialize(2048);current=generator.generateKeyPair();
      for(var entry:java.util.Map.of("current",current,"next",generator.generateKeyPair()).entrySet()) {
        Files.writeString(keys.resolve(entry.getKey()+".pem"),"-----BEGIN PRIVATE KEY-----\n"+java.util.Base64.getMimeEncoder(64,new byte[]{'\n'}).encodeToString(entry.getValue().getPrivate().getEncoded())+"\n-----END PRIVATE KEY-----\n");
      }
      issuer=AuthUserPrincipalIssuer.load(keys,"current",Clock.systemUTC());
      ServerInterceptor auth=new ServerInterceptor(){
        @Override public <Q,S> ServerCall.Listener<Q> interceptCall(ServerCall<Q,S> call,Metadata headers,ServerCallHandler<Q,S> next){
          String raw=headers.get(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER));
          try {
            var token=SignedJWT.parse(raw.substring(7));assertTrue(token.verify(new RSASSAVerifier((RSAPublicKey)current.getPublic())));
            var claims=token.getJWTClaimsSet();assertEquals("auth",claims.getIssuer());assertEquals("service:auth",claims.getSubject());
            assertEquals(java.util.List.of("user"),claims.getAudience());assertEquals("/voice.user.v1.UserService/GetSpaceAdmissionProfileEligibility",claims.getStringClaim("rpc"));
            assertTrue(Context.current().getDeadline().timeRemaining(TimeUnit.MILLISECONDS)<=2000);
            return next.startCall(call,headers);
          } catch(Exception error){throw new RuntimeException(error);}
        }
      };
      var service=new UserServiceGrpc.UserServiceImplBase(){
        @Override public void getSpaceAdmissionProfileEligibility(GetSpaceAdmissionProfileEligibilityRequest req,io.grpc.stub.StreamObserver<GetSpaceAdmissionProfileEligibilityResponse> observer){
          assertEquals(request.account().toString(),req.getAccountId());assertEquals(request.profile().toString(),req.getProfileId());
          assertEquals(request.requestHash().substring(7),java.util.HexFormat.of().formatHex(req.getObservationBindingSha256().toByteArray()));
          observer.onNext(override.get()==null?response():override.get());observer.onCompleted();
        }
      };
      var cert=resource("server-cert.pem");var key=resource("server-key.pem");
      server=NettyServerBuilder.forPort(0).sslContext(GrpcSslContexts.forServer(cert,key).trustManager(cert)
          .clientAuth(io.grpc.netty.shaded.io.netty.handler.ssl.ClientAuth.REQUIRE).build()).intercept(auth).addService(service).build().start();
      channel=NettyChannelBuilder.forAddress("localhost",server.getPort()).sslContext(GrpcSslContexts.forClient().trustManager(cert).keyManager(cert,key).build()).build();
      client=new AdmissionProfileClient(UserServiceGrpc.newBlockingStub(channel),issuer);
    }
    GetSpaceAdmissionProfileEligibilityResponse response(){return GetSpaceAdmissionProfileEligibilityResponse.newBuilder()
        .setAccountId(request.account().toString()).setProfileId(request.profile().toString())
        .setObservationBindingSha256(com.google.protobuf.ByteString.copyFrom(java.util.HexFormat.of().parseHex(request.requestHash().substring(7)))).setProfileRevision(4).build();}
    @Override public void close()throws Exception{channel.shutdownNow().awaitTermination(5,TimeUnit.SECONDS);server.shutdownNow().awaitTermination(5,TimeUnit.SECONDS);}
  }
  static java.io.File resource(String name)throws Exception{return new java.io.File(AdmissionProfileClientTest.class.getResource("/principal-tls/"+name).toURI());}
}
