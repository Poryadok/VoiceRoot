package voice.backend.auth.sdkidentity;

import jakarta.validation.Valid;
import jakarta.validation.constraints.*;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/v1/auth/sdk/authorizations")
@ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
public class SdkAuthorizationRestController {
  private final SdkAuthorizationService service;

  public SdkAuthorizationRestController(SdkAuthorizationService service) { this.service = service; }

  public record StartRequest(@NotNull UUID idempotencyKey, @NotBlank @Size(max = 2048) String redirectUri,
      @NotNull @Pattern(regexp = "[A-Za-z0-9_-]{43}") String codeChallenge,
      @NotNull @Pattern(regexp = "[A-Za-z0-9_-]{43,128}") String state,
      @NotEmpty @Size(max = 16) Set<@NotBlank @Size(max = 64) String> scopes,
      @NotBlank @Size(max = 4096) String deviceProof) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }

  public record ApprovalRequest(@NotNull UUID profileId, @Positive long policyRevision) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }

  public record CodeRequest(@NotNull @Pattern(regexp = "[A-Za-z0-9_-]{43}") String code,
      @NotBlank @Size(max = 2048) String redirectUri,
      @NotNull @Pattern(regexp = "[A-Za-z0-9._~-]{43,128}") String codeVerifier,
      @NotBlank @Size(max = 4096) String deviceProof) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }

  public record PossessionRequest(@NotBlank @Size(max = 4096) String deviceProof) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }

  @PostMapping
  public SdkAuthorizationService.AuthorizationRequest start(
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @Valid @RequestBody StartRequest request) {
    return service.start(bearer(authorization), request.deviceProof(), request.idempotencyKey(),
        request.redirectUri(), request.codeChallenge(), request.state(), request.scopes());
  }

  @GetMapping("/{requestId}")
  public SdkAuthorizationService.ConsentView inspect(@PathVariable UUID requestId,
      @RequestHeader(value = "Authorization", required = false) String authorization) {
    bearer(authorization);
    return service.inspect(requestId, authorization);
  }

  @PostMapping("/{requestId}/approve")
  public SdkAuthorizationService.Approval approve(@PathVariable UUID requestId,
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @Valid @RequestBody ApprovalRequest request) {
    bearer(authorization);
    return service.approve(requestId, authorization, request.profileId(), request.policyRevision());
  }

  @PostMapping("/{requestId}/exchange")
  public SdkAuthorizationService.LinkedSession exchange(@PathVariable UUID requestId,
      @Valid @RequestBody CodeRequest request) {
    return service.exchange(requestId, request.code(), request.redirectUri(), request.codeVerifier(), request.deviceProof());
  }

  @PostMapping("/linked-session")
  public SdkAuthorizationService.LinkedSession linkedSession(
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @Valid @RequestBody PossessionRequest request) {
    return service.linkedSession(bearer(authorization), request.deviceProof());
  }

  @ExceptionHandler(SdkIdentityDeniedException.class)
  public ResponseEntity<Map<String, String>> denied() {
    return ResponseEntity.status(401).body(Map.of("error", "invalid_sdk_identity"));
  }

  @ExceptionHandler(SdkAuthorizationConflictException.class)
  public ResponseEntity<Map<String, String>> conflict() {
    return ResponseEntity.status(409).body(Map.of("error", "sdk_authorization_conflict"));
  }

  @ExceptionHandler({org.springframework.web.bind.MethodArgumentNotValidException.class,
      org.springframework.http.converter.HttpMessageNotReadableException.class,
      org.springframework.beans.TypeMismatchException.class})
  public ResponseEntity<Map<String, String>> invalidRequest() {
    // Never let default exception logging print rejected credentials or JSON content.
    return ResponseEntity.badRequest().body(Map.of("error", "invalid_sdk_request"));
  }

  private static String bearer(String header) {
    if (header == null || !header.startsWith("Bearer ") || header.length() <= 7 || header.length() > 16384) {
      throw new SdkIdentityDeniedException();
    }
    return header.substring(7);
  }
}
