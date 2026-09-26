package voice.backend.auth.rest;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.springframework.boot.test.system.CapturedOutput;
import org.springframework.boot.test.system.OutputCaptureExtension;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.AuthSession;
import voice.backend.auth.service.LinkedAccountsService;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.SendOtpCommand;

@ExtendWith(OutputCaptureExtension.class)
class RegistrationIntentRestControllerTest {
  private final AuthService auth = mock(AuthService.class);
  private final OtpService otp = mock(OtpService.class);
  private MockMvc mvc;

  @BeforeEach
  void setup() {
    mvc = MockMvcBuilders.standaloneSetup(
        new AuthRestController(auth, mock(LinkedAccountsService.class), otp)).build();
    when(auth.register(any())).thenReturn(new AuthSession("access", "refresh", 900,
        UUID.randomUUID().toString(), UUID.randomUUID().toString(), "guest"));
  }

  private String body(String optionalIntent) {
    return "{\"email\":\"new@example.test\",\"password\":\"Strong password 123\","
        + "\"guest\":false,\"device_info_json\":\"{}\"" + optionalIntent + "}";
  }

  @Test
  void optionalRegistrationIntentIsForwardedWithoutChangingInitialEmailOtp() throws Exception {
    UUID intent = UUID.randomUUID();
    mvc.perform(post("/api/v1/auth/register").contentType(MediaType.APPLICATION_JSON)
        .content(body(",\"registrationIntentId\":\"" + intent + "\"")))
        .andExpect(status().isOk()).andExpect(jsonPath("$.session.account_type").value("guest"));
    var command = ArgumentCaptor.forClass(RegisterCommand.class);
    verify(auth).register(command.capture());
    assertThat(command.getValue().registrationIntentId()).isEqualTo(intent);
    assertThat(command.getValue().email()).isEqualTo("new@example.test");
    assertThat(command.getValue().deviceInfoJson()).isEqualTo("{}");
    var verification = ArgumentCaptor.forClass(SendOtpCommand.class);
    verify(otp).sendOtp(verification.capture(), same(auth));
    assertThat(verification.getValue().otpType()).isEqualTo("email_verify");
  }

  @Test
  void omittedIntentAndLegacyFiveArgumentDtoRemainCompatible() throws Exception {
    var legacy = new AuthRestController.RegisterRequest("new@example.test", null, "Strong password 123", false, "{}");
    assertThat(legacy.registrationIntentId()).isNull();
    mvc.perform(post("/api/v1/auth/register").contentType(MediaType.APPLICATION_JSON).content(body("")))
        .andExpect(status().isOk());
    var command = ArgumentCaptor.forClass(RegisterCommand.class);
    verify(auth).register(command.capture());
    assertThat(command.getValue().registrationIntentId()).isNull();
  }

  @Test
  void unavailableIntentBindingReturns503AndDoesNotSendOtp() throws Exception {
    when(auth.register(any())).thenThrow(new AuthException("auth_unavailable"));
    mvc.perform(post("/api/v1/auth/register").contentType(MediaType.APPLICATION_JSON)
        .content(body(",\"registrationIntentId\":\"" + UUID.randomUUID() + "\"")))
        .andExpect(status().isServiceUnavailable())
        .andExpect(content().json("{\"error\":\"auth_unavailable\"}", true));
    verifyNoInteractions(otp);
  }

  @Test
  void invalidIntentReturns400WithoutIssuingSessionOrSendingOtp() throws Exception {
    when(auth.register(any())).thenThrow(new AuthException("validation_failed"));
    mvc.perform(post("/api/v1/auth/register").contentType(MediaType.APPLICATION_JSON)
        .content(body(",\"registrationIntentId\":\"" + UUID.randomUUID() + "\"")))
        .andExpect(status().isBadRequest())
        .andExpect(content().json("{\"error\":\"validation_failed\"}", true));
    verifyNoInteractions(otp);
  }

  @Test
  void malformedRegistrationIntentUuidNeverReachesAuthService() throws Exception {
    mvc.perform(post("/api/v1/auth/register").contentType(MediaType.APPLICATION_JSON)
        .content(body(",\"registrationIntentId\":\"not-a-uuid\"")))
        .andExpect(status().isBadRequest());
    verifyNoInteractions(auth, otp);
  }

  @Test
  void malformedRegistrationIntentDoesNotLogOrEchoSensitiveInput(CapturedOutput output) throws Exception {
    String intentMarker = "PRIVATE_REGISTRATION_INTENT_CREDENTIAL";
    String passwordMarker = "PRIVATE_REGISTRATION_PASSWORD_CREDENTIAL";
    var response = mvc.perform(post("/api/v1/auth/register").contentType(MediaType.APPLICATION_JSON)
        .content(body(",\"registrationIntentId\":\"" + intentMarker + "\"")
            .replace("Strong password 123", passwordMarker)))
        .andExpect(status().isBadRequest()).andReturn().getResponse();
    assertThat(response.getContentAsString()).doesNotContain(intentMarker, passwordMarker);
    assertThat(output.getAll()).doesNotContain(intentMarker, passwordMarker);
    verifyNoInteractions(auth, otp);
  }
}
