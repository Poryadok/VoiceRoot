package voice.backend.auth.sdkidentity;

import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import java.util.Map;
import java.util.UUID;
import org.springframework.context.annotation.Conditional;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/v1/auth/sdk/conversions")
@Conditional(SdkConversionConfiguration.Enabled.class)
public class SdkConversionRestController {
  private final SdkConversionService service;

  public SdkConversionRestController(SdkConversionService service) { this.service = service; }

  public record PrepareRequest(@NotNull UUID idempotencyKey, @NotNull UUID bindingId,
                               @NotBlank @Size(max = 4096) String deviceProof) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }

  public record StatusRequest(@NotNull Long issuedAt, @NotBlank @Size(max = 4096) String deviceProof) {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }

  public record EmptyRequest() {
    @com.fasterxml.jackson.annotation.JsonAnySetter
    public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown SDK field"); }
  }

  @PostMapping("/new")
  public SdkConversionService.Operation prepareNew(
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @Valid @RequestBody PrepareRequest request) {
    return service.prepareNew(bearer(authorization), request.deviceProof(), request.idempotencyKey(), request.bindingId());
  }

  @PostMapping("/existing")
  public SdkConversionService.Operation prepareExisting(
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @Valid @RequestBody PrepareRequest request) {
    return service.prepareExisting(bearer(authorization), request.deviceProof(), request.idempotencyKey(), request.bindingId());
  }

  @PostMapping("/{operationId}/status")
  public SdkConversionService.Operation status(@PathVariable UUID operationId, @Valid @RequestBody StatusRequest request) {
    return service.status(operationId, request.issuedAt(), request.deviceProof());
  }

  @PostMapping("/{operationId}/attach-new-target")
  public SdkConversionService.Operation attachNewTarget(@PathVariable UUID operationId,
      @RequestHeader(value = "Authorization", required = false) String authorization,
      @RequestBody(required = false) EmptyRequest request) {
    bearer(authorization);
    return service.attachNewTarget(operationId, authorization);
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
    return ResponseEntity.badRequest().body(Map.of("error", "invalid_sdk_request"));
  }

  private static String bearer(String header) {
    if (header == null || !header.startsWith("Bearer ") || header.length() <= 7 || header.length() > 16384) {
      throw new SdkIdentityDeniedException();
    }
    return header.substring(7);
  }
}
