package voice.backend.auth.userdb;

import app.voice.user.v1.Profile;
import app.voice.user.v1.SwitchProfileRequest;
import app.voice.user.v1.UserServiceGrpc;
import io.grpc.Metadata;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.MetadataUtils;
import java.util.Locale;
import java.util.UUID;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.ProfileSwitchException;

/** Validates a requested profile by calling the existing User Service SwitchProfile contract. */
public final class GrpcProfileSwitchValidator implements ProfileSwitchValidator {
  private static final Metadata.Key<String> USER_ID_HEADER =
      Metadata.Key.of("x-voice-user-id", Metadata.ASCII_STRING_MARSHALLER);
  private static final Metadata.Key<String> PROFILE_ID_HEADER =
      Metadata.Key.of("x-voice-profile-id", Metadata.ASCII_STRING_MARSHALLER);
  private static final Metadata.Key<String> SUBSCRIPTION_TIER_HEADER =
      Metadata.Key.of("x-voice-subscription-tier", Metadata.ASCII_STRING_MARSHALLER);

  private final UserServiceGrpc.UserServiceBlockingStub stub;

  public GrpcProfileSwitchValidator(UserServiceGrpc.UserServiceBlockingStub stub) {
    this.stub = stub;
  }

  @Override
  public void validateOwnedSwitchable(UUID accountId, UUID profileId) {
    validateOwnedSwitchable(accountId, null, profileId, null);
  }

  @Override
  public void validateOwnedSwitchable(
      UUID accountId, UUID currentProfileId, UUID profileId, String subscriptionTier) {
    Metadata headers = new Metadata();
    headers.put(USER_ID_HEADER, accountId.toString());
    if (currentProfileId != null) {
      headers.put(PROFILE_ID_HEADER, currentProfileId.toString());
    }
    if (subscriptionTier != null && !subscriptionTier.isBlank()) {
      headers.put(SUBSCRIPTION_TIER_HEADER, subscriptionTier);
    }
    try {
      Profile profile =
          stub.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
              .switchProfile(
                  SwitchProfileRequest.newBuilder().setProfileId(profileId.toString()).build())
              .getProfile();
      validateResponse(accountId, profileId, profile);
    } catch (StatusRuntimeException ex) {
      throw mapStatus(ex);
    } catch (IllegalArgumentException ex) {
      throw malformed();
    }
  }

  private static void validateResponse(UUID accountId, UUID profileId, Profile profile) {
    UUID returnedProfileId = UUID.fromString(profile.getId());
    UUID returnedAccountId = UUID.fromString(profile.getAccountId());
    if (!profileId.equals(returnedProfileId) || !accountId.equals(returnedAccountId)) {
      throw malformed();
    }
    if (profile.hasFrozenAt()) {
      throw new ProfileSwitchException(
          "profile_frozen", ProfileSwitchException.Kind.PRECONDITION);
    }
  }

  private static RuntimeException mapStatus(StatusRuntimeException ex) {
    Status.Code code = ex.getStatus().getCode();
    if (code == Status.Code.NOT_FOUND) {
      return new ProfileSwitchException("profile_not_found", ProfileSwitchException.Kind.NOT_FOUND);
    }
    if (code == Status.Code.PERMISSION_DENIED || code == Status.Code.UNAUTHENTICATED) {
      return new ProfileSwitchException("profile_forbidden", ProfileSwitchException.Kind.FORBIDDEN);
    }
    if (code == Status.Code.FAILED_PRECONDITION) {
      String description = ex.getStatus().getDescription();
      String message =
          description != null && description.toLowerCase(Locale.ROOT).contains("deleted")
              ? "profile_deleted"
              : "profile_frozen";
      return new ProfileSwitchException(message, ProfileSwitchException.Kind.PRECONDITION);
    }
    return new AuthException("auth_unavailable");
  }

  private static ProfileSwitchException malformed() {
    return new ProfileSwitchException(
        "malformed_user_response", ProfileSwitchException.Kind.PRECONDITION);
  }
}
