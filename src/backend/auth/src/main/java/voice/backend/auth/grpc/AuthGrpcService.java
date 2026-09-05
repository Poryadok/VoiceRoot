package voice.backend.auth.grpc;

import app.voice.auth.v1.DeleteAccountRequest;
import app.voice.auth.v1.DeleteAccountResponse;
import app.voice.auth.v1.RestoreAccountRequest;
import app.voice.auth.v1.RestoreAccountResponse;
import app.voice.auth.v1.VerifyOTPRequest;
import app.voice.auth.v1.VerifyOTPResponse;
import app.voice.auth.v1.ConvertGuestRequest;
import app.voice.auth.v1.ConvertGuestResponse;
import app.voice.auth.v1.GetE2EKeyBackupRequest;
import app.voice.auth.v1.GetE2EKeyBackupResponse;
import app.voice.auth.v1.GetGuestReminderRequest;
import app.voice.auth.v1.GetGuestReminderResponse;
import app.voice.auth.v1.ListSessionsRequest;
import app.voice.auth.v1.ListSessionsResponse;
import app.voice.auth.v1.MarkGuestReminderShownRequest;
import app.voice.auth.v1.MarkGuestReminderShownResponse;
import app.voice.auth.v1.FilterDeletedAccountIDsRequest;
import app.voice.auth.v1.FilterDeletedAccountIDsResponse;
import app.voice.auth.v1.ResolvePhoneHashesRequest;
import app.voice.auth.v1.ResolvePhoneHashesResponse;
import app.voice.auth.v1.PhoneHashProfileMatch;
import app.voice.auth.v1.PutE2EKeyBackupRequest;
import app.voice.auth.v1.PutE2EKeyBackupResponse;
import app.voice.auth.v1.RevokeSessionRequest;
import app.voice.auth.v1.RevokeSessionResponse;
import app.voice.auth.v1.SessionInfo;
import app.voice.auth.v1.SetAccountStatusRequest;
import app.voice.auth.v1.SetAccountStatusResponse;
import app.voice.auth.v1.SwitchActiveProfileRequest;
import app.voice.auth.v1.SwitchActiveProfileResponse;
import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.AuthSession;
import app.voice.auth.v1.Enable2FARequest;
import app.voice.auth.v1.Enable2FAResponse;
import app.voice.auth.v1.GetJWKSRequest;
import app.voice.auth.v1.GetJWKSResponse;
import app.voice.auth.v1.LoginRequest;
import app.voice.auth.v1.LoginResponse;
import app.voice.auth.v1.LogoutRequest;
import app.voice.auth.v1.LogoutResponse;
import app.voice.auth.v1.RefreshTokenRequest;
import app.voice.auth.v1.RefreshTokenResponse;
import app.voice.auth.v1.RegisterRequest;
import app.voice.auth.v1.RegisterResponse;
import app.voice.auth.v1.TokenClaims;
import app.voice.auth.v1.ValidateTokenRequest;
import app.voice.auth.v1.ValidateTokenResponse;
import app.voice.auth.v1.Verify2FARequest;
import app.voice.auth.v1.Verify2FAResponse;
import com.google.protobuf.Timestamp;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.time.Instant;
import java.util.concurrent.atomic.AtomicReference;
import org.springframework.stereotype.Component;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.ConvertGuestCommand;
import voice.backend.auth.service.GuestReminderState;
import voice.backend.auth.service.LoginCommand;
import voice.backend.auth.service.LogoutCommand;
import voice.backend.auth.service.RefreshCommand;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.service.VerifyOtpCommand;

@Component
public class AuthGrpcService extends AuthServiceGrpc.AuthServiceImplBase {
  private final AuthService authService;
  private final OtpService otpService;
  private final AtomicReference<String> lastAccessToken = new AtomicReference<>("");

  public AuthGrpcService(AuthService authService, OtpService otpService) {
    this.authService = authService;
    this.otpService = otpService;
  }

  @Override
  public void register(RegisterRequest request, StreamObserver<RegisterResponse> responseObserver) {
    run(responseObserver, () -> {
      voice.backend.auth.service.AuthSession session = authService.register(
          new RegisterCommand(request.getEmail(), request.getPhone(), request.getPassword(), request.getGuest(), "{}"));
      rememberAccess(session.accessToken());
      return RegisterResponse.newBuilder().setSession(toProto(session)).build();
    });
  }

  @Override
  public void login(LoginRequest request, StreamObserver<LoginResponse> responseObserver) {
    run(responseObserver, () -> {
      voice.backend.auth.service.AuthSession session = authService.login(
          new LoginCommand(request.getEmail(), request.getPhone(), request.getPassword(), request.getTotpCode(), request.getDeviceInfoJson()));
      rememberAccess(session.accessToken());
      return LoginResponse.newBuilder().setSession(toProto(session)).build();
    });
  }

  @Override
  public void refreshToken(RefreshTokenRequest request, StreamObserver<RefreshTokenResponse> responseObserver) {
    run(responseObserver, () -> {
      voice.backend.auth.service.AuthSession session = authService.refresh(
          new RefreshCommand(request.getRefreshToken(), request.getDeviceInfoJson()));
      rememberAccess(session.accessToken());
      return RefreshTokenResponse.newBuilder().setSession(toProto(session)).build();
    });
  }

  @Override
  public void enable2FA(Enable2FARequest request, StreamObserver<Enable2FAResponse> responseObserver) {
    run(responseObserver, () -> {
      voice.backend.auth.service.TotpEnrollment enrollment =
          authService.enable2FA(lastAccessToken(), request.getPassword());
      return Enable2FAResponse.newBuilder()
          .setTotpUri(enrollment.totpUri())
          .setSecretBackupHint(enrollment.secretBackupHint())
          .addAllBackupCodes(enrollment.backupCodes())
          .build();
    });
  }

  @Override
  public void verify2FA(Verify2FARequest request, StreamObserver<Verify2FAResponse> responseObserver) {
    run(responseObserver, () -> {
      voice.backend.auth.service.AuthSession session = authService.verify2FA(lastAccessToken(), request.getTotpCode());
      rememberAccess(session.accessToken());
      return Verify2FAResponse.newBuilder().setSession(toProto(session)).build();
    });
  }

  @Override
  public void verifyOTP(VerifyOTPRequest request, StreamObserver<VerifyOTPResponse> responseObserver) {
    run(responseObserver, () -> {
      otpService.verifyOtp(
          new VerifyOtpCommand(
              null,
              null,
              request.getCode(),
              request.getOtpType(),
              resolveAccessToken()),
          authService);
      return VerifyOTPResponse.getDefaultInstance();
    });
  }

  @Override
  public void deleteAccount(DeleteAccountRequest request, StreamObserver<DeleteAccountResponse> responseObserver) {
    run(responseObserver, () -> {
      authService.deleteAccount(lastAccessToken(), request.getPassword(), request.getTotpCode());
      return DeleteAccountResponse.getDefaultInstance();
    });
  }

  @Override
  public void restoreAccount(
      RestoreAccountRequest request, StreamObserver<RestoreAccountResponse> responseObserver) {
    run(responseObserver, () -> {
      voice.backend.auth.service.AuthSession session = authService.restoreAccount(request.getToken());
      return RestoreAccountResponse.newBuilder().setSession(toProto(session)).build();
    });
  }

  @Override
  public void convertGuest(ConvertGuestRequest request, StreamObserver<ConvertGuestResponse> responseObserver) {
    run(responseObserver, () -> {
      voice.backend.auth.service.AuthSession session =
          authService.convertGuest(
              resolveAccessToken(),
              new ConvertGuestCommand(
                  request.hasEmail() ? request.getEmail() : null,
                  request.hasPhone() ? request.getPhone() : null,
                  request.getPassword()));
      rememberAccess(session.accessToken());
      return ConvertGuestResponse.newBuilder().setSession(toProto(session)).build();
    });
  }

  @Override
  public void logout(LogoutRequest request, StreamObserver<LogoutResponse> responseObserver) {
    run(responseObserver, () -> {
      authService.logout(new LogoutCommand(null, request.getRefreshToken()));
      return LogoutResponse.getDefaultInstance();
    });
  }

  @Override
  public void validateToken(ValidateTokenRequest request, StreamObserver<ValidateTokenResponse> responseObserver) {
    run(responseObserver, () -> ValidateTokenResponse.newBuilder().setClaims(toProto(authService.validate(request.getAccessToken()))).build());
  }

  @Override
  public void getJWKS(GetJWKSRequest request, StreamObserver<GetJWKSResponse> responseObserver) {
    run(responseObserver, () -> GetJWKSResponse.newBuilder().setKeysJson(authService.jwksJson()).build());
  }

  @Override
  public void resolvePhoneHashes(
      ResolvePhoneHashesRequest request, StreamObserver<ResolvePhoneHashesResponse> responseObserver) {
    run(responseObserver, () -> {
      ResolvePhoneHashesResponse.Builder builder = ResolvePhoneHashesResponse.newBuilder();
      authService
          .resolvePhoneHashes(request.getPhoneHashesList())
          .forEach(
              (hash, profileId) ->
                  builder.addMatches(
                      PhoneHashProfileMatch.newBuilder()
                          .setPhoneHash(hash)
                          .setProfileId(profileId)
                          .build()));
      return builder.build();
    });
  }

  @Override
  public void filterDeletedAccountIDs(
      FilterDeletedAccountIDsRequest request,
      StreamObserver<FilterDeletedAccountIDsResponse> responseObserver) {
    run(
        responseObserver,
        () ->
            FilterDeletedAccountIDsResponse.newBuilder()
                .addAllDeletedAccountIds(authService.filterDeletedAccountIds(request.getAccountIdsList()))
                .build());
  }

  @Override
  public void putE2EKeyBackup(
      PutE2EKeyBackupRequest request, StreamObserver<PutE2EKeyBackupResponse> responseObserver) {
    run(responseObserver, () -> {
      authService.putE2EKeyBackup(
          resolveAccessToken(), request.getEncryptedBlob(), request.getPasswordHint());
      return PutE2EKeyBackupResponse.getDefaultInstance();
    });
  }

  @Override
  public void getE2EKeyBackup(
      GetE2EKeyBackupRequest request, StreamObserver<GetE2EKeyBackupResponse> responseObserver) {
    run(responseObserver, () -> {
      var backup = authService.getE2EKeyBackup(resolveAccessToken());
      var builder = GetE2EKeyBackupResponse.newBuilder().setEncryptedBlob(backup.encryptedBlob());
      if (backup.passwordHint() != null && !backup.passwordHint().isBlank()) {
        builder.setPasswordHint(backup.passwordHint());
      }
      return builder.build();
    });
  }

  @Override
  public void setAccountStatus(SetAccountStatusRequest request, StreamObserver<SetAccountStatusResponse> responseObserver) {
    run(responseObserver, () -> {
      authService.setAccountStatus(request.getAccountId(), request.getStatus());
      return SetAccountStatusResponse.getDefaultInstance();
    });
  }

  @Override
  public void switchActiveProfile(
      SwitchActiveProfileRequest request, StreamObserver<SwitchActiveProfileResponse> responseObserver) {
    run(responseObserver, () -> {
      String token = request.getAccessToken();
      if (token == null || token.isBlank()) {
        token = lastAccessToken.get();
      } else if (!token.regionMatches(true, 0, "Bearer ", 0, 7)) {
        token = "Bearer " + token.trim();
      }
      String deviceInfo = request.getDeviceInfoJson();
      if (deviceInfo == null || deviceInfo.isBlank()) {
        deviceInfo = "{}";
      }
      voice.backend.auth.service.AuthSession session =
          authService.switchActiveProfile(token, request.getProfileId(), deviceInfo);
      rememberAccess(session.accessToken());
      return SwitchActiveProfileResponse.newBuilder().setSession(toProto(session)).build();
    });
  }

  @Override
  public void getGuestReminder(
      GetGuestReminderRequest request, StreamObserver<GetGuestReminderResponse> responseObserver) {
    run(responseObserver, () -> {
      GuestReminderState state = authService.getGuestReminder(resolveAccessToken());
      GetGuestReminderResponse.Builder builder =
          GetGuestReminderResponse.newBuilder().setShouldShow(state.shouldShow());
      if (state.lastShownAt() != null) {
        builder.setLastShownAt(toTimestamp(state.lastShownAt()));
      }
      return builder.build();
    });
  }

  @Override
  public void markGuestReminderShown(
      MarkGuestReminderShownRequest request,
      StreamObserver<MarkGuestReminderShownResponse> responseObserver) {
    run(responseObserver, () -> {
      GuestReminderState state = authService.markGuestReminderShown(resolveAccessToken());
      return MarkGuestReminderShownResponse.newBuilder()
          .setLastShownAt(toTimestamp(state.lastShownAt()))
          .build();
    });
  }

  @Override
  public void listSessions(
      ListSessionsRequest request, StreamObserver<ListSessionsResponse> responseObserver) {
    run(responseObserver, () -> {
      ListSessionsResponse.Builder builder = ListSessionsResponse.newBuilder();
      for (voice.backend.auth.service.ActiveSession session :
          authService.listSessions(resolveAccessToken())) {
        builder.addSessions(
            SessionInfo.newBuilder()
                .setId(session.id())
                .setDeviceInfoJson(session.deviceInfoJson() == null ? "{}" : session.deviceInfoJson())
                .setCreatedAt(toTimestamp(session.createdAt()))
                .setExpiresAt(toTimestamp(session.expiresAt()))
                .setCurrent(session.current())
                .build());
      }
      return builder.build();
    });
  }

  @Override
  public void revokeSession(
      RevokeSessionRequest request, StreamObserver<RevokeSessionResponse> responseObserver) {
    run(responseObserver, () -> {
      authService.revokeSession(resolveAccessToken(), request.getSessionId());
      return RevokeSessionResponse.getDefaultInstance();
    });
  }

  private <T> void run(StreamObserver<T> observer, GrpcCall<T> call) {
    try {
      observer.onNext(call.execute());
      observer.onCompleted();
    } catch (AuthException ex) {
      observer.onError(toGrpcStatus(ex).asRuntimeException());
    }
  }

  private static Status toGrpcStatus(AuthException ex) {
    return switch (ex.getMessage()) {
      case "validation_failed" -> Status.INVALID_ARGUMENT.withDescription(ex.getMessage());
      case "registration_conflict" -> Status.FAILED_PRECONDITION.withDescription(ex.getMessage());
      case "auth_unavailable" -> Status.UNAVAILABLE.withDescription(ex.getMessage());
      case "not_found" -> Status.NOT_FOUND.withDescription(ex.getMessage());
      default -> Status.UNAUTHENTICATED.withDescription(ex.getMessage());
    };
  }

  private AuthSession toProto(voice.backend.auth.service.AuthSession session) {
    return AuthSession.newBuilder()
        .setAccessToken(session.accessToken())
        .setRefreshToken(session.refreshToken())
        .setExpiresInSeconds(session.expiresInSeconds())
        .setAccountId(session.accountId())
        .setProfileId(session.profileId())
        .setAccountType(session.accountType() == null ? "regular" : session.accountType())
        .build();
  }

  private TokenClaims toProto(voice.backend.auth.service.TokenClaims claims) {
    return TokenClaims.newBuilder()
        .setUserId(claims.userId())
        .setProfileId(claims.profileId() == null ? "" : claims.profileId())
        .addAllRoles(claims.roles())
        .setSubscriptionTier(claims.subscriptionTier())
        .setExpiresAt(Timestamp.newBuilder().setSeconds(claims.expiresAt().getEpochSecond()).setNanos(claims.expiresAt().getNano()))
        .setAccountType(claims.normalizedAccountType())
        .build();
  }

  private static Timestamp toTimestamp(Instant instant) {
    return Timestamp.newBuilder()
        .setSeconds(instant.getEpochSecond())
        .setNanos(instant.getNano())
        .build();
  }

  private interface GrpcCall<T> {
    T execute();
  }

  private void rememberAccess(String accessToken) {
    if (accessToken != null && !accessToken.isBlank()) {
      lastAccessToken.set("Bearer " + accessToken);
    }
  }

  private String lastAccessToken() {
    String token = lastAccessToken.get();
    if (token == null || token.isBlank()) {
      throw new AuthException("invalid_token");
    }
    return token;
  }

  private String resolveAccessToken() {
    String authorization = AuthorizationServerInterceptor.AUTHORIZATION.get();
    if (authorization != null && !authorization.isBlank()) {
      return authorization;
    }
    return lastAccessToken();
  }
}
