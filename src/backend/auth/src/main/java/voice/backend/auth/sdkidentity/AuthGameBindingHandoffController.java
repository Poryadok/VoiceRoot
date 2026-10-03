package voice.backend.auth.sdkidentity;

import com.fasterxml.jackson.annotation.JsonAnySetter;
import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import jakarta.validation.constraints.Size;
import java.util.Map;
import java.util.UUID;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.ResponseEntity;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/internal/v1/auth/game-bindings/handoffs")
@ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
public class AuthGameBindingHandoffController {
  private final AuthGameBindingHandoffService service;
  private final SdkAuthorizationService authorizations;

  public AuthGameBindingHandoffController(AuthGameBindingHandoffService service, SdkAuthorizationService authorizations) {
    this.service = service;
    this.authorizations = authorizations;
  }

  public record ExchangeRequest(@JsonProperty("challenge_id") @NotNull UUID challengeId,
      @JsonProperty("operation_id") @NotNull UUID operationId,
      @JsonProperty("code") @NotBlank @Pattern(regexp = "[A-Za-z0-9_-]{43}") String code,
      @JsonProperty("code_verifier") @NotBlank @Size(min = 43, max = 128) String codeVerifier,
      @JsonProperty("device_proof") @NotBlank @Size(max = 4096) String deviceProof) {
    @JsonAnySetter public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown field"); }
  }
  public record ExchangeResponse(@JsonProperty("handoff_jws") String handoffJws) {}

  public record ClaimRequest(@JsonProperty("operation_id") @NotNull UUID operationId,
      @JsonProperty("request_sha256") @NotNull @Pattern(regexp = "[0-9a-f]{64}") String requestSha256,
      @JsonProperty("handoff_jws") @NotBlank @Size(max = 8192) String handoffJws,
      @JsonProperty("device_proof") @NotBlank @Size(max = 4096) String deviceProof) {
    @JsonAnySetter public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown field"); }
  }
  public record ClaimResponse(@JsonProperty("claim_id") UUID claimId, @JsonProperty("operation_id") UUID operationId,
      @JsonProperty("assertion_jti") UUID assertionJti, @JsonProperty("expires_at") java.time.Instant expiresAt) {}
  public record CompletionRequest(@JsonProperty("claim_id") @NotNull UUID claimId,
      @JsonProperty("operation_id") @NotNull UUID operationId,
      @JsonProperty("outcome") @NotBlank @Pattern(regexp = "succeeded|failed") String outcome,
      @JsonProperty("binding_id") UUID bindingId) {
    @JsonAnySetter public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown field"); }
  }
  public record CompletionResponse(@JsonProperty("claim_id") UUID claimId,
      @JsonProperty("operation_id") UUID operationId, @JsonProperty("outcome") String outcome,
      @JsonProperty("binding_id") UUID bindingId, String status) {}
  public record RevocationRequest(@JsonProperty("operation_id") @NotNull UUID operationId) {
    @JsonAnySetter public void unknown(String name, Object value) { throw new IllegalArgumentException("unknown field"); }
  }
  public record RevocationResponse(@JsonProperty("operation_id") UUID operationId,
      String status, @JsonProperty("authority_revision") long authorityRevision) {}

  @PostMapping("/exchange")
  public ResponseEntity<ExchangeResponse> exchange(@Valid @RequestBody ExchangeRequest request) {
    String handoff = authorizations.exchangeGameBinding(request.challengeId(), request.operationId(), request.code(),
        request.codeVerifier(), request.deviceProof());
    return ResponseEntity.ok().contentType(MediaType.APPLICATION_JSON).header("Cache-Control", "no-store")
        .body(new ExchangeResponse(handoff));
  }

  @PostMapping("/claim")
  public ResponseEntity<ClaimResponse> claim(@Valid @RequestBody ClaimRequest request) {
    var receipt = service.claim(request.handoffJws(), request.deviceProof(), request.operationId(), request.requestSha256());
    return ResponseEntity.ok().contentType(MediaType.APPLICATION_JSON).header("Cache-Control", "no-store")
        .body(new ClaimResponse(receipt.claimId(), receipt.operationId(), receipt.assertionJti(), receipt.expiresAt()));
  }

  @PostMapping("/completion")
  public ResponseEntity<CompletionResponse> complete(@Valid @RequestBody CompletionRequest request) {
    var receipt = service.complete(request.claimId(), request.operationId(), request.outcome(), request.bindingId());
    return ResponseEntity.ok().contentType(MediaType.APPLICATION_JSON).header("Cache-Control", "no-store")
        .body(new CompletionResponse(receipt.claimId(), receipt.operationId(), receipt.outcome(), receipt.bindingId(), receipt.status()));
  }

  @PostMapping("/revoke")
  public ResponseEntity<RevocationResponse> revoke(@Valid @RequestBody RevocationRequest request) {
    var receipt = service.revoke(request.operationId());
    return ResponseEntity.ok().contentType(MediaType.APPLICATION_JSON).header("Cache-Control", "no-store")
        .body(new RevocationResponse(receipt.operationId(), receipt.status(), receipt.authorityRevision()));
  }

  @ExceptionHandler(SdkAuthorizationConflictException.class)
  public ResponseEntity<Map<String, String>> conflict() {
    return ResponseEntity.status(409).body(Map.of("error", "game_binding_handoff_conflict"));
  }
  @ExceptionHandler(SdkIdentityDeniedException.class)
  public ResponseEntity<Map<String, String>> denied() {
    return ResponseEntity.status(401).body(Map.of("error", "invalid_game_binding_handoff"));
  }
}
