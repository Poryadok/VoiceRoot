package voice.backend.auth.support;

import app.voice.user.v1.ClearVerificationRequest;
import app.voice.user.v1.ClearVerificationResponse;
import app.voice.user.v1.EnsurePrimaryProfileRequest;
import app.voice.user.v1.EnsurePrimaryProfileResponse;
import app.voice.user.v1.MarkAccountRegularRequest;
import app.voice.user.v1.MarkAccountRegularResponse;
import app.voice.user.v1.Profile;
import app.voice.user.v1.ResolvePrimaryProfileIDsRequest;
import app.voice.user.v1.ResolvePrimaryProfileIDsResponse;
import app.voice.user.v1.SetVerificationRequest;
import app.voice.user.v1.SetVerificationResponse;
import app.voice.user.v1.SwitchProfileRequest;
import app.voice.user.v1.SwitchProfileResponse;
import app.voice.user.v1.UserServiceGrpc;
import app.voice.user.v1.VerificationStatus;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/** Test-only in-process User RPC fake; no User database is involved. */
public final class RecordingUserGrpcService extends UserServiceGrpc.UserServiceImplBase {
  public enum Outcome {
    SUCCESS,
    INVALID_ARGUMENT,
    UNAVAILABLE,
    NOT_FOUND,
    PERMISSION_DENIED,
    UNAUTHENTICATED,
    DEADLINE_EXCEEDED,
    FAILED_PRECONDITION,
    MALFORMED
  }

  private Outcome ensureOutcome = Outcome.SUCCESS;
  private Outcome resolveOutcome = Outcome.SUCCESS;
  private Outcome markRegularOutcome = Outcome.SUCCESS;
  private Outcome setVerificationOutcome = Outcome.SUCCESS;
  private Outcome clearVerificationOutcome = Outcome.SUCCESS;
  private Outcome switchOutcome = Outcome.SUCCESS;
  private Profile ensuredProfile;
  private Profile switchedProfile;
  private final Map<String, String> resolvedPrimaryProfileIds = new LinkedHashMap<>();
  private final List<EnsurePrimaryProfileRequest> ensureRequests = new ArrayList<>();
  private final List<ResolvePrimaryProfileIDsRequest> resolveRequests = new ArrayList<>();
  private final List<MarkAccountRegularRequest> markRegularRequests = new ArrayList<>();
  private final List<SetVerificationRequest> setVerificationRequests = new ArrayList<>();
  private final List<ClearVerificationRequest> clearVerificationRequests = new ArrayList<>();
  private final List<SwitchProfileRequest> switchRequests = new ArrayList<>();
  private final List<String> rpcCalls = new ArrayList<>();

  public void setEnsuredProfile(Profile profile) { this.ensuredProfile = profile; }
  public void setSwitchedProfile(Profile profile) { this.switchedProfile = profile; }
  public void setEnsureOutcome(Outcome outcome) { this.ensureOutcome = outcome; }
  public void setResolveOutcome(Outcome outcome) { this.resolveOutcome = outcome; }
  public void setMarkRegularOutcome(Outcome outcome) { this.markRegularOutcome = outcome; }
  public void setSetVerificationOutcome(Outcome outcome) { this.setVerificationOutcome = outcome; }
  public void setClearVerificationOutcome(Outcome outcome) { this.clearVerificationOutcome = outcome; }
  public void setSwitchOutcome(Outcome outcome) { this.switchOutcome = outcome; }
  public Map<String, String> resolvedPrimaryProfileIds() { return resolvedPrimaryProfileIds; }
  public List<EnsurePrimaryProfileRequest> ensureRequests() { return ensureRequests; }
  public List<ResolvePrimaryProfileIDsRequest> resolveRequests() { return resolveRequests; }
  public List<MarkAccountRegularRequest> markRegularRequests() { return markRegularRequests; }
  public List<SetVerificationRequest> setVerificationRequests() { return setVerificationRequests; }
  public List<ClearVerificationRequest> clearVerificationRequests() { return clearVerificationRequests; }
  public List<SwitchProfileRequest> switchRequests() { return switchRequests; }
  public List<String> rpcCalls() { return rpcCalls; }

  @Override
  public void ensurePrimaryProfile(
      EnsurePrimaryProfileRequest request,
      StreamObserver<EnsurePrimaryProfileResponse> observer) {
    ensureRequests.add(request);
    rpcCalls.add("EnsurePrimaryProfile");
    if (fail(ensureOutcome, observer)) return;
    EnsurePrimaryProfileResponse.Builder response = EnsurePrimaryProfileResponse.newBuilder();
    if (ensureOutcome != Outcome.MALFORMED && ensuredProfile != null) response.setProfile(ensuredProfile);
    observer.onNext(response.build());
    observer.onCompleted();
  }

  @Override
  public void resolvePrimaryProfileIDs(
      ResolvePrimaryProfileIDsRequest request,
      StreamObserver<ResolvePrimaryProfileIDsResponse> observer) {
    resolveRequests.add(request);
    rpcCalls.add("ResolvePrimaryProfileIDs");
    if (fail(resolveOutcome, observer)) return;
    ResolvePrimaryProfileIDsResponse.Builder response = ResolvePrimaryProfileIDsResponse.newBuilder();
    if (resolveOutcome != Outcome.MALFORMED) response.putAllPrimaryProfileIds(resolvedPrimaryProfileIds);
    observer.onNext(response.build());
    observer.onCompleted();
  }

  @Override
  public void markAccountRegular(
      MarkAccountRegularRequest request,
      StreamObserver<MarkAccountRegularResponse> observer) {
    markRegularRequests.add(request);
    rpcCalls.add("MarkAccountRegular");
    if (fail(markRegularOutcome, observer)) return;
    observer.onNext(MarkAccountRegularResponse.getDefaultInstance());
    observer.onCompleted();
  }

  @Override
  public void setVerification(
      SetVerificationRequest request, StreamObserver<SetVerificationResponse> observer) {
    setVerificationRequests.add(request);
    rpcCalls.add("SetVerification");
    if (fail(setVerificationOutcome, observer)) return;
    SetVerificationResponse.Builder response = SetVerificationResponse.newBuilder();
    if (setVerificationOutcome != Outcome.MALFORMED) {
      response.setVerificationStatus(
          VerificationStatus.newBuilder().setProfileId(request.getProfileId())
              .setVerificationType(request.getVerificationType()).setBadge(request.getBadge()));
    }
    observer.onNext(response.build());
    observer.onCompleted();
  }

  @Override
  public void clearVerification(
      ClearVerificationRequest request, StreamObserver<ClearVerificationResponse> observer) {
    clearVerificationRequests.add(request);
    rpcCalls.add("ClearVerification");
    if (fail(clearVerificationOutcome, observer)) return;
    ClearVerificationResponse.Builder response = ClearVerificationResponse.newBuilder();
    if (clearVerificationOutcome != Outcome.MALFORMED) {
      response.setVerificationStatus(
          VerificationStatus.newBuilder().setProfileId(request.getProfileId()).setVerificationType("none"));
    }
    observer.onNext(response.build());
    observer.onCompleted();
  }

  @Override
  public void switchProfile(
      SwitchProfileRequest request, StreamObserver<SwitchProfileResponse> observer) {
    switchRequests.add(request);
    rpcCalls.add("SwitchProfile");
    if (fail(switchOutcome, observer)) return;
    SwitchProfileResponse.Builder response = SwitchProfileResponse.newBuilder();
    if (switchOutcome != Outcome.MALFORMED && switchedProfile != null) response.setProfile(switchedProfile);
    observer.onNext(response.build());
    observer.onCompleted();
  }

  private static boolean fail(Outcome outcome, StreamObserver<?> observer) {
    Status status = switch (outcome) {
      case INVALID_ARGUMENT -> Status.INVALID_ARGUMENT;
      case UNAVAILABLE -> Status.UNAVAILABLE;
      case NOT_FOUND -> Status.NOT_FOUND;
      case PERMISSION_DENIED -> Status.PERMISSION_DENIED;
      case UNAUTHENTICATED -> Status.UNAUTHENTICATED;
      case DEADLINE_EXCEEDED -> Status.DEADLINE_EXCEEDED;
      case FAILED_PRECONDITION -> Status.FAILED_PRECONDITION;
      default -> null;
    };
    if (status == null) return false;
    observer.onError(status.withDescription("test User " + status.getCode()).asRuntimeException());
    return true;
  }
}
