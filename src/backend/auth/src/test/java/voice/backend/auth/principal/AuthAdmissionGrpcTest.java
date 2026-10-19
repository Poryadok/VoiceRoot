package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import app.voice.auth.v1.*;
import com.google.protobuf.ByteString;
import com.google.protobuf.UnknownFieldSet;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.stub.MetadataUtils;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;
import voice.backend.auth.admission.AdmissionObservation;
import voice.backend.auth.grpc.AuthGrpcService;

class AuthAdmissionGrpcTest {
  final UUID account=UUID.randomUUID(),profile=UUID.randomUUID(),space=UUID.randomUUID();
  final AtomicInteger reads=new AtomicInteger();
  ObserveSpaceAdmissionSubjectRequest request() {
    return ObserveSpaceAdmissionSubjectRequest.newBuilder().setProtocolVersion(1)
        .setPhase(SpaceAdmissionPhase.SPACE_ADMISSION_PHASE_SUBMIT).setSubjectRole(SpaceAdmissionSubjectRole.SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT)
        .setAccountId(account.toString()).setProfileId(profile.toString()).setSpaceId(space.toString())
        .setAdmissionBindingSha256(ByteString.copyFrom(new byte[32])).setInteractiveSessionEpoch(8).build();
  }
  @Test void signedMutualTlsSpaceGetsTypedEligibleAndOtherCallersCannotBorrow() throws Exception {
    try(var fixture=new Fixture()) {
      var req=request();var response=fixture.call(req,req,"space");
      assertEquals(SpaceAdmissionDecision.SPACE_ADMISSION_DECISION_ELIGIBLE,response.getDecision());
      assertEquals(8,response.getSessionEpochFloor());assertEquals("guest",response.getAccountType());
      assertEquals(ByteString.copyFrom(java.util.HexFormat.of().parseHex(AuthPrincipalServerInterceptor.requestHash(req).substring(7))),response.getRequestBindingSha256());
      int count=reads.get();
      for(String issuer:Set.of("role","voice","gateway")) assertEquals(Status.Code.PERMISSION_DENIED,
          assertThrows(StatusRuntimeException.class,()->fixture.call(req,req,issuer)).getStatus().getCode());
      assertEquals(count,reads.get());
      var altered=req.toBuilder().setSpaceId(UUID.randomUUID().toString()).build();
      assertEquals(Status.Code.UNAUTHENTICATED,assertThrows(StatusRuntimeException.class,()->fixture.call(req,altered,"space")).getStatus().getCode());
      assertEquals(count,reads.get());
    }
  }
  @Test void unknownFieldsEnumsAndUnsignedEpochFailBeforeOwnerReads() throws Exception {
    try(var fixture=new Fixture()) {
      var base=request();
      var unknown=UnknownFieldSet.newBuilder().addField(99,UnknownFieldSet.Field.newBuilder().addVarint(1).build()).build();
      for(var req:new ObserveSpaceAdmissionSubjectRequest[]{base.toBuilder().setProtocolVersion(2).build(),base.toBuilder().setPhaseValue(99).build(),
          base.toBuilder().setSubjectRoleValue(99).build(),base.toBuilder().setInteractiveSessionEpoch(-1L).build(),
          base.toBuilder().clearInteractiveSessionEpoch().build(),base.toBuilder().setUnknownFields(unknown).build()}) {
        assertEquals(Status.Code.INVALID_ARGUMENT,assertThrows(StatusRuntimeException.class,()->fixture.call(req,req,"space")).getStatus().getCode());
      }
      assertEquals(0,reads.get());
    }
  }
  class Fixture implements AutoCloseable {
    final AuthPrincipalVerifierTest signed=new AuthPrincipalVerifierTest();
    final Server server;final ManagedChannel channel;
    Fixture() throws Exception {
      var domain=new AdmissionObservation(id->{reads.incrementAndGet();return new AdmissionObservation.Account(account,"guest",true,3,7,8);},
          r->new AdmissionObservation.Profile(account,profile,r.requestHash(),4,false,false));
      var service=new AuthGrpcService(null,null);service.setAdmissionObservation(domain);
      var builder=NettyServerBuilder.forPort(0);
      var env=new org.springframework.mock.env.MockEnvironment()
          .withProperty("AUTH_GRPC_TLS_CERT_FILE",AuthPrincipalServicesTest.resource("server-cert.pem").getAbsolutePath())
          .withProperty("AUTH_GRPC_TLS_KEY_FILE",AuthPrincipalServicesTest.resource("server-key.pem").getAbsolutePath())
          .withProperty("AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE",AuthPrincipalServicesTest.resource("server-cert.pem").getAbsolutePath());
      env.setActiveProfiles("production");AuthPrincipalTransport.configureSessionFloor(builder,env,true);
      server=builder.addService(AuthPrincipalServices.sessionFloorService(service.bindService(),new AuthPrincipalServerInterceptor(signed.verifier))).build().start();
      channel=NettyChannelBuilder.forAddress("localhost",server.getPort()).sslContext(AuthPrincipalServicesTest.tlsClientContext(true,"server-cert.pem")).build();
    }
    ObserveSpaceAdmissionSubjectResponse call(ObserveSpaceAdmissionSubjectRequest signedReq,ObserveSpaceAdmissionSubjectRequest body,String issuer) {
      var claims=AuthPrincipalVerifierTest.claims();claims.put("iss",issuer);claims.put("sub","service:"+issuer);claims.put("principal_type","service");
      claims.remove("account_id");claims.remove("profile_id");claims.remove("session_epoch");
      claims.put("rpc",AuthPrincipalServerInterceptor.SPACE_ADMISSION_RPC);claims.put("request_hash",AuthPrincipalServerInterceptor.requestHash(signedReq));
      var headers=new Metadata();headers.put(AuthPrincipalServerInterceptorTest.AUTH,"Bearer "+AuthPrincipalVerifierTest.token(claims));headers.put(AuthPrincipalServerInterceptorTest.REQUEST_ID,"request-1");
      return AuthServiceGrpc.newBlockingStub(channel).withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers)).withDeadlineAfter(3,TimeUnit.SECONDS).observeSpaceAdmissionSubject(body);
    }
    @Override public void close() throws Exception { channel.shutdownNow().awaitTermination(5,TimeUnit.SECONDS);server.shutdownNow().awaitTermination(5,TimeUnit.SECONDS); }
  }
}
