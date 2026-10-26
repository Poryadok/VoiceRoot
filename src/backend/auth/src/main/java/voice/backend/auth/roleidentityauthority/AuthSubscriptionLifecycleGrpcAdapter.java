package voice.backend.auth.roleidentityauthority;

import app.voice.auth.v1.*;
import com.google.protobuf.Timestamp;
import io.grpc.Context;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.time.Instant;
import java.util.Objects;
import java.util.UUID;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.VerifiedPrincipal;

/** Register exclusively on the closed Subscription mTLS capability. */
public final class AuthSubscriptionLifecycleGrpcAdapter extends AuthSubscriptionLifecycleServiceGrpc.AuthSubscriptionLifecycleServiceImplBase {
  private final AuthSubscriptionLifecycleOwnerStore store;
  public AuthSubscriptionLifecycleGrpcAdapter(AuthSubscriptionLifecycleOwnerStore store) { this.store = Objects.requireNonNull(store); }

  @Override public void observeSubscriptionAccount(ObserveSubscriptionAccountRequest request, StreamObserver<ObserveSubscriptionAccountResponse> response) {
    var principal = VerifiedPrincipal.current();
    if (principal == null || !VerifiedPrincipal.SERVICE.equals(principal.kind()) || !principal.issuer().equals("subscription")
        || AuthPrincipalServerInterceptor.verifiedRequestId() == null) {
      response.onError(Status.PERMISSION_DENIED.withDescription("Subscription owner caller denied").asRuntimeException()); return;
    }
    final UUID account;
    try {
      account = UUID.fromString(request.getAccountId());
      if (!account.toString().equals(request.getAccountId()) || account.equals(new UUID(0,0))
          || request.getMinimumLifecycleRevision() < 0 || !request.getUnknownFields().asMap().isEmpty()) throw new IllegalArgumentException();
    } catch (RuntimeException invalid) {
      response.onError(Status.INVALID_ARGUMENT.withDescription("invalid Subscription owner request").asRuntimeException()); return;
    }
    try {
      if (Context.current().isCancelled()) throw new IllegalStateException();
      var observed = store.observe(account, request.getMinimumLifecycleRevision());
      if (Context.current().isCancelled() || !account.equals(observed.accountId())) throw new IllegalStateException();
      var result = ObserveSubscriptionAccountResponse.newBuilder().setProtocolVersion(1).setAccountId(account.toString())
          .setAccountType(observed.accountType()).setAccountStatus(observed.status())
          .setRegularEmailVerificationPending(observed.emailVerificationPending()).setSessionEpoch(observed.sessionEpoch())
          .setSecurityRevision(observed.securityRevision()).setObservedAt(timestamp(observed.observedAt()))
          .setCaptureCoverage(observed.coverage() == AuthSubscriptionLifecycleOwnerStore.Coverage.CAPTURED
              ? SubscriptionAccountCaptureCoverage.SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_CAPTURED
              : SubscriptionAccountCaptureCoverage.SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNRECORDED);
      if (observed.deletedAt() != null) result.setDeletedAt(timestamp(observed.deletedAt()));
      if (observed.head() != null) result.setHead(fact(observed.head()));
      for (var fact : observed.facts()) result.addFacts(fact(fact));
      response.onNext(result.build()); response.onCompleted();
    } catch (RuntimeException unavailable) {
      response.onError((Context.current().isCancelled() ? Status.DEADLINE_EXCEEDED : Status.UNAVAILABLE)
          .withDescription("Subscription owner source unavailable").asRuntimeException());
    }
  }

  private static SubscriptionAccountLifecycleFact fact(AuthSubscriptionLifecycleOwnerStore.Fact value) {
    return SubscriptionAccountLifecycleFact.newBuilder().setAccountId(value.accountId().toString()).setSourceRevision(value.sourceRevision())
        .setEventId(value.eventId().toString()).setDeletionCycleId(value.deletionCycleId().toString()).setSessionEpoch(value.sessionEpoch())
        .setKind(value.kind().equals("DELETE_SCHEDULED") ? SubscriptionAccountLifecycleKind.SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_DELETE_SCHEDULED
            : SubscriptionAccountLifecycleKind.SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED)
        .setScheduledAt(timestamp(value.scheduledAt())).setPurgeAt(timestamp(value.purgeAt()))
        .setEffectiveAt(timestamp(value.effectiveAt())).setCapturedAt(timestamp(value.capturedAt())).build();
  }
  private static Timestamp timestamp(Instant value) { return Timestamp.newBuilder().setSeconds(value.getEpochSecond()).setNanos(value.getNano()).build(); }
}
