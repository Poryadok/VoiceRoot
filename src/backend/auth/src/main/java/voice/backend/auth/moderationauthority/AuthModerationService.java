package voice.backend.auth.moderationauthority;

import app.voice.automod.v1.*;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.util.Set;
import java.util.UUID;
import java.util.function.Supplier;
import org.springframework.dao.DataAccessException;
import voice.backend.auth.principal.VerifiedPrincipal;

/** Registered exclusively on Auth's request-bound, mTLS private listener. */
public final class AuthModerationService extends AuthModerationAuthorityServiceGrpc.AuthModerationAuthorityServiceImplBase {
  private final AuthModerationStore store;
  public AuthModerationService(AuthModerationStore store){this.store=store;}
  @Override public void resolveModerationSubject(ResolveModerationSubjectRequest request,StreamObserver<ResolveModerationSubjectResponse> observer){
    respond(observer,()->{require(Set.of("messaging","moderation"));return store.resolve(uuid(request.getAccountId()),request.hasSubjectId()?uuid(request.getSubjectId()):null);});
  }
  @Override public void listTerminalModerationSubjects(ListTerminalModerationSubjectsRequest request,StreamObserver<ListTerminalModerationSubjectsResponse> observer){
    respond(observer,()->{require(Set.of("messaging","moderation"));if(!request.getUnknownFields().asMap().isEmpty())throw Status.INVALID_ARGUMENT.asRuntimeException();return store.terminalSnapshot();});
  }
  @Override public void reconcileModerationRestriction(ReconcileModerationRestrictionRequest request,StreamObserver<ReconcileModerationRestrictionResponse> observer){
    respond(observer,()->{require(Set.of("moderation"));if(request.getProtocolVersion()!=1)throw Status.INVALID_ARGUMENT.asRuntimeException();return store.reconcile(uuid(request.getAccountId()),uuid(request.getSubjectId()),request.getRevision(),request.getModerationBanned());});
  }
  private static UUID uuid(String raw){try{UUID id=UUID.fromString(raw);if(!id.toString().equals(raw))throw new IllegalArgumentException();return id;}catch(IllegalArgumentException ex){throw Status.INVALID_ARGUMENT.withDescription("invalid authority identifier").asRuntimeException();}}
  private static void require(Set<String> callers){var p=VerifiedPrincipal.current();if(p==null || !VerifiedPrincipal.SERVICE.equals(p.kind()) || !callers.contains(p.issuer()))throw Status.UNAUTHENTICATED.withDescription("private authority required").asRuntimeException();}
  private static <T> void respond(StreamObserver<T> observer,Supplier<T> action){try{T value=action.get();observer.onNext(value);observer.onCompleted();}catch(DataAccessException ex){observer.onError(Status.FAILED_PRECONDITION.withDescription("moderation authority conflict or unavailable").asRuntimeException());}catch(RuntimeException ex){observer.onError(ex instanceof io.grpc.StatusRuntimeException?ex:Status.INTERNAL.withDescription("moderation authority unavailable").asRuntimeException());}}
}
