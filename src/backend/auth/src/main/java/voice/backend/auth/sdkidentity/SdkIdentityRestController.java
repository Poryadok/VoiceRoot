package voice.backend.auth.sdkidentity;

import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import com.fasterxml.jackson.core.StreamReadFeature;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.core.JsonFactory;
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
  private static final ObjectMapper LIFECYCLE_JSON = new ObjectMapper(
      JsonFactory.builder().enable(StreamReadFeature.STRICT_DUPLICATE_DETECTION).build())
      .enable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES);
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
  public record DeviceKeyChallengeRequest(@NotBlank String purpose, @NotNull UUID applicationId,
      @NotNull UUID environmentId, @NotBlank @Size(max = 2048) String publicJwk, UUID replacesDeviceId) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }
  public record DeviceKeyRotateRequest(@NotNull UUID challengeId, @NotBlank @Size(max = 16384) String providerToken,
      @NotBlank @Size(max = 4096) String currentKeyProof, @NotBlank @Size(max = 4096) String newKeyProof,
      @NotNull UUID requestId) {}
  public record DeviceKeyRecoverRequest(@NotNull UUID challengeId, @NotBlank @Size(max = 16384) String providerToken,
      @NotBlank @Size(max = 4096) String newKeyProof, @NotNull UUID requestId) {}
  public record DeviceKeyRevokeRequest(@NotBlank @Size(max = 16384) String providerToken,
      @NotBlank @Size(max = 4096) String currentKeyProof, @NotNull UUID requestId) {}

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

  @PostMapping("/device-keys/challenges")
  public SdkIdentityService.Challenge deviceKeyChallenge(
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @Valid @RequestBody DeviceKeyChallengeRequest request) {
    if ("rotate".equals(request.purpose())) {
      return service.deviceKeyChallenge("rotate", request.applicationId(), request.environmentId(),
          bearer(authorization), null, request.publicJwk());
    }
    if ("recover".equals(request.purpose()) && authorization == null) {
      return service.deviceKeyChallenge("recover", request.applicationId(), request.environmentId(),
          null, request.replacesDeviceId(), request.publicJwk());
    }
    throw new SdkIdentityDeniedException();
  }

  @PostMapping("/device-keys/rotate")
  public SdkIdentityService.DeviceKeyResult rotate(@RequestBody byte[] rawBody) {
    DeviceKeyRotateRequest request = lifecycleRequest(rawBody, DeviceKeyRotateRequest.class);
    return service.rotate(request.challengeId(), request.providerToken(), request.currentKeyProof(),
        request.newKeyProof(), request.requestId(), rawBody);
  }

  @PostMapping("/device-keys/recover")
  public SdkIdentityService.DeviceKeyResult recover(@RequestBody byte[] rawBody) {
    DeviceKeyRecoverRequest request = lifecycleRequest(rawBody, DeviceKeyRecoverRequest.class);
    return service.recover(request.challengeId(), request.providerToken(), request.newKeyProof(),
        request.requestId(), rawBody);
  }

  @DeleteMapping("/devices/{deviceId}")
  public SdkIdentityService.DeviceKeyResult revokeDevice(@PathVariable UUID deviceId,
      @RequestBody byte[] rawBody) {
    DeviceKeyRevokeRequest request = lifecycleRequest(rawBody, DeviceKeyRevokeRequest.class);
    return service.revokeDevice(deviceId, request.providerToken(), request.currentKeyProof(),
        request.requestId(), rawBody);
  }

  @PostMapping(value = "/device-authority", consumes = "text/plain", produces = "application/jwt")
  public ResponseEntity<String> deviceAuthority(
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @RequestBody byte[] rawProof) {
    if (rawProof == null || rawProof.length == 0 || rawProof.length > 4096) {
      throw new SdkIdentityDeniedException();
    }
    String proof = new String(rawProof, java.nio.charset.StandardCharsets.US_ASCII);
    if (!java.util.Arrays.equals(rawProof, proof.getBytes(java.nio.charset.StandardCharsets.US_ASCII))) {
      throw new SdkIdentityDeniedException();
    }
    String assertion = service.deviceAuthority(bearer(authorization), proof, rawProof);
    return ResponseEntity.ok().contentType(org.springframework.http.MediaType.valueOf("application/jwt"))
        .body(assertion);
  }

  @ExceptionHandler(SdkIdentityDeniedException.class)
  public ResponseEntity<Map<String, String>> denied() {
    return ResponseEntity.status(401).body(Map.of("error", "invalid_sdk_identity"));
  }

  @ExceptionHandler(SdkDeviceKeyConflictException.class)
  public ResponseEntity<Map<String, String>> lifecycleConflict() {
    return ResponseEntity.status(409).body(Map.of("error", "device_key_request_conflict"));
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

  private static <T> T lifecycleRequest(byte[] body, Class<T> type) {
    if (body == null || body.length == 0 || body.length > 65536) throw new SdkIdentityDeniedException();
    try { return LIFECYCLE_JSON.readValue(body, type); }
    catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }
}
