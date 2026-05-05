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
import voice.backend.auth.service.LoginCommand;
import voice.backend.auth.service.LogoutCommand;
import voice.backend.auth.service.RefreshCommand;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.TokenClaims;

@RestController
@RequestMapping("/api/v1/auth")
public class AuthRestController {
  private final AuthService authService;

  public AuthRestController(AuthService authService) {
    this.authService = authService;
  }

  @PostMapping("/register")
  public SessionResponse register(@Valid @RequestBody RegisterRequest request) {
    return SessionResponse.from(authService.register(new RegisterCommand(
        request.email(), request.phone(), request.password(), request.guest(), request.deviceInfoJson())));
  }

  @PostMapping("/login")
  public SessionResponse login(@Valid @RequestBody LoginRequest request) {
    return SessionResponse.from(authService.login(new LoginCommand(
        request.email(), request.phone(), request.password(), request.deviceInfoJson())));
  }

  @PostMapping("/refresh")
  public SessionResponse refresh(@Valid @RequestBody RefreshRequest request) {
    return SessionResponse.from(authService.refresh(new RefreshCommand(request.refreshToken(), request.deviceInfoJson())));
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

  @GetMapping("/.well-known/jwks.json")
  public ResponseEntity<String> jwks() {
    return ResponseEntity.ok(authService.jwksJson());
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
      @JsonProperty("device_info_json") String deviceInfoJson) {}

  public record RefreshRequest(
      @JsonProperty("refresh_token") @NotBlank String refreshToken,
      @JsonProperty("device_info_json") String deviceInfoJson) {}

  public record LogoutRequest(@JsonProperty("refresh_token") @NotBlank String refreshToken) {}

  public record SessionResponse(
      @JsonProperty("access_token") String accessToken,
      @JsonProperty("refresh_token") String refreshToken,
      @JsonProperty("expires_in_seconds") long expiresInSeconds,
      @JsonProperty("account_id") String accountId,
      @JsonProperty("profile_id") String profileId) {
    public static SessionResponse from(AuthSession session) {
      return new SessionResponse(
          session.accessToken(),
          session.refreshToken(),
          session.expiresInSeconds(),
          session.accountId(),
          session.profileId());
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
}
