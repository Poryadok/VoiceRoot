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
public class GrpcUserVerificationSync implements UserVerificationSync, AutoCloseable {
  private static final Metadata.Key<String> INTERNAL_CALLER_HEADER =
      Metadata.Key.of("x-voice-internal-caller", Metadata.ASCII_STRING_MARSHALLER);

  private final ManagedChannel channel;
  private final UserServiceGrpc.UserServiceBlockingStub stub;

  public GrpcUserVerificationSync(ManagedChannel channel) {
    this.channel = channel;
    Metadata headers = new Metadata();
    headers.put(INTERNAL_CALLER_HEADER, "auth");
    this.stub =
        UserServiceGrpc.newBlockingStub(channel)
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
      stub.setVerification(req);
    } catch (StatusRuntimeException ex) {
      throw new AuthException("verification_sync_failed");
    }
  }

  @Override
  public void clearVerification(UUID profileId) {
    try {
      stub.clearVerification(
          ClearVerificationRequest.newBuilder().setProfileId(profileId.toString()).build());
    } catch (StatusRuntimeException ex) {
      throw new AuthException("verification_sync_failed");
    }
  }

  @Override
  public void close() {
    if (channel != null) {
      channel.shutdown();
    }
  }
}
