package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.hamcrest.Matchers.is;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.ApplicationContext;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import voice.backend.auth.support.CapturingMailSender;
import voice.backend.auth.support.RecordingAuthEventPublisher;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class GuestConvertNatsEventIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired ApplicationContext applicationContext;
  @Autowired CapturingMailSender mailSender;

  @Test
  void emailOtpCompletionPublishesUserGuestConvertedEvent() throws Exception {
    RecordingAuthEventPublisher events = findRecordingPublisher(applicationContext);
    assertThat(events).isNotNull();
    events.clear();

    JsonNode guest =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"password\":\"Correct horse battery staple\",\"guest\":true,\"device_info_json\":\"{}\"}"));

    mockMvc
        .perform(
            post("/api/v1/auth/convert-guest")
                .header("Authorization", "Bearer " + guest.get("access_token").asText())
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"nats-guest@example.com\",\"password\":\"New account password 1\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_id", is(guest.get("account_id").asText())))
        .andExpect(jsonPath("$.session.account_type", is("guest")));

    assertThat(events.publishedSubjects())
        .as("convert-guest submit must not publish before email verification")
        .doesNotContain("user.guest_converted");

    mailSender.clear();
    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"nats-guest@example.com\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isNoContent());
    String code = mailSender.lastCode();
    assertThat(code).matches("\\d{6}");

    mockMvc
        .perform(
            post("/api/v1/auth/otp/verify")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"nats-guest@example.com\",\"code\":\""
                        + code
                        + "\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isNoContent());

    assertThat(events.publishedSubjects())
        .as("verified email OTP must publish user.guest_converted exactly once")
        .containsExactly("user.guest_converted");
  }

  private JsonNode postJson(String path, String body) throws Exception {
    String response =
        mockMvc
            .perform(post(path).contentType(MediaType.APPLICATION_JSON).content(body))
            .andExpect(status().isOk())
            .andReturn()
            .getResponse()
            .getContentAsString();
    return objectMapper.readTree(response);
  }

  private static JsonNode session(JsonNode envelope) {
    assertThat(envelope.has("session")).isTrue();
    return envelope.get("session");
  }

  private static RecordingAuthEventPublisher findRecordingPublisher(ApplicationContext ctx) {
    for (String name : ctx.getBeanDefinitionNames()) {
      Object bean = ctx.getBean(name);
      if (bean instanceof RecordingAuthEventPublisher recording) {
        return recording;
      }
    }
    return null;
  }
}
