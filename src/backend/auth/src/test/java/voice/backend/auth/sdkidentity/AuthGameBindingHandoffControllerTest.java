package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.UUID;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

class AuthGameBindingHandoffControllerTest {
  private final ObjectMapper json = new ObjectMapper();

  @Test
  void gisClaimAndCompletionJsonUseStrictSnakeCaseWireFields() throws Exception {
    UUID operation = UUID.randomUUID();
    var claim = json.readValue("""
        {"operation_id":"%s","request_sha256":"%s","handoff_jws":"signed.handoff","device_proof":"proof"}
        """.formatted(operation, "a".repeat(64)), AuthGameBindingHandoffController.ClaimRequest.class);
    assertThat(claim.operationId()).isEqualTo(operation);
    assertThat(claim.requestSha256()).isEqualTo("a".repeat(64));
    assertThatThrownBy(() -> json.readValue("""
        {"operation_id":"%s","request_sha256":"%s","handoff_jws":"signed.handoff","device_proof":"proof","extra":true}
        """.formatted(operation, "a".repeat(64)), AuthGameBindingHandoffController.ClaimRequest.class))
        .hasRootCauseInstanceOf(IllegalArgumentException.class);
    var completion = json.readValue("""
        {"claim_id":"%s","operation_id":"%s","outcome":"succeeded","binding_id":"%s"}
        """.formatted(UUID.randomUUID(), operation, UUID.randomUUID()), AuthGameBindingHandoffController.CompletionRequest.class);
    assertThat(completion.operationId()).isEqualTo(operation);
    assertThat(completion.outcome()).isEqualTo("succeeded");
  }

  @Test
  void claimPassesOnlySignedHandoffDeviceProofAndStableOperationToService() {
    AuthGameBindingHandoffService service = mock(AuthGameBindingHandoffService.class);
    AuthGameBindingHandoffController controller = new AuthGameBindingHandoffController(service, mock(SdkAuthorizationService.class));
    UUID operation = UUID.randomUUID();
    UUID claim = UUID.randomUUID();
    var jti = UUID.randomUUID();
    var expires = java.time.Instant.parse("2026-09-28T00:00:30Z");
    when(service.claim("signed.handoff", "device.proof", operation, "a".repeat(64)))
        .thenReturn(new AuthGameBindingHandoffService.ClaimReceipt(claim, operation, jti, expires));

    var response = controller.claim(new AuthGameBindingHandoffController.ClaimRequest(operation,
        "a".repeat(64), "signed.handoff", "device.proof"));

    verify(service).claim("signed.handoff", "device.proof", operation, "a".repeat(64));
    assertThat(response.getHeaders().getFirst("Cache-Control")).isEqualTo("no-store");
    assertThat(response.getBody().claimId()).isEqualTo(claim);
    assertThat(response.getBody().operationId()).isEqualTo(operation);
    assertThat(response.getBody().assertionJti()).isEqualTo(jti);
    assertThat(response.getBody().expiresAt()).isEqualTo(expires);
  }

  @Test
  void exchangeConsumesTheT14CodeOnlyThroughThePrivateStrictWire() throws Exception {
    SdkAuthorizationService authorizations = mock(SdkAuthorizationService.class);
    AuthGameBindingHandoffController controller = new AuthGameBindingHandoffController(
        mock(AuthGameBindingHandoffService.class), authorizations);
    UUID challenge = UUID.randomUUID();
    UUID operation = UUID.randomUUID();
    var request = json.readValue("""
        {"challenge_id":"%s","operation_id":"%s","code":"%s","code_verifier":"%s","device_proof":"proof"}
        """.formatted(challenge, operation, "c".repeat(43), "v".repeat(43)),
        AuthGameBindingHandoffController.ExchangeRequest.class);
    when(authorizations.exchangeGameBinding(challenge, operation, "c".repeat(43), "v".repeat(43), "proof"))
        .thenReturn("signed.handoff");

    var response = controller.exchange(request);

    org.mockito.Mockito.verify(authorizations).exchangeGameBinding(challenge, operation,
        "c".repeat(43), "v".repeat(43), "proof");
    assertThat(response.getHeaders().getFirst("Cache-Control")).isEqualTo("no-store");
    assertThat(response.getBody().handoffJws()).isEqualTo("signed.handoff");
    assertThatThrownBy(() -> json.readValue("""
        {"challenge_id":"%s","operation_id":"%s","code":"%s","code_verifier":"%s","device_proof":"proof","raw_subject":"no"}
        """.formatted(challenge, operation, "c".repeat(43), "v".repeat(43)),
        AuthGameBindingHandoffController.ExchangeRequest.class))
        .hasRootCauseInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void completionRequiresClaimOperationAndOutcomeAndReturnsDurableReceipt() {
    AuthGameBindingHandoffService service = mock(AuthGameBindingHandoffService.class);
    AuthGameBindingHandoffController controller = new AuthGameBindingHandoffController(service, mock(SdkAuthorizationService.class));
    UUID claim = UUID.randomUUID();
    UUID operation = UUID.randomUUID();
    UUID binding = UUID.randomUUID();
    when(service.complete(claim, operation, "succeeded", binding))
        .thenReturn(new AuthGameBindingHandoffService.CompletionReceipt(claim, operation, "succeeded", binding, "completed"));

    var response = controller.complete(new AuthGameBindingHandoffController.CompletionRequest(claim, operation, "succeeded", binding));

    verify(service).complete(claim, operation, "succeeded", binding);
    assertThat(response.getHeaders().getFirst("Cache-Control")).isEqualTo("no-store");
    assertThat(response.getBody().bindingId()).isEqualTo(binding);
    assertThat(response.getBody().outcome()).isEqualTo("succeeded");
    assertThat(response.getBody().status()).isEqualTo("completed");
  }

  @Test
  void revokeRequiresTheStableOperationAndReturnsPendingDrainOrRevokedReceipt() throws Exception {
    AuthGameBindingHandoffService service = mock(AuthGameBindingHandoffService.class);
    AuthGameBindingHandoffController controller = new AuthGameBindingHandoffController(service, mock(SdkAuthorizationService.class));
    UUID operation = UUID.randomUUID();
    var request = json.readValue("{\"operation_id\":\"%s\"}".formatted(operation),
        AuthGameBindingHandoffController.RevocationRequest.class);
    when(service.revoke(operation)).thenReturn(new AuthGameBindingHandoffService.RevocationReceipt(operation, "revoking", 4));

    var response = controller.revoke(request);

    verify(service).revoke(operation);
    assertThat(response.getHeaders().getFirst("Cache-Control")).isEqualTo("no-store");
    assertThat(response.getBody().status()).isEqualTo("revoking");
    assertThat(response.getBody().authorityRevision()).isEqualTo(4);
    assertThatThrownBy(() -> json.readValue("{\"operation_id\":\"%s\",\"binding_id\":\"%s\"}"
        .formatted(operation, UUID.randomUUID()), AuthGameBindingHandoffController.RevocationRequest.class))
        .hasRootCauseInstanceOf(IllegalArgumentException.class);
  }
}
