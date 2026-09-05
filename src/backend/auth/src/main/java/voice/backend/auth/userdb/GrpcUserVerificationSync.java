package voice.backend.auth.userdb;

import app.voice.user.v1.ClearVerificationRequest;
import app.voice.user.v1.SetVerificationRequest;
import app.voice.user.v1.UserServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.MetadataUtils;
import java.util.UUID;
import voice.backend.auth.service.AuthException;

/** S2S verification sync via User Service SetVerification / ClearVerification. */
public class GrpcUserVerificationSync implements UserVerificationSync {
  private static final Metadata.Key<String> INTERNAL_CALLER_HEADER =
      Metadata.Key.of("x-voice-internal-caller", Metadata.ASCII_STRING_MARSHALLER);

  private final UserServiceGrpc.UserServiceBlockingStub stub;

  public GrpcUserVerificationSync(ManagedChannel channel) {
    this(authenticatedStub(channel));
  }

  public GrpcUserVerificationSync(UserServiceGrpc.UserServiceBlockingStub stub) {
    this.stub = stub;
  }

  private static UserServiceGrpc.UserServiceBlockingStub authenticatedStub(ManagedChannel channel) {
    Metadata headers = new Metadata();
    headers.put(INTERNAL_CALLER_HEADER, "auth");
    return UserServiceGrpc.newBlockingStub(channel)
        .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers));
  }

  @Override
  public void setPersonalVerification(UUID profileId, String badge) {
    try {
      var req =
          SetVerificationRequest.newBuilder()
              .setProfileId(profileId.toString())
              .setVerificationType("personal")
              .setBadge(badge == null || badge.isBlank() ? "verified" : badge)
              .build();
      var status = stub.setVerification(req).getVerificationStatus();
      if (!profileId.toString().equals(status.getProfileId())
          || !"personal".equals(status.getVerificationType())) {
        throw new AuthException("verification_sync_failed");
      }
    } catch (StatusRuntimeException ex) {
      throw new AuthException("verification_sync_failed");
    }
  }

  @Override
  public void clearVerification(UUID profileId) {
    try {
      var status =
          stub.clearVerification(
                  ClearVerificationRequest.newBuilder().setProfileId(profileId.toString()).build())
              .getVerificationStatus();
      if (!profileId.toString().equals(status.getProfileId())
          || !"none".equals(status.getVerificationType())) {
        throw new AuthException("verification_sync_failed");
      }
    } catch (StatusRuntimeException ex) {
      throw new AuthException("verification_sync_failed");
    }
  }
}
