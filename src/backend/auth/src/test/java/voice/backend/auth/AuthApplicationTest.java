package voice.backend.auth;

import static org.hamcrest.Matchers.is;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class AuthApplicationTest {
  @Autowired
  private MockMvc mockMvc;

  @Autowired private voice.backend.auth.releasepolicy.ReleasePolicy releasePolicy;
  @Autowired private voice.backend.auth.service.AuthService authService;
  @Autowired private voice.backend.auth.service.OtpService otpService;

  @Test
  void ownerGuardsUseExactlyTheBootstrapPolicy() {
    org.assertj.core.api.Assertions.assertThat(authService.releasePolicy()).isSameAs(releasePolicy);
    org.assertj.core.api.Assertions.assertThat(otpService.releasePolicy()).isSameAs(releasePolicy);
    org.assertj.core.api.Assertions.assertThat(releasePolicy.profile()).isEqualTo("development");
  }

  @Test
  void healthReturnsOk() throws Exception {
    mockMvc.perform(get("/health"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.service", is("auth")))
        .andExpect(jsonPath("$.status", is("ok")));
  }
}
