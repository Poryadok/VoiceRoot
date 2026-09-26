package voice.backend.auth.sdkidentity;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

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
}
