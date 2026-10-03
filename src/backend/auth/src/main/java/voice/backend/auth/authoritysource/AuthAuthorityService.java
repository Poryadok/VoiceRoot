package voice.backend.auth.authoritysource;

import com.google.protobuf.ByteString;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import voice.authority.v1.Authority;
import voice.authority.v1.AuthoritySourceServiceGrpc;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;

final class AuthAuthorityService extends AuthoritySourceServiceGrpc.AuthoritySourceServiceImplBase {
  private final AuthAuthorityReader reader;
  AuthAuthorityService(AuthAuthorityReader reader){this.reader=reader;}
  private static void scope(Authority.SourceScope scope) {
    if(!scope.getUnknownFields().asMap().isEmpty() || scope.getSchemaVersion()!=1 || !AuthAuthorityReader.id(scope.getSpaceId()) || !scope.getEnvironmentId().isEmpty() || scope.getProfileIdsCount()!=0 || scope.getVoiceRoomIdsCount()!=0) throw Status.INVALID_ARGUMENT.withDescription("invalid Auth source scope").asRuntimeException();
    try {AuthAuthorityReader.validateAccounts(scope.getAccountIdsList());}catch(IllegalArgumentException ex){throw Status.INVALID_ARGUMENT.withDescription("invalid Auth source scope").asRuntimeException();}
  }
  private static AuthSourceVerifier.Verified principal(String method,com.google.protobuf.MessageLite request) {
    var principal=AuthSourceVerifier.Verified.CONTEXT.get();if(principal==null)throw AuthSourceVerifier.invalid();principal.require(method,AuthPrincipalServerInterceptor.requestHash(request));return principal;
  }
  @Override public void readSnapshot(Authority.ReadSnapshotRequest request,StreamObserver<Authority.ReadSnapshotResponse> observer) {
    try {
      var principal=principal(AuthSourceVerifier.SNAPSHOT,request);
      if(!request.hasScope() || !request.getUnknownFields().asMap().isEmpty())throw Status.INVALID_ARGUMENT.withDescription("invalid Auth source request").asRuntimeException();
      scope(request.getScope());var snapshot=reader.snapshot(request.getScope().getAccountIdsList());
      principal.require(AuthSourceVerifier.SNAPSHOT,AuthPrincipalServerInterceptor.requestHash(request));
      if(snapshot.revision()<=0 || snapshot.canonicalState().length==0 || snapshot.canonicalState().length>AuthAuthorityReader.MAX_STATE_BYTES || (snapshot.validUntilUnixMillis()!=0 && snapshot.validUntilUnixMillis()<=System.currentTimeMillis()))throw new IllegalStateException();
      observer.onNext(Authority.ReadSnapshotResponse.newBuilder().setScope(request.getScope()).setOwner(Authority.AuthorityOwner.AUTHORITY_OWNER_AUTH).setRevision(snapshot.revision()).setComplete(true).setCanonicalState(ByteString.copyFrom(snapshot.canonicalState())).setValidUntilUnixMillis(snapshot.validUntilUnixMillis()).build());observer.onCompleted();
    } catch(RuntimeException ex){fail(observer,ex);}
  }
  @Override public void readRevision(Authority.ReadRevisionRequest request,StreamObserver<Authority.ReadRevisionResponse> observer) {
    try {
      var principal=principal(AuthSourceVerifier.REVISION,request);
      if(!request.hasScope() || !request.getUnknownFields().asMap().isEmpty())throw Status.INVALID_ARGUMENT.withDescription("invalid Auth source request").asRuntimeException();
      scope(request.getScope());long revision=reader.revision(request.getScope().getAccountIdsList());principal.require(AuthSourceVerifier.REVISION,AuthPrincipalServerInterceptor.requestHash(request));
      if(revision<=0)throw new IllegalStateException();
      observer.onNext(Authority.ReadRevisionResponse.newBuilder().setScope(request.getScope()).setOwner(Authority.AuthorityOwner.AUTHORITY_OWNER_AUTH).setRevision(revision).build());observer.onCompleted();
    } catch(RuntimeException ex){fail(observer,ex);}
  }
  private static void fail(StreamObserver<?> observer,RuntimeException ex){var status=Status.fromThrowable(ex);if(status.getCode()!=Status.Code.INVALID_ARGUMENT && status.getCode()!=Status.Code.UNAUTHENTICATED)status=Status.UNAVAILABLE;observer.onError(status.withDescription("Auth owning source rejected").asRuntimeException());}
}
