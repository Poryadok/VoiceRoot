package voice.backend.auth.moderationauthority;

import io.grpc.Status;
import java.util.UUID;

/** Process-local phase authority; no SQL ready flag or caller UUID can open it. */
public final class AuthModerationBootAuthority {
  private final UUID bootId=UUID.randomUUID();
  private long generation;
  private boolean closed;
  private AuthModerationStartupVerifier.VerifiedAuthority authority;
  UUID bootId(){return bootId;}
  synchronized void install(AuthModerationStartupVerifier.VerifiedAuthority verified) {
    if(closed || verified==null || !bootId.equals(verified.bootId()))throw unavailable();
    if(authority!=null && !authority.same(verified))throw unavailable();
    authority=verified;
  }
  public synchronized Runnable acquire() {
    if(closed || authority==null)throw unavailable();
    long captured=generation;
    return ()->{synchronized(this){if(closed || authority==null || generation!=captured)throw unavailable();}};
  }
  public synchronized void close(){if(!closed){closed=true;generation++;authority=null;}}
  static io.grpc.StatusRuntimeException unavailable(){return Status.UNAVAILABLE.withDescription("Auth current-boot moderation authority is incomplete").asRuntimeException();}
}
