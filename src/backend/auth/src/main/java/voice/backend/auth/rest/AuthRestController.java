package voice.backend.auth.rest;

import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.List;
import java.util.Map;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.AuthSession;
import voice.backend.auth.service.ConvertGuestCommand;
import voice.backend.auth.service.LinkedAccountsService;
import voice.backend.auth.service.LoginCommand;
import voice.backend.auth.service.LogoutCommand;
import voice.backend.auth.service.ProfileSwitchException;
import voice.backend.auth.service.RefreshCommand;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.TokenClaims;
import voice.backend.auth.service.TotpEnrollment;

@RestController
@RequestMapping("/api/v1/auth")
public class AuthRestController {
  private final AuthService authService;
  private final LinkedAccountsService linkedAccountsService;

  public AuthRestController(AuthService authService, LinkedAccountsService linkedAccountsService) {
    this.authService = authService;
    this.linkedAccountsService = linkedAccountsService;
  }

  @PostMapping("/register")
  public SessionEnvelope register(@Valid @RequestBody RegisterRequest request) {
    return SessionEnvelope.from(authService.register(new RegisterCommand(
        request.email(), request.phone(), request.password(), request.guest(), request.deviceInfoJson())));
  }

  @PostMapping("/login")
  public SessionEnvelope login(@Valid @RequestBody LoginRequest request) {
    return SessionEnvelope.from(authService.login(new LoginCommand(
        request.email(), request.phone(), request.password(), request.totpCode(), request.deviceInfoJson())));
  }

  @PostMapping("/2fa/enable")
  public Enable2FAResponse enable2FA(
      @RequestHeader(name = "Authorization", required = false) String authorization,
      @Valid @RequestBody Enable2FARequest request) {
    TotpEnrollment enrollment = authService.enable2FA(authorization, request.password());
    return Enable2FAResponse.from(enrollment);
  }

  @PostMapping("/2fa/verify")
  public SessionEnvelope verify2FA(
      @RequestHeader(name = "Authorization", required = false) String authorization,
      @Valid @RequestBody Verify2FARequest request) {
    return SessionEnvelope.from(authService.verify2FA(authorization, request.totpCode()));
  }

  @PostMapping("/refresh")
  public SessionEnvelope refresh(@Valid @RequestBody RefreshRequest request) {
    return SessionEnvelope.from(authService.refresh(new RefreshCommand(request.refreshToken(), request.deviceInfoJson())));
  }

  @PostMapping("/logout")
  public ResponseEntity<Void> logout(
      @RequestHeader(name = "Authorization", required = false) String authorization,
      @Valid @RequestBody LogoutRequest request) {
    authService.logout(new LogoutCommand(authorization, request.refreshToken()));
    return ResponseEntity.noContent().build();
  }

  @PostMapping("/validate")
  public ClaimsResponse validate(@RequestHeader(name = "Authorization", required = false) String authorization) {
    return ClaimsResponse.from(authService.validate(authorization));
  }

  @PostMapping("/switch-profile")
  public SessionBody switchProfile(
      @RequestHeader(name = "Authorization", required = false) String authorization,
      @Valid @RequestBody SwitchProfileRequest request) {
    AuthSession session =
        authService.switchActiveProfile(authorization, request.profileId(), "{}");
    return SessionBody.from(session);
  }

  @PostMapping("/convert-guest")
  public SessionEnvelope convertGuest(
      @RequestHeader(name = "Authorization", required = false) String authorization,
      @Valid @RequestBody ConvertGuestRequest request) {
    return SessionEnvelope.from(
        authService.convertGuest(
            authorization, new ConvertGuestCommand(request.email(), request.phone(), request.password())));
  }

  @GetMapping("/linked-accounts")
  public Map<String, Object> listLinkedAccounts() {
    return Map.of("linked_accounts", List.of());
  }

  @PostMapping("/linked-accounts/twitch/callback")
  public Map<String, String> twitchCallback(
      @RequestHeader(name = "Authorization", required = false) String authorization,
      @Valid @RequestBody OAuthCallbackRequest request) {
    TokenClaims claims = authService.validate(authorization);
    var result =
        linkedAccountsService.completeTwitchCallback(
            java.util.UUID.fromString(claims.profileId()), request.code());
    return Map.of("verification_type", result.verificationType(), "badge", result.badge());
  }

  @PostMapping("/linked-accounts/twitch/unlink")
  public ResponseEntity<Void> twitchUnlink(
      @RequestHeader(name = "Authorization", required = false) String authorization) {
    TokenClaims claims = authService.validate(authorization);
    linkedAccountsService.unlinkTwitch(java.util.UUID.fromString(claims.profileId()));
    return ResponseEntity.noContent().build();
  }

  @PostMapping("/linked-accounts/twitch/link")
  public Map<String, String> twitchLinkStart() {
    return Map.of("authorization_url", "https://id.twitch.tv/oauth2/authorize");
  }

  @GetMapping("/.well-known/jwks.json")
  public ResponseEntity<String> jwks() {
    return ResponseEntity.ok(authService.jwksJson());
  }

  @ExceptionHandler(ProfileSwitchException.class)
  public ResponseEntity<Map<String, String>> profileSwitchError(ProfileSwitchException ex) {
    HttpStatus status =
        switch (ex.kind()) {
          case FORBIDDEN -> HttpStatus.FORBIDDEN;
          case PRECONDITION -> HttpStatus.PRECONDITION_FAILED;
          case NOT_FOUND -> HttpStatus.NOT_FOUND;
        };
    return ResponseEntity.status(status).body(Map.of("error", ex.getMessage()));
  }

  @ExceptionHandler(AuthException.class)
  public ResponseEntity<Map<String, String>> authError(AuthException ex) {
    HttpStatus status = switch (ex.getMessage()) {
      case "validation_failed" -> HttpStatus.BAD_REQUEST;
      case "auth_unavailable" -> HttpStatus.SERVICE_UNAVAILABLE;
      default -> HttpStatus.UNAUTHORIZED;
    };
    return ResponseEntity.status(status).body(Map.of("error", ex.getMessage()));
  }

  public record RegisterRequest(
      String email,
      String phone,
      @NotBlank String password,
      boolean guest,
      @JsonProperty("device_info_json") String deviceInfoJson) {}

  public record LoginRequest(
      String email,
      String phone,
      @NotBlank String password,
      @JsonProperty("totp_code") String totpCode,
      @JsonProperty("device_info_json") String deviceInfoJson) {}

  public record Enable2FARequest(@NotBlank String password) {}

  public record Verify2FARequest(@JsonProperty("totp_code") @NotBlank String totpCode) {}

  public record RefreshRequest(
      @JsonProperty("refresh_token") @NotBlank String refreshToken,
      @JsonProperty("device_info_json") String deviceInfoJson) {}

  public record LogoutRequest(@JsonProperty("refresh_token") @NotBlank String refreshToken) {}

  public record SwitchProfileRequest(@JsonProperty("profile_id") @NotBlank String profileId) {}

  public record ConvertGuestRequest(
      String email, String phone, @NotBlank String password) {}

  public record OAuthCallbackRequest(
      @NotBlank String code, @JsonProperty("redirect_uri") String redirectUri) {}

  /** Aligns with proto `RegisterResponse` / `AuthSession` nesting. */
  public record SessionEnvelope(@JsonProperty("session") SessionBody session) {
    public static SessionEnvelope from(AuthSession session) {
      return new SessionEnvelope(SessionBody.from(session));
    }
  }

  public record SessionBody(
      @JsonProperty("access_token") String accessToken,
      @JsonProperty("refresh_token") String refreshToken,
      @JsonProperty("expires_in_seconds") long expiresInSeconds,
      @JsonProperty("account_id") String accountId,
      @JsonProperty("profile_id") String profileId,
      @JsonProperty("account_type") String accountType) {
    public static SessionBody from(AuthSession session) {
      String accountType = session.accountType();
      if (accountType == null || accountType.isBlank()) {
        accountType = "regular";
      }
      return new SessionBody(
          session.accessToken(),
          session.refreshToken(),
          session.expiresInSeconds(),
          session.accountId(),
          session.profileId(),
          accountType);
    }
  }

  public record ClaimsResponse(
      @JsonProperty("user_id") String userId,
      @JsonProperty("profile_id") String profileId,
      List<String> roles,
      @JsonProperty("subscription_tier") String subscriptionTier,
      @JsonProperty("expires_at") String expiresAt,
      String jti) {
    public static ClaimsResponse from(TokenClaims claims) {
      return new ClaimsResponse(
          claims.userId(),
          claims.profileId(),
          claims.roles(),
          claims.subscriptionTier(),
          claims.expiresAt().toString(),
          claims.jti());
    }
  }

  public record Enable2FAResponse(
      @JsonProperty("totp_uri") String totpUri,
      @JsonProperty("secret_backup_hint") String secretBackupHint,
      @JsonProperty("backup_codes") List<String> backupCodes) {
    public static Enable2FAResponse from(TotpEnrollment enrollment) {
      return new Enable2FAResponse(enrollment.totpUri(), enrollment.secretBackupHint(), enrollment.backupCodes());
    }
  }
}
