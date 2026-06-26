package voice.backend.auth.web;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class HttpAccessLogFilterTest {
  @Autowired MockMvc mockMvc;

  @Test
  void logsHttpAccessWithStructuredMdc() throws Exception {
    Logger filterLogger = (Logger) LoggerFactory.getLogger(HttpAccessLogFilter.class);
    ListAppender<ILoggingEvent> appender = new ListAppender<>();
    appender.start();
    filterLogger.addAppender(appender);
    filterLogger.setLevel(Level.INFO);

    try {
      mockMvc
          .perform(get("/health").header(RequestIdFilter.HEADER, "access-req-1"))
          .andExpect(status().isOk());

      assertThat(appender.list).isNotEmpty();
      ILoggingEvent event =
          appender.list.stream()
              .filter(e -> "http_access".equals(e.getMDCPropertyMap().get("event")))
              .findFirst()
              .orElseThrow();
      assertThat(event.getMDCPropertyMap()).containsEntry("event", "http_access");
      assertThat(event.getMDCPropertyMap()).containsEntry("method", "GET");
      assertThat(event.getMDCPropertyMap()).containsEntry("path", "/health");
      assertThat(event.getMDCPropertyMap()).containsEntry("status", "200");
      assertThat(event.getMDCPropertyMap()).containsKey("duration_ms");
      assertThat(event.getMDCPropertyMap()).containsEntry("request_id", "access-req-1");
    } finally {
      filterLogger.detachAppender(appender);
    }
  }

  @Test
  void logsNonOkStatusOnFailedRequest() throws Exception {
    Logger filterLogger = (Logger) LoggerFactory.getLogger(HttpAccessLogFilter.class);
    ListAppender<ILoggingEvent> appender = new ListAppender<>();
    appender.start();
    filterLogger.addAppender(appender);
    filterLogger.setLevel(Level.INFO);

    try {
      mockMvc
          .perform(
              post("/api/v1/auth/login")
                  .contentType(MediaType.APPLICATION_JSON)
                  .content("{}"))
          .andExpect(status().isBadRequest());

      ILoggingEvent event =
          appender.list.stream()
              .filter(e -> "/api/v1/auth/login".equals(e.getMDCPropertyMap().get("path")))
              .findFirst()
              .orElseThrow();
      assertThat(event.getMDCPropertyMap()).containsEntry("event", "http_access");
      assertThat(event.getMDCPropertyMap()).containsEntry("method", "POST");
      assertThat(event.getMDCPropertyMap()).containsEntry("status", "400");
    } finally {
      filterLogger.detachAppender(appender);
    }
  }
}
