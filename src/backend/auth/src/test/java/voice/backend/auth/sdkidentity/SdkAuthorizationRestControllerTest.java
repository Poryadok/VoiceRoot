package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URI;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
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
class SdkAuthorizationRestControllerTest {
  private static final String ROOT = "/api/v1/auth/sdk/authorizations";
  private static final UUID REQUEST = UUID.fromString("11111111-1111-4111-8111-111111111111");
  private static final UUID KEY = UUID.fromString("22222222-2222-4222-8222-222222222222");
  private static final UUID PROFILE = UUID.fromString("33333333-3333-4333-8333-333333333333");
  private static final String REDIRECT = "https://game.example/callback";
  private static final String CHALLENGE = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM";
  private static final String VERIFIER = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk";
  private static final String STATE = "s".repeat(43);
  private static final String CODE = "c".repeat(43);
  private static final Set<String> SCOPES = Set.of("game.voice.join", "game.identity.read");
  private static final Instant EXPIRES = Instant.parse("2026-09-26T12:05:00Z");
  private final ObjectMapper json = new ObjectMapper();
  private SdkAuthorizationService service;
  private MockMvc mvc;

  @BeforeEach
  void setup() {
    service = mock(SdkAuthorizationService.class);
    mvc = MockMvcBuilders.standaloneSetup(new SdkAuthorizationRestController(service)).build();
  }

  private Map<String, Object> start() {
    return new LinkedHashMap<>(Map.of("idempotencyKey", KEY.toString(), "redirectUri", REDIRECT,
        "codeChallenge", CHALLENGE, "state", STATE, "scopes", SCOPES, "deviceProof", "signed-device-proof"));
  }

  private Map<String, Object> approval() {
    return new LinkedHashMap<>(Map.of("profileId", PROFILE.toString(), "policyRevision", 7));
  }

  private Map<String, Object> exchange() {
    return new LinkedHashMap<>(Map.of("code", CODE, "redirectUri", REDIRECT,
        "codeVerifier", VERIFIER, "deviceProof", "signed-code-proof"));
  }

  private MockHttpServletRequestBuilder postJson(String path, Map<String, Object> body) throws Exception {
    return post(path).contentType(MediaType.APPLICATION_JSON).content(json.writeValueAsString(body));
  }

  private SdkAuthorizationService.LinkedSession linked(String token) {
    return new SdkAuthorizationService.LinkedSession(UUID.randomUUID(), UUID.randomUUID(), PROFILE,
        UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), SCOPES, 2, 7, token, EXPIRES);
  }

  @Test
  void startForwardsSdkBearerAndExactSignedRequestFields() throws Exception {
    when(service.start(any(), any(), any(), any(), any(), any(), any())).thenReturn(
        new SdkAuthorizationService.AuthorizationRequest(REQUEST, 7, EXPIRES, "Example Game", SCOPES));
    mvc.perform(postJson(ROOT, start()).header("Authorization", "Bearer sdk-token"))
        .andExpect(status().isOk()).andExpect(jsonPath("$.requestId").value(REQUEST.toString()))
        .andExpect(jsonPath("$.policyRevision").value(7))
        .andExpect(jsonPath("$.displayName").value("Example Game"));
    verify(service).start("sdk-token", "signed-device-proof", KEY, REDIRECT, CHALLENGE, STATE, SCOPES);
  }

  @Test
  void approvalForwardsVoiceAuthorizationAndExplicitSelectedProfile() throws Exception {
    when(service.approve(REQUEST, "Bearer voice-token", PROFILE, 7)).thenReturn(
        new SdkAuthorizationService.Approval(CODE, URI.create(REDIRECT + "?code=" + CODE + "&state=" + STATE), EXPIRES));
    mvc.perform(postJson(ROOT + "/" + REQUEST + "/approve", approval())
        .header("Authorization", "Bearer voice-token"))
        .andExpect(status().isOk()).andExpect(jsonPath("$.code").value(CODE))
        .andExpect(jsonPath("$.redirectUri").value(REDIRECT + "?code=" + CODE + "&state=" + STATE));
    verify(service).approve(REQUEST, "Bearer voice-token", PROFILE, 7);
  }

  @Test
  void codeExchangeForwardsCodePkceRedirectAndDeviceProofWithoutBearer() throws Exception {
    when(service.exchange(REQUEST, CODE, REDIRECT, VERIFIER, "signed-code-proof")).thenReturn(linked("linked-token"));
    mvc.perform(postJson(ROOT + "/" + REQUEST + "/exchange", exchange()))
        .andExpect(status().isOk()).andExpect(jsonPath("$.profileId").value(PROFILE.toString()))
        .andExpect(jsonPath("$.accessToken").value("linked-token"));
    verify(service).exchange(REQUEST, CODE, REDIRECT, VERIFIER, "signed-code-proof");
  }

  @Test
  void linkedSessionForwardsBearerAndPossessionSeparately() throws Exception {
    when(service.linkedSession("linked-token", "linked-device-proof")).thenReturn(linked(null));
    mvc.perform(postJson(ROOT + "/linked-session", Map.of("deviceProof", "linked-device-proof"))
        .header("Authorization", "Bearer linked-token"))
        .andExpect(status().isOk()).andExpect(jsonPath("$.profileId").value(PROFILE.toString()));
    verify(service).linkedSession("linked-token", "linked-device-proof");
  }

  @Test
  void consentViewForwardsCurrentVoiceAuthorizationAndDoesNotApprove() throws Exception {
    UUID app = UUID.randomUUID();
    UUID env = UUID.randomUUID();
    when(service.inspect(REQUEST, "Bearer voice-token")).thenReturn(new SdkAuthorizationService.ConsentView(
        REQUEST, app, env, "Example Game", SCOPES, "game-player", 7, EXPIRES));
    mvc.perform(get(ROOT + "/" + REQUEST).header("Authorization", "Bearer voice-token"))
        .andExpect(status().isOk()).andExpect(jsonPath("$.requestId").value(REQUEST.toString()))
        .andExpect(jsonPath("$.applicationId").value(app.toString()))
        .andExpect(jsonPath("$.environmentId").value(env.toString()))
        .andExpect(jsonPath("$.gameSubject").value("game-player"))
        .andExpect(jsonPath("$.policyRevision").value(7))
        .andExpect(jsonPath("$.code").doesNotExist()).andExpect(jsonPath("$.accessToken").doesNotExist());
    verify(service).inspect(REQUEST, "Bearer voice-token");
    verifyNoMoreInteractions(service);
  }

  @Test
  void missingCredentialsNeverReachAnyBearerProtectedServiceMethod() throws Exception {
    for (var request : new MockHttpServletRequestBuilder[] {
        postJson(ROOT, start()), postJson(ROOT + "/" + REQUEST + "/approve", approval()),
        postJson(ROOT + "/linked-session", Map.of("deviceProof", "proof")), get(ROOT + "/" + REQUEST)}) {
      mvc.perform(request).andExpect(status().isUnauthorized())
          .andExpect(content().json("{\"error\":\"invalid_sdk_identity\"}", true));
    }
    verifyNoInteractions(service);
  }

  @ParameterizedTest
  @ValueSource(strings = {"Basic secret", "Bearer ", "bearer sdk-token"})
  void malformedBearerDoesNotReachStart(String authorization) throws Exception {
    mvc.perform(postJson(ROOT, start()).header("Authorization", authorization))
        .andExpect(status().isUnauthorized())
        .andExpect(content().json("{\"error\":\"invalid_sdk_identity\"}", true));
    verifyNoInteractions(service);
  }

  @Test
  void identityDenialsAcrossRoutesUseSameCoarseResponse() throws Exception {
    when(service.start(any(), any(), any(), any(), any(), any(), any())).thenThrow(new SdkIdentityDeniedException());
    when(service.approve(any(), any(), any(), anyLong())).thenThrow(new SdkIdentityDeniedException());
    when(service.exchange(any(), any(), any(), any(), any())).thenThrow(new SdkIdentityDeniedException());
    when(service.linkedSession(any(), any())).thenThrow(new SdkIdentityDeniedException());
    when(service.inspect(any(), any())).thenThrow(new SdkIdentityDeniedException());
    for (var request : new MockHttpServletRequestBuilder[] {
        postJson(ROOT, start()), postJson(ROOT + "/" + REQUEST + "/approve", approval()),
        postJson(ROOT + "/" + REQUEST + "/exchange", exchange()),
        postJson(ROOT + "/linked-session", Map.of("deviceProof", "proof")), get(ROOT + "/" + REQUEST)}) {
      mvc.perform(request.header("Authorization", "Bearer credential"))
          .andExpect(status().isUnauthorized())
          .andExpect(content().json("{\"error\":\"invalid_sdk_identity\"}", true));
    }
  }

  @Test
  void changedIdempotencyBodyReturnsConflict() throws Exception {
    when(service.start(any(), any(), any(), any(), any(), any(), any()))
        .thenThrow(new SdkAuthorizationConflictException());
    mvc.perform(postJson(ROOT, start()).header("Authorization", "Bearer sdk-token"))
        .andExpect(status().isConflict());
  }

  @Test
  void callerCannotSupplyAuthorityFieldsOnAnyRequest() throws Exception {
    var requests = Map.of(ROOT, start(), ROOT + "/" + REQUEST + "/approve", approval(),
        ROOT + "/" + REQUEST + "/exchange", exchange(),
        ROOT + "/linked-session", new LinkedHashMap<String, Object>(Map.of("deviceProof", "proof")));
    for (var entry : requests.entrySet()) {
      entry.getValue().put("accountId", "caller-selected-account");
      mvc.perform(postJson(entry.getKey(), entry.getValue()).header("Authorization", "Bearer credential"))
          .andExpect(status().isBadRequest())
          .andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    }
    verifyNoInteractions(service);
  }

  @Test
  void approvalRequiresExplicitProfileAndPositivePolicyRevision() throws Exception {
    for (var invalid : List.<Map<String, Object>>of(Map.of("policyRevision", 7),
        Map.of("profileId", PROFILE.toString(), "policyRevision", 0),
        Map.of("profileId", PROFILE.toString(), "policyRevision", -1))) {
      mvc.perform(postJson(ROOT + "/" + REQUEST + "/approve", invalid)
          .header("Authorization", "Bearer voice-token"))
          .andExpect(status().isBadRequest())
          .andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    }
    verifyNoInteractions(service);
  }

  @Test
  void missingProofNeverReachesStartExchangeOrLinkedSession() throws Exception {
    var start = start(); start.remove("deviceProof");
    var exchange = exchange(); exchange.remove("deviceProof");
    for (var request : new MockHttpServletRequestBuilder[] {postJson(ROOT, start),
        postJson(ROOT + "/" + REQUEST + "/exchange", exchange), postJson(ROOT + "/linked-session", Map.of())}) {
      mvc.perform(request.header("Authorization", "Bearer credential")).andExpect(status().isBadRequest());
    }
    verifyNoInteractions(service);
  }

  @Test
  void oversizedSensitiveFieldsAreRejectedWithoutLoggingOrEcho(CapturedOutput output) throws Exception {
    String secretProof = "SECRET_DEVICE_PROOF_DO_NOT_LOG_".repeat(150);
    for (String route : new String[] {ROOT, ROOT + "/" + REQUEST + "/exchange", ROOT + "/linked-session"}) {
      Map<String, Object> body = route.equals(ROOT) ? start()
          : route.endsWith("/exchange") ? exchange() : new LinkedHashMap<>();
      body.put("deviceProof", secretProof);
      mvc.perform(postJson(route, body).header("Authorization", "Bearer credential"))
          .andExpect(status().isBadRequest())
          .andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    }
    for (String field : new String[] {"code", "codeVerifier"}) {
      var body = exchange();
      body.put(field, "SENSITIVE_".repeat(20));
      mvc.perform(postJson(ROOT + "/" + REQUEST + "/exchange", body)).andExpect(status().isBadRequest())
          .andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    }
    assertThat(output.getAll()).doesNotContain("SECRET_DEVICE_PROOF_DO_NOT_LOG_", "SENSITIVE_");
    verifyNoInteractions(service);
  }

  @Test
  void malformedJsonAndUuidProduceCoarseErrorsWithoutLoggingInput(CapturedOutput output) throws Exception {
    String marker = "PRIVATE_RAW_INPUT_MUST_NOT_APPEAR";
    mvc.perform(post(ROOT).header("Authorization", "Bearer credential").contentType(MediaType.APPLICATION_JSON)
        .content("{\"deviceProof\":\"" + marker + "\",\"idempotencyKey\":\"invalid-uuid\"}"))
        .andExpect(status().isBadRequest()).andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    mvc.perform(post(ROOT).header("Authorization", "Bearer credential").contentType(MediaType.APPLICATION_JSON)
        .content("{\"deviceProof\":\"" + marker))
        .andExpect(status().isBadRequest()).andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    mvc.perform(postJson(ROOT + "/bad-uuid/approve", approval()).header("Authorization", "Bearer voice-token"))
        .andExpect(status().isBadRequest()).andExpect(content().json("{\"error\":\"invalid_sdk_request\"}", true));
    assertThat(output.getAll()).doesNotContain(marker);
    verifyNoInteractions(service);
  }
}
