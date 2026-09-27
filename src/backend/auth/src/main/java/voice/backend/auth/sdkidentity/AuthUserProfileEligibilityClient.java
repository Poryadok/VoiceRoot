package voice.backend.auth.sdkidentity;

import app.voice.user.v1.GetSdkProfileEligibilityRequest;
import app.voice.user.v1.GetSdkProfileEligibilityResponse;
import app.voice.user.v1.UserServiceGrpc;
import com.google.protobuf.MessageLite;
import io.grpc.Metadata;
import io.grpc.stub.MetadataUtils;
import java.time.Duration;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.UUID;
import java.util.concurrent.TimeUnit;

/** Request-bound, TLS-only client for User's Auth-protected selected-profile read. */
public final class AuthUserProfileEligibilityClient implements SdkProfileEligibility {
  public static final String RPC = "/voice.user.v1.UserService/GetSdkProfileEligibility";
  private static final Metadata.Key<String> AUTHORIZATION =
      Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);
  private static final Metadata.Key<String> REQUEST_ID =
      Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER);
  private final UserServiceGrpc.UserServiceBlockingStub stub;
  private final AuthUserPrincipalIssuer issuer;
  private final Duration deadline;

  public AuthUserProfileEligibilityClient(UserServiceGrpc.UserServiceBlockingStub stub,
      AuthUserPrincipalIssuer issuer) {
    this(stub, issuer, Duration.ofSeconds(15));
  }

  public AuthUserProfileEligibilityClient(UserServiceGrpc.UserServiceBlockingStub stub,
      AuthUserPrincipalIssuer issuer, Duration deadline) {
    this.stub = java.util.Objects.requireNonNull(stub, "stub");
    this.issuer = java.util.Objects.requireNonNull(issuer, "issuer");
    if (deadline == null || deadline.isZero() || deadline.isNegative()) {
      throw new IllegalArgumentException("auth.user-grpc.deadline must be positive");
    }
    this.deadline = deadline;
  }

  @Override
  public Profile inspect(UUID accountId, UUID profileId) {
    try {
      if (accountId == null || profileId == null) throw denied();
      GetSdkProfileEligibilityRequest request = GetSdkProfileEligibilityRequest.newBuilder()
          .setAccountId(accountId.toString()).setProfileId(profileId.toString()).build();
      String requestId = UUID.randomUUID().toString();
      String token = issuer.issue(RPC, requestId, requestHash(request));
      Metadata headers = new Metadata();
      headers.put(AUTHORIZATION, "Bearer " + token);
      headers.put(REQUEST_ID, requestId);
      GetSdkProfileEligibilityResponse response = stub.withDeadlineAfter(deadline.toNanos(), TimeUnit.NANOSECONDS)
          .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers)).getSdkProfileEligibility(request);
      UUID returnedAccount = canonicalUuid(response.getAccountId());
      UUID returnedProfile = canonicalUuid(response.getProfileId());
      if (!accountId.equals(returnedAccount) || !profileId.equals(returnedProfile)
          || response.getProfileRevision() <= 0 || response.getDeleted() || response.getFrozen()) {
        throw denied();
      }
      return new Profile(returnedAccount, returnedProfile, response.getProfileRevision(),
          response.getDeleted(), response.getFrozen());
    } catch (SdkIdentityDeniedException denied) {
      throw denied;
    } catch (RuntimeException unavailableOrInvalid) {
      throw denied();
    }
  }

  static String requestHash(MessageLite request) {
    try {
      byte[] digest = MessageDigest.getInstance("SHA-256").digest(request.toByteArray());
      return "sha256:" + HexFormat.of().formatHex(digest);
    } catch (java.security.NoSuchAlgorithmException impossible) {
      throw new IllegalStateException("SHA-256 unavailable", impossible);
    }
  }

  private static UUID canonicalUuid(String value) {
    try {
      UUID parsed = UUID.fromString(value);
      if (!parsed.toString().equals(value)) throw denied();
      return parsed;
    } catch (RuntimeException malformed) {
      throw denied();
    }
  }

  private static SdkIdentityDeniedException denied() {
    return new SdkIdentityDeniedException();
  }
}
