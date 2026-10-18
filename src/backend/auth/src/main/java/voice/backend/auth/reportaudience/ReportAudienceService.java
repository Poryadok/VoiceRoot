package voice.backend.auth.reportaudience;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.ClassifyReportAudienceAccountsRequest;
import app.voice.auth.v1.ClassifyReportAudienceAccountsResponse;
import com.google.protobuf.Timestamp;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.time.Clock;
import java.time.Instant;
import java.util.UUID;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;

final class ReportAudienceService extends AuthServiceGrpc.AuthServiceImplBase {
  private final ReportAudienceReader reader;
  private final Clock clock;
  ReportAudienceService(ReportAudienceReader reader,Clock clock){this.reader=reader;this.clock=clock;}
  @Override public void classifyReportAudienceAccounts(ClassifyReportAudienceAccountsRequest request,StreamObserver<ClassifyReportAudienceAccountsResponse> observer) {
    try {
      var authority=ReportAudienceVerifier.Verified.CONTEXT.get();if(authority==null)throw ReportAudienceVerifier.invalid();
      String hash=AuthPrincipalServerInterceptor.requestHash(request);authority.require(ReportAudienceVerifier.CLASSIFY,hash);
      try {
        ReportAudienceReader.id(request.getEvaluationId());ReportAudienceReader.validate(request.getAccountIdsList());
        if(!request.getUnknownFields().asMap().isEmpty())throw new IllegalArgumentException();
      } catch(IllegalArgumentException invalid){throw Status.INVALID_ARGUMENT.withDescription("invalid report audience batch").asRuntimeException();}
      Instant started=clock.instant();var accounts=reader.classify(request.getAccountIdsList());Instant completed=clock.instant();
      authority.require(ReportAudienceVerifier.CLASSIFY,hash);
      if(completed.isBefore(started) || !completed.isBefore(started.plusSeconds(30)))throw new IllegalStateException();
      observer.onNext(ClassifyReportAudienceAccountsResponse.newBuilder().setEvaluationId(request.getEvaluationId()).addAllAccounts(accounts)
          .setObservationStartedAt(timestamp(started)).setObservationCompletedAt(timestamp(completed)).setExpiresAt(timestamp(started.plusSeconds(30)))
          .setSnapshotId(UUID.randomUUID().toString()).build());observer.onCompleted();
    } catch(RuntimeException error) {
      var status=Status.fromThrowable(error);if(status.getCode()!=Status.Code.UNAUTHENTICATED && status.getCode()!=Status.Code.INVALID_ARGUMENT)status=Status.UNAVAILABLE;
      observer.onError(status.withDescription("Auth report audience rejected").asRuntimeException());
    }
  }
  private static Timestamp timestamp(Instant value){return Timestamp.newBuilder().setSeconds(value.getEpochSecond()).setNanos(value.getNano()).build();}
}
