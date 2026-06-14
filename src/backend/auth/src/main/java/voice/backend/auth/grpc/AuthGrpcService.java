package voice.backend.auth.grpc;

import app.voice.auth.v1.GetE2EKeyBackupRequest;
import app.voice.auth.v1.GetE2EKeyBackupResponse;
import app.voice.auth.v1.PutE2EKeyBackupRequest;
import app.voice.auth.v1.PutE2EKeyBackupResponse;
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
import java.util.concurrent.atomic.AtomicReference;
import org.springframework.stereotype.Component;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.LoginCommand;
import voice.backend.auth.service.LogoutCommand;
import voice.backend.auth.service.RefreshCommand;
import voice.backend.auth.service.RegisterCommand;

@Component
public class AuthGrpcService extends AuthServiceGrpc.AuthServiceImplBase {
  private final AuthService authService;
  private final AtomicReference<String> lastAccessToken = new AtomicReference<>("");

  public AuthGrpcService(AuthService authService) {
    this.authService = authService;
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
  public void putE2EKeyBackup(
      PutE2EKeyBackupRequest request, StreamObserver<PutE2EKeyBackupResponse> responseObserver) {
    run(responseObserver, () -> {
      authService.putE2EKeyBackup(
          lastAccessToken(), request.getEncryptedBlob(), request.getPasswordHint());
      return PutE2EKeyBackupResponse.getDefaultInstance();
    });
  }

  @Override
  public void getE2EKeyBackup(
      GetE2EKeyBackupRequest request, StreamObserver<GetE2EKeyBackupResponse> responseObserver) {
    run(responseObserver, () -> {
      var backup = authService.getE2EKeyBackup(lastAccessToken());
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
      voice.backend.auth.service.AuthSession session =
          authService.switchActiveProfile(token, request.getProfileId(), request.getDeviceInfoJson());
      rememberAccess(session.accessToken());
      return SwitchActiveProfileResponse.newBuilder().setSession(toProto(session)).build();
    });
  }

  private <T> void run(StreamObserver<T> observer, GrpcCall<T> call) {
    try {
      observer.onNext(call.execute());
      observer.onCompleted();
    } catch (AuthException ex) {
      observer.onError(Status.UNAUTHENTICATED.withDescription(ex.getMessage()).asRuntimeException());
    }
  }

  private AuthSession toProto(voice.backend.auth.service.AuthSession session) {
    return AuthSession.newBuilder()
        .setAccessToken(session.accessToken())
        .setRefreshToken(session.refreshToken())
        .setExpiresInSeconds(session.expiresInSeconds())
        .setAccountId(session.accountId())
        .setProfileId(session.profileId())
        .build();
  }

  private TokenClaims toProto(voice.backend.auth.service.TokenClaims claims) {
    return TokenClaims.newBuilder()
        .setUserId(claims.userId())
        .setProfileId(claims.profileId() == null ? "" : claims.profileId())
        .addAllRoles(claims.roles())
        .setSubscriptionTier(claims.subscriptionTier())
        .setExpiresAt(Timestamp.newBuilder().setSeconds(claims.expiresAt().getEpochSecond()).setNanos(claims.expiresAt().getNano()))
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
}
