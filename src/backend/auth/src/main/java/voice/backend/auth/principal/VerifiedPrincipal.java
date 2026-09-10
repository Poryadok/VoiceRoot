package voice.backend.auth.principal;

import io.grpc.Context;
import java.util.UUID;

/** Identity installed only after signed, request-bound principal verification. */
public record VerifiedPrincipal(
    String kind, String issuer, UUID accountId, UUID profileId, long sessionEpoch) {
  public static final String SERVICE = "service";
  public static final String DELEGATED_USER = "delegated_user";
  static final Context.Key<VerifiedPrincipal> CONTEXT = Context.key("auth-verified-principal");

  public static VerifiedPrincipal current() {
    return CONTEXT.get();
  }
}

