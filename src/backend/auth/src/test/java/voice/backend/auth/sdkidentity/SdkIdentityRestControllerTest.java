package voice.backend.auth.sdkidentity;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;
import static org.mockito.ArgumentMatchers.eq;

import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;

@org.junit.jupiter.api.extension.ExtendWith(org.springframework.boot.test.system.OutputCaptureExtension.class)
class SdkIdentityRestControllerTest {
  private SdkIdentityService service;
  private MockMvc mvc;

  @BeforeEach void setup() {
    service = mock(SdkIdentityService.class);
    mvc = MockMvcBuilders.standaloneSetup(new SdkIdentityRestController(service)).build();
  }

  @Test void missingIndependentProofIsRejectedBeforeService() throws Exception {
    mvc.perform(post("/api/v1/auth/sdk/exchange").contentType(MediaType.APPLICATION_JSON)
        .content("{\"challengeId\":\"" + UUID.randomUUID()
            + "\",\"gameTicket\":\"developer\",\"deviceProof\":\"proof\"}"))
        .andExpect(status().isBadRequest());
    verifyNoInteractions(service);
  }

  @Test void missingBearerNeverReachesSessionOrRevocation() throws Exception {
    for (String route : new String[] {"session", "revoke"}) {
      mvc.perform(post("/api/v1/auth/sdk/" + route).contentType(MediaType.APPLICATION_JSON)
          .content("{\"deviceProof\":\"proof\"}"))
          .andExpect(status().isUnauthorized())
          .andExpect(jsonPath("$.error").value("invalid_sdk_identity"));
    }
    verifyNoInteractions(service);
  }

  @Test void invalidProofReturnsCoarseDenialWithoutTokenOrProviderData() throws Exception {
    when(service.exchange(any(), any(), any(), any())).thenThrow(new SdkIdentityDeniedException());
    mvc.perform(post("/api/v1/auth/sdk/exchange").contentType(MediaType.APPLICATION_JSON)
        .content("{\"challengeId\":\"" + UUID.randomUUID()
            + "\",\"providerToken\":\"secret-provider-token\","
            + "\"gameTicket\":\"secret-game-ticket\",\"deviceProof\":\"proof\"}"))
        .andExpect(status().isUnauthorized())
        .andExpect(content().json("{\"error\":\"invalid_sdk_identity\"}"));
  }

  @Test void bearerAndPossessionArePassedSeparatelyForRevoke() throws Exception {
    mvc.perform(post("/api/v1/auth/sdk/revoke").header("Authorization", "Bearer opaque-token")
        .contentType(MediaType.APPLICATION_JSON).content("{\"deviceProof\":\"signed-proof\"}"))
        .andExpect(status().isNoContent());
    verify(service).revoke("opaque-token", "signed-proof");
  }

  @Test void invalidCredentialSizeNeverLogsOrEchoesCredential(
      org.springframework.boot.test.system.CapturedOutput output) throws Exception {
    String secret = "SECRET_DEVICE_PROOF_MUST_NOT_BE_LOGGED".repeat(130);
    mvc.perform(post("/api/v1/auth/sdk/session").header("Authorization", "Bearer opaque-token")
        .contentType(MediaType.APPLICATION_JSON).content("{\"deviceProof\":\"" + secret + "\"}"))
        .andExpect(status().isBadRequest());
    org.assertj.core.api.Assertions.assertThat(output.getAll()).doesNotContain(secret);
    verifyNoInteractions(service);
  }

  @Test void callerCannotAddUndocumentedAuthorityFields() throws Exception {
    mvc.perform(post("/api/v1/auth/sdk/exchange").contentType(MediaType.APPLICATION_JSON)
        .content("{\"challengeId\":\"" + UUID.randomUUID()
            + "\",\"providerToken\":\"provider\",\"gameTicket\":\"game\","
            + "\"deviceProof\":\"proof\",\"accountId\":\"caller-selected-account\"}"))
        .andExpect(status().isBadRequest());
    verifyNoInteractions(service);
  }

  @Test void rotateRejectsDuplicateJsonNamesBeforeCallingAuth() throws Exception {
    mvc.perform(post("/api/v1/auth/sdk/device-keys/rotate").contentType(MediaType.APPLICATION_JSON)
        .content("{\"challengeId\":\"" + UUID.randomUUID() + "\",\"challengeId\":\""
            + UUID.randomUUID() + "\",\"providerToken\":\"secret\","
            + "\"currentKeyProof\":\"current\",\"newKeyProof\":\"new\","
            + "\"requestId\":\"" + UUID.randomUUID() + "\"}"))
        .andExpect(status().isUnauthorized());
    verifyNoInteractions(service);
  }

  @Test void rotatePreservesOriginalRequestBytesForIdempotencyReceipt() throws Exception {
    UUID challengeId = UUID.randomUUID();
    UUID requestId = UUID.randomUUID();
    byte[] body = ("{ \"challengeId\": \"" + challengeId + "\", \"providerToken\": \"p\","
        + " \"currentKeyProof\": \"c\", \"newKeyProof\": \"n\", \"requestId\": \""
        + requestId + "\" }").getBytes(java.nio.charset.StandardCharsets.UTF_8);
    when(service.rotate(eq(challengeId), eq("p"), eq("c"), eq("n"), eq(requestId), any()))
        .thenReturn(new SdkIdentityService.DeviceKeyResult(UUID.randomUUID(), UUID.randomUUID(), 2, 2));
    mvc.perform(post("/api/v1/auth/sdk/device-keys/rotate").contentType(MediaType.APPLICATION_JSON).content(body))
        .andExpect(status().isOk());
    verify(service).rotate(eq(challengeId), eq("p"), eq("c"), eq("n"), eq(requestId), eq(body));
  }

  @Test void deviceAuthorityRequiresBearerAndPassesExactSignedRequestBytes() throws Exception {
    byte[] proof = "compact.signed.jws".getBytes(java.nio.charset.StandardCharsets.US_ASCII);
    when(service.deviceAuthority("opaque-token", new String(proof, java.nio.charset.StandardCharsets.US_ASCII), proof))
        .thenReturn("auth.assertion.jwt");
    mvc.perform(post("/api/v1/auth/sdk/device-authority").header("Authorization", "Bearer opaque-token")
        .contentType("text/plain").accept("application/jwt").content(proof))
        .andExpect(status().isOk()).andExpect(content().contentType("application/jwt"))
        .andExpect(content().string("auth.assertion.jwt"));
    verify(service).deviceAuthority("opaque-token", "compact.signed.jws", proof);

    mvc.perform(post("/api/v1/auth/sdk/device-authority").contentType("text/plain").content(proof))
        .andExpect(status().isUnauthorized());
  }
}
