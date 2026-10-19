package voice.backend.auth.admission;

import app.voice.user.v1.GetSpaceAdmissionProfileEligibilityRequest;
import app.voice.user.v1.UserServiceGrpc;
import com.google.protobuf.ByteString;
import io.grpc.Context;
import io.grpc.Metadata;
import io.grpc.Status;
import io.grpc.stub.MetadataUtils;
import java.util.HexFormat;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

/** Nested request shares the outer RPC's deadline; no positive eligibility cache. */
public final class AdmissionProfileClient implements AdmissionObservation.Profiles {
  private final UserServiceGrpc.UserServiceBlockingStub stub;
  private final AuthUserPrincipalIssuer issuer;
  public AdmissionProfileClient(UserServiceGrpc.UserServiceBlockingStub stub, AuthUserPrincipalIssuer issuer) {
    this.stub=java.util.Objects.requireNonNull(stub); this.issuer=java.util.Objects.requireNonNull(issuer);
  }
  @Override public AdmissionObservation.Profile inspect(AdmissionObservation.Request r) {
    if (org.springframework.transaction.support.TransactionSynchronizationManager.isActualTransactionActive()
        || Thread.holdsLock(this)) throw Status.UNAVAILABLE.withDescription("admission owner call under lock").asRuntimeException();
    try {
      long remaining=TimeUnit.SECONDS.toNanos(2);
      if (Context.current().getDeadline()!=null) remaining=Math.min(remaining,Context.current().getDeadline().timeRemaining(TimeUnit.NANOSECONDS));
      if (remaining<=0 || Context.current().isCancelled()) throw Status.DEADLINE_EXCEEDED.asRuntimeException();
      byte[] binding=HexFormat.of().parseHex(r.requestHash().substring(7));
      var request=GetSpaceAdmissionProfileEligibilityRequest.newBuilder().setAccountId(r.account().toString())
          .setProfileId(r.profile().toString()).setObservationBindingSha256(ByteString.copyFrom(binding)).build();
      String id=UUID.randomUUID().toString();
      Metadata headers=new Metadata();
      headers.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),
          "Bearer "+issuer.issueAdmissionProfile(id,AuthPrincipalServerInterceptor.requestHash(request)));
      headers.put(Metadata.Key.of("x-request-id",Metadata.ASCII_STRING_MARSHALLER),id);
      var response=stub.withDeadlineAfter(remaining,TimeUnit.NANOSECONDS)
          .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers)).getSpaceAdmissionProfileEligibility(request);
      if (!response.getUnknownFields().asMap().isEmpty() || response.getProfileRevision()<=0
          || !response.getAccountId().equals(r.account().toString()) || !response.getProfileId().equals(r.profile().toString())
          || !response.getObservationBindingSha256().equals(request.getObservationBindingSha256())) throw new IllegalArgumentException();
      return new AdmissionObservation.Profile(r.account(),r.profile(),r.requestHash(),response.getProfileRevision(),response.getDeleted(),response.getFrozen());
    } catch (io.grpc.StatusRuntimeException error) {
      if (error.getStatus().getCode()==Status.Code.PERMISSION_DENIED || error.getStatus().getCode()==Status.Code.NOT_FOUND)
        throw Status.PERMISSION_DENIED.withDescription("admission profile ineligible").asRuntimeException();
      throw Status.UNAVAILABLE.withDescription("admission profile authority unavailable").asRuntimeException();
    } catch (RuntimeException error) {
      throw Status.UNAVAILABLE.withDescription("admission profile authority unavailable").asRuntimeException();
    }
  }
}
