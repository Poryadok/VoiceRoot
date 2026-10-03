package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
import org.springframework.boot.test.system.CapturedOutput;
import org.springframework.boot.test.system.OutputCaptureExtension;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.request.MockHttpServletRequestBuilder;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;

@ExtendWith(OutputCaptureExtension.class)
class SdkConversionRestControllerTest {
  private static final String ROOT = "/api/v1/auth/sdk/conversions";
  private static final UUID OPERATION = UUID.fromString("11111111-1111-4111-8111-111111111111");
  private static final UUID KEY = UUID.fromString("22222222-2222-4222-8222-222222222222");
  private static final UUID BINDING = UUID.fromString("33333333-3333-4333-8333-333333333333");
  private static final long ISSUED_AT = 1790424000L;
  private final ObjectMapper json = new ObjectMapper();
  private SdkConversionService service;
  private MockMvc mvc;

  @BeforeEach
  void setup() {
    service = mock(SdkConversionService.class);
    mvc = MockMvcBuilders.standaloneSetup(new SdkConversionRestController(service)).build();
  }

  private Map<String, Object> prepare() {
    return new LinkedHashMap<>(Map.of("idempotencyKey", KEY.toString(), "bindingId", BINDING.toString(),
        "deviceProof", "signed-prepare-proof"));
  }

  private Map<String, Object> statusBody() {
    return new LinkedHashMap<>(Map.of("issuedAt", ISSUED_AT, "deviceProof", "signed-status-proof"));
  }

  private MockHttpServletRequestBuilder postJson(String path, Map<String, Object> body) throws Exception {
    return post(path).contentType(MediaType.APPLICATION_JSON).content(json.writeValueAsString(body));
  }

  private SdkConversionService.Operation operation(String mode) {
    return new SdkConversionService.Operation(OPERATION, mode, "prepared", 1,
        "new".equals(mode) ? UUID.randomUUID() : null,
        "new".equals(mode) ? Instant.parse("2026-09-26T12:15:00Z") : null, null, null, null);
  }

  @ParameterizedTest
  @ValueSource(strings = {"new", "existing"})
  void prepareForwardsCorrectCredentialProofAndIdempotencyBinding(String mode) throws Exception {
    String token = mode.equals("new") ? "sdk-source-token" : "linked-source-token";
    if (mode.equals("new")) when(service.prepareNew(token, "signed-prepare-proof", KEY, BINDING)).thenReturn(operation(mode));
    else when(service.prepareExisting(token, "signed-prepare-proof", KEY, BINDING)).thenReturn(operation(mode));
    mvc.perform(postJson(ROOT + "/" + mode, prepare()).header("Authorization", "Bearer " + token))
        .andExpect(status().isOk()).andExpect(jsonPath("$.operationId").value(OPERATION.toString()))
        .andExpect(jsonPath("$.mode").value(mode)).andExpect(jsonPath("$.state").value("prepared"));
    if (mode.equals("new")) verify(service).prepareNew(token, "signed-prepare-proof", KEY, BINDING);
    else verify(service).prepareExisting(token, "signed-prepare-proof", KEY, BINDING);
    verifyNoMoreInteractions(service);
  }

  @Test
  void recoveryStatusUsesOperationTimestampAndDeviceProofWithoutBearer() throws Exception {
    when(service.status(OPERATION, ISSUED_AT, "signed-status-proof")).thenReturn(operation("new"));
    mvc.perform(postJson(ROOT + "/" + OPERATION + "/status", statusBody()))
        .andExpect(status().isOk()).andExpect(jsonPath("$.operationId").value(OPERATION.toString()));
    verify(service).status(OPERATION, ISSUED_AT, "signed-status-proof");
    verifyNoMoreInteractions(service);
  }

  @Test
  void attachNewTargetForwardsCurrentVoiceBearerWithoutCallerSelectedAccount() throws Exception {
    when(service.attachNewTarget(OPERATION, "Bearer voice-token")).thenReturn(operation("new"));
    mvc.perform(post(ROOT + "/" + OPERATION + "/attach-new-target")
        .header("Authorization", "Bearer voice-token"))
        .andExpect(status().isOk()).andExpect(jsonPath("$.operationId").value(OPERATION.toString()));
    verify(service).attachNewTarget(OPERATION, "Bearer voice-token");
    verifyNoMoreInteractions(service);
  }

  @Test
  void attachNewTargetRejectsCallerSelectedAccountBodyBeforeService() throws Exception {
    mvc.perform(postJson(ROOT + "/" + OPERATION + "/attach-new-target",
        Map.of("targetAccountId", UUID.randomUUID().toString()))
        .header("Authorization", "Bearer voice-token"))
        .andExpect(status().isBadRequest())
        .andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    verifyNoInteractions(service);
  }

  @Test
  void missingCredentialsNeverReachPrepareOrAttach() throws Exception {
    for (var request : new MockHttpServletRequestBuilder[] {postJson(ROOT + "/new", prepare()),
        postJson(ROOT + "/existing", prepare()), post(ROOT + "/" + OPERATION + "/attach-new-target")}) {
      mvc.perform(request).andExpect(status().isUnauthorized())
          .andExpect(content().json("{\"error\":\"invalid_sdk_identity\"}", true));
    }
    verifyNoInteractions(service);
  }

  @ParameterizedTest
  @ValueSource(strings = {"Basic credential", "Bearer ", "bearer credential"})
  void malformedBearerIsRejectedBeforePrepare(String bearer) throws Exception {
    mvc.perform(postJson(ROOT + "/new", prepare()).header("Authorization", bearer))
        .andExpect(status().isUnauthorized()).andExpect(content().json("{\"error\":\"invalid_sdk_identity\"}", true));
    verifyNoInteractions(service);
  }

  @Test
  void identityDenialsAcrossAllRoutesReturnCoarse401() throws Exception {
    when(service.prepareNew(any(), any(), any(), any())).thenThrow(new SdkIdentityDeniedException());
    when(service.prepareExisting(any(), any(), any(), any())).thenThrow(new SdkIdentityDeniedException());
    when(service.status(any(), anyLong(), any())).thenThrow(new SdkIdentityDeniedException());
    when(service.attachNewTarget(any(), any())).thenThrow(new SdkIdentityDeniedException());
    for (var request : new MockHttpServletRequestBuilder[] {postJson(ROOT + "/new", prepare()),
        postJson(ROOT + "/existing", prepare()), postJson(ROOT + "/" + OPERATION + "/status", statusBody()),
        post(ROOT + "/" + OPERATION + "/attach-new-target")}) {
      mvc.perform(request.header("Authorization", "Bearer credential")).andExpect(status().isUnauthorized())
          .andExpect(content().json("{\"error\":\"invalid_sdk_identity\"}", true));
    }
  }

  @Test
  void changedPreparePayloadConflictReturns409() throws Exception {
    when(service.prepareNew(any(), any(), any(), any())).thenThrow(new SdkAuthorizationConflictException());
    mvc.perform(postJson(ROOT + "/new", prepare()).header("Authorization", "Bearer sdk-token"))
        .andExpect(status().isConflict());
  }

  @Test
  void undocumentedAccountAuthorityFieldsCannotReachService() throws Exception {
    for (String path : new String[] {ROOT + "/new", ROOT + "/existing", ROOT + "/" + OPERATION + "/status"}) {
      var body = path.endsWith("/status") ? statusBody() : prepare();
      body.put("targetAccountId", "caller-selected-account");
      mvc.perform(postJson(path, body).header("Authorization", "Bearer credential"))
          .andExpect(status().isBadRequest()).andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    }
    verifyNoInteractions(service);
  }

  @Test
  void missingRequiredProofOrBindingFailsBeforeService() throws Exception {
    for (String missing : new String[] {"idempotencyKey", "bindingId", "deviceProof"}) {
      var body = prepare(); body.remove(missing);
      mvc.perform(postJson(ROOT + "/new", body).header("Authorization", "Bearer sdk-token"))
          .andExpect(status().isBadRequest());
    }
    var status = statusBody(); status.remove("deviceProof");
    mvc.perform(postJson(ROOT + "/" + OPERATION + "/status", status)).andExpect(status().isBadRequest());
    verifyNoInteractions(service);
  }

  @Test
  void oversizedDeviceProofIsRejectedWithoutLoggingOrEcho(CapturedOutput output) throws Exception {
    String marker = "PRIVATE_CONVERSION_DEVICE_PROOF_";
    String secret = marker.repeat(150);
    for (String path : new String[] {ROOT + "/new", ROOT + "/existing", ROOT + "/" + OPERATION + "/status"}) {
      var body = path.endsWith("/status") ? statusBody() : prepare();
      body.put("deviceProof", secret);
      mvc.perform(postJson(path, body).header("Authorization", "Bearer credential"))
          .andExpect(status().isBadRequest()).andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    }
    assertThat(output.getAll()).doesNotContain(marker);
    verifyNoInteractions(service);
  }

  @Test
  void malformedUuidAndTimestampCannotLeakSensitiveParserInput(CapturedOutput output) throws Exception {
    String marker = "PRIVATE_CONVERSION_PARSER_CREDENTIAL";
    var prepare = prepare(); prepare.put("bindingId", marker);
    mvc.perform(postJson(ROOT + "/new", prepare).header("Authorization", "Bearer credential"))
        .andExpect(status().isBadRequest()).andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    var status = statusBody(); status.put("issuedAt", marker);
    mvc.perform(postJson(ROOT + "/" + OPERATION + "/status", status))
        .andExpect(status().isBadRequest()).andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    mvc.perform(postJson(ROOT + "/bad-uuid/status", statusBody()))
        .andExpect(status().isBadRequest()).andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    assertThat(output.getAll()).doesNotContain(marker);
    verifyNoInteractions(service);
  }
}
