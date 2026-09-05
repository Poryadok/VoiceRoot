package voice.backend.auth.userdb;

import app.voice.user.v1.EnsurePrimaryProfileRequest;
import app.voice.user.v1.MarkAccountRegularRequest;
import app.voice.user.v1.Profile;
import app.voice.user.v1.UserServiceGrpc;
import io.grpc.StatusRuntimeException;
import java.util.UUID;
import voice.backend.auth.service.AuthException;

/** User Service backed primary-profile lifecycle for Auth session issuance. */
public final class GrpcPrimaryProfileProvisioner implements PrimaryProfileProvisioner {
  private final UserServiceGrpc.UserServiceBlockingStub stub;

  public GrpcPrimaryProfileProvisioner(UserServiceGrpc.UserServiceBlockingStub stub) {
    this.stub = stub;
  }

  @Override
  public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
    try {
      Profile profile =
          stub.ensurePrimaryProfile(
                  EnsurePrimaryProfileRequest.newBuilder()
                      .setAccountId(accountId.toString())
                      .setDisplayHint(displayHint == null ? "" : displayHint)
                      .setIsGuestAccount(guestAccount)
                      .build())
              .getProfile();
      UUID profileId = parseUuid(profile.getId());
      UUID responseAccountId = parseUuid(profile.getAccountId());
      if (!accountId.equals(responseAccountId)
          || !profile.getIsPrimary()
          || profile.hasFrozenAt()) {
        throw new AuthException("malformed_user_response");
      }
      return profileId.toString();
    } catch (StatusRuntimeException ex) {
      throw new AuthException("auth_unavailable");
    } catch (IllegalArgumentException ex) {
      throw new AuthException("malformed_user_response");
    }
  }

  @Override
  public void clearGuestAccountFlag(UUID accountId) {
    try {
      stub.markAccountRegular(
          MarkAccountRegularRequest.newBuilder().setAccountId(accountId.toString()).build());
    } catch (StatusRuntimeException ex) {
      throw new AuthException("auth_unavailable");
    }
  }

  private static UUID parseUuid(String value) {
    if (value == null || value.isBlank()) {
      throw new IllegalArgumentException("missing UUID");
    }
    return UUID.fromString(value);
  }
}
