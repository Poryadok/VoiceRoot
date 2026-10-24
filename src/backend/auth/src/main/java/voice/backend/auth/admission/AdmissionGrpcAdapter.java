package voice.backend.auth.admission;

import app.voice.auth.v1.*;
import com.google.protobuf.ByteString;
import io.grpc.Context;
import io.grpc.Status;
import java.util.HexFormat;
import java.util.UUID;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.VerifiedPrincipal;

/** Exact additive wire adapter; the protected listener authenticates the whole request first. */
public final class AdmissionGrpcAdapter {
  private static final java.util.concurrent.ScheduledExecutorService TIMER = java.util.concurrent.Executors.newSingleThreadScheduledExecutor(r -> {
    Thread t=new Thread(r,"admission-deadline"); t.setDaemon(true); return t;
  });
  private AdmissionGrpcAdapter() {}
  public static ObserveSpaceAdmissionSubjectResponse observe(ObserveSpaceAdmissionSubjectRequest req, AdmissionObservation service) {
    var bounded=Context.current().withDeadlineAfter(2,java.util.concurrent.TimeUnit.SECONDS,TIMER);
    try { return bounded.call(() -> observeBounded(req,service)); }
    catch (RuntimeException error) { throw error; }
    catch (Exception error) { throw Status.UNAVAILABLE.withDescription("admission authority unavailable").asRuntimeException(); }
    finally { bounded.cancel(null); }
  }
  private static ObserveSpaceAdmissionSubjectResponse observeBounded(ObserveSpaceAdmissionSubjectRequest req, AdmissionObservation service) {
    var actor = VerifiedPrincipal.current();
    if (actor == null || !"service".equals(actor.kind()) || !"space".equals(actor.issuer()))
      throw Status.UNAUTHENTICATED.withDescription("Space principal required").asRuntimeException();
    if (req == null || !req.getUnknownFields().asMap().isEmpty() || req.getProtocolVersion() != 1
        || req.getAdmissionBindingSha256().size() != 32) throw invalid();
    AdmissionObservation.Phase phase = switch (req.getPhase()) {
      case SPACE_ADMISSION_PHASE_START -> AdmissionObservation.Phase.START;
      case SPACE_ADMISSION_PHASE_READ -> AdmissionObservation.Phase.READ;
      case SPACE_ADMISSION_PHASE_SUBMIT -> AdmissionObservation.Phase.SUBMIT;
      case SPACE_ADMISSION_PHASE_APPROVE -> AdmissionObservation.Phase.APPROVE;
      case SPACE_ADMISSION_PHASE_FIRST_PUBLISH -> AdmissionObservation.Phase.FIRST_PUBLISH;
      case SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY -> AdmissionObservation.Phase.SET_ENTRY_POLICY;
      default -> throw invalid();
    };
    AdmissionObservation.Role role = switch (req.getSubjectRole()) {
      case SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT -> AdmissionObservation.Role.APPLICANT;
      case SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER -> AdmissionObservation.Role.REVIEWER;
      case SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR -> AdmissionObservation.Role.SETTINGS_ACTOR;
      default -> throw invalid();
    };
    if (service == null) throw Status.UNAVAILABLE.withDescription("admission authority unavailable").asRuntimeException();
    if (Context.current().isCancelled()) throw Status.DEADLINE_EXCEEDED.asRuntimeException();
    var result = service.observe(new AdmissionObservation.Request(phase, role, id(req.getAccountId()), id(req.getProfileId()),
        id(req.getSpaceId()), HexFormat.of().formatHex(req.getAdmissionBindingSha256().toByteArray()),
        AuthPrincipalServerInterceptor.requestHash(req), req.hasInteractiveSessionEpoch() ? req.getInteractiveSessionEpoch() : null));
    if (Context.current().isCancelled()) throw Status.DEADLINE_EXCEEDED.asRuntimeException();
    return ObserveSpaceAdmissionSubjectResponse.newBuilder()
        .setRequestBindingSha256(ByteString.copyFrom(HexFormat.of().parseHex(result.requestHash().substring(7))))
        .setAccountId(result.account().toString()).setProfileId(result.profile().toString()).setAccountType(result.type())
        .setAccountSecurityRevision(result.accountRevision()).setSessionEpochFloor(result.floor()).setProfileRevision(result.profileRevision())
        .setDecision(SpaceAdmissionDecision.SPACE_ADMISSION_DECISION_ELIGIBLE).build();
  }
  private static UUID id(String value) {
    try { UUID id=UUID.fromString(value); if (!id.toString().equals(value) || id.equals(new UUID(0,0))) throw invalid(); return id; }
    catch (IllegalArgumentException e) { throw invalid(); }
  }
  private static io.grpc.StatusRuntimeException invalid() { return Status.INVALID_ARGUMENT.withDescription("invalid admission request").asRuntimeException(); }
}
