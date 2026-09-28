package voice.backend.auth.rest;

import static org.hamcrest.Matchers.is;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.validation.beanvalidation.LocalValidatorFactoryBean;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthSession;
import voice.backend.auth.service.LinkedAccountsService;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.SendOtpCommand;

class AuthRegisterValidationTest {
  @ParameterizedTest
  @ValueSource(strings = {"not-an-email", ""})
  void malformedEmailIsRejectedBeforeRegistrationOrVerificationMail(String email) throws Exception {
    AuthService authService = mock(AuthService.class);
    OtpService otpService = mock(OtpService.class);
    when(authService.register(any(RegisterCommand.class)))
        .thenReturn(new AuthSession("access", "refresh", 900, "account", "profile", "guest", true));
    LocalValidatorFactoryBean validator = new LocalValidatorFactoryBean();
    validator.afterPropertiesSet();
    try {
      MockMvc mockMvc = MockMvcBuilders
          .standaloneSetup(new AuthRestController(authService, mock(LinkedAccountsService.class), otpService))
          .setValidator(validator)
          .build();

      mockMvc.perform(post("/api/v1/auth/register")
              .contentType("application/json")
              .content("{\"email\":\"" + email
                  + "\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
          .andExpect(status().isBadRequest())
          .andExpect(jsonPath("$.error", is("validation_failed")));

      verifyNoInteractions(authService, otpService);
    } finally {
      validator.destroy();
    }
  }

  @Test
  void verificationSenderFailureStillReturnsAuthUnavailable() throws Exception {
    AuthService authService = mock(AuthService.class);
    OtpService otpService = mock(OtpService.class);
    when(authService.register(any(RegisterCommand.class)))
        .thenReturn(new AuthSession("access", "refresh", 900, "account", "profile", "guest", true));
    doThrow(new AuthException("auth_unavailable"))
        .when(otpService).sendOtp(any(SendOtpCommand.class), any(AuthService.class));
    MockMvc mockMvc = MockMvcBuilders
        .standaloneSetup(new AuthRestController(authService, mock(LinkedAccountsService.class), otpService))
        .build();

    mockMvc.perform(post("/api/v1/auth/register")
            .contentType("application/json")
            .content("{\"email\":\"valid@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isServiceUnavailable())
        .andExpect(jsonPath("$.error", is("auth_unavailable")));

    verify(authService).register(any(RegisterCommand.class));
    verify(otpService).sendOtp(any(SendOtpCommand.class), any(AuthService.class));
  }
}
