package voice.backend.auth.sdkidentity;

import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import java.util.Map;
import java.util.UUID;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

/** Dedicated bootstrap credentials never enter the regular/guest token pipeline. */
@RestController
@RequestMapping("/api/v1/auth/sdk")
@ConditionalOnProperty(prefix = "auth.sdk-identity", name = "enabled", havingValue = "true")
public class SdkIdentityRestController {
  private final SdkIdentityService service;

  public SdkIdentityRestController(SdkIdentityService service) { this.service = service; }

  public record ChallengeRequest(@NotNull UUID applicationId, @NotNull UUID environmentId,
                                 @NotBlank @Size(max = 2048) String devicePublicJwk) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }
  public record ExchangeRequest(@NotNull UUID challengeId,
                                @NotBlank @Size(max = 16384) String providerToken,
                                @NotBlank @Size(max = 16384) String gameTicket,
                                @NotBlank @Size(max = 4096) String deviceProof) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }
  public record PossessionRequest(@NotBlank @Size(max = 4096) String deviceProof) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }

  @PostMapping("/challenges")
  public SdkIdentityService.Challenge challenge(@Valid @RequestBody ChallengeRequest request) {
    return service.challenge(request.applicationId(), request.environmentId(), request.devicePublicJwk());
  }

  @PostMapping("/exchange")
  public SdkIdentityService.Session exchange(@Valid @RequestBody ExchangeRequest request) {
    return service.exchange(request.challengeId(), request.providerToken(), request.gameTicket(), request.deviceProof());
  }

  @PostMapping("/session")
  public SdkIdentityService.Session session(
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @Valid @RequestBody PossessionRequest request) {
    return service.session(bearer(authorization), request.deviceProof());
  }

  @PostMapping("/revoke")
  public ResponseEntity<Void> revoke(
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @Valid @RequestBody PossessionRequest request) {
    service.revoke(bearer(authorization), request.deviceProof());
    return ResponseEntity.noContent().build();
  }

  @ExceptionHandler(SdkIdentityDeniedException.class)
  public ResponseEntity<Map<String, String>> denied() {
    return ResponseEntity.status(401).body(Map.of("error", "invalid_sdk_identity"));
  }

  @ExceptionHandler({org.springframework.web.bind.MethodArgumentNotValidException.class,
      org.springframework.http.converter.HttpMessageNotReadableException.class})
  public ResponseEntity<Map<String, String>> invalidRequest() {
    // The default Spring resolver logs rejected values, including raw provider/device credentials.
    return ResponseEntity.badRequest().body(Map.of("error", "invalid_sdk_request"));
  }

  private static String bearer(String header) {
    if (header == null || !header.startsWith("Bearer ") || header.length() <= 7 || header.length() > 256) {
      throw new SdkIdentityDeniedException();
    }
    return header.substring(7);
  }
}
