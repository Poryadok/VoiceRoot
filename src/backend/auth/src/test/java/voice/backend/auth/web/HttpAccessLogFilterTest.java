package voice.backend.auth.web;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import jakarta.servlet.ServletException;
import java.util.Map;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.NullAndEmptySource;
import org.junit.jupiter.params.provider.ValueSource;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class HttpAccessLogFilterTest {
  @Autowired MockMvc mockMvc;

  private final Logger filterLogger =
      (Logger) LoggerFactory.getLogger(HttpAccessLogFilter.class);
  private final ListAppender<ILoggingEvent> appender = new ListAppender<>();
  private Level originalLevel;

  @BeforeEach
  void captureAccessLogs() {
    originalLevel = filterLogger.getLevel();
    appender.start();
    filterLogger.addAppender(appender);
    filterLogger.setLevel(Level.INFO);
  }

  @AfterEach
  void stopCapturingAccessLogs() {
    filterLogger.detachAppender(appender);
    appender.stop();
    filterLogger.setLevel(originalLevel);
    MDC.remove("peer_ip");
  }

  @Test
  void logsHttpAccessWithStructuredMdc() throws Exception {
    mockMvc
        .perform(
            get("/health")
                .header(RequestIdFilter.HEADER, "access-req-1")
                .with(
                    request -> {
                      request.setRemoteAddr("192.0.2.24");
                      return request;
                    }))
        .andExpect(status().isOk());

    Map<String, String> mdc = accessEventForPath("/health").getMDCPropertyMap();
    assertThat(mdc).containsEntry("event", "http_access");
    assertThat(mdc).containsEntry("method", "GET");
    assertThat(mdc).containsEntry("path", "/health");
    assertThat(mdc).containsEntry("status", "200");
    assertThat(mdc).containsKey("duration_ms");
    assertThat(mdc).containsEntry("request_id", "access-req-1");
    assertThat(mdc).containsEntry("peer_ip", "192.0.2.24");
    assertThat(MDC.get("peer_ip")).isNull();
  }

  @Test
  void logsNonOkStatusOnFailedRequest() throws Exception {
    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{}"))
        .andExpect(status().isBadRequest());

    ILoggingEvent event = accessEventForPath("/api/v1/auth/login");
    assertThat(event.getMDCPropertyMap()).containsEntry("event", "http_access");
    assertThat(event.getMDCPropertyMap()).containsEntry("method", "POST");
    assertThat(event.getMDCPropertyMap()).containsEntry("status", "400");
  }

  @Test
  void configuredConsoleEncoderIncludesPeerIpInJsonOutput() throws Exception {
    MockHttpServletRequest request = requestWithRemoteAddr("192.0.2.99");
    new HttpAccessLogFilter().doFilter(request, new MockHttpServletResponse(), (ignored, response) -> {});
    var root = (Logger) LoggerFactory.getLogger(org.slf4j.Logger.ROOT_LOGGER_NAME);
    @SuppressWarnings("unchecked")
    var console = (ch.qos.logback.core.OutputStreamAppender<ILoggingEvent>) root.getAppender("CONSOLE");
    assertThat(console).isNotNull();
    byte[] encoded = console.getEncoder().encode(accessEventForPath("/audit-peer"));
    var json = new com.fasterxml.jackson.databind.ObjectMapper().readTree(encoded);
    assertThat(json.path("peer_ip").asText()).isEqualTo("192.0.2.99");
  }

  @ParameterizedTest
  @ValueSource(strings = {"198.51.100.17", "2001:db8:85a3::8a2e:370:7334"})
  void logsValidIpv4AndIpv6ServletPeerAddresses(String remoteAddr) throws Exception {
    MockHttpServletRequest request = requestWithRemoteAddr(remoteAddr);

    new HttpAccessLogFilter()
        .doFilter(request, new MockHttpServletResponse(), (ignoredRequest, ignoredResponse) -> {});

    assertThat(accessEventForPath("/audit-peer").getMDCPropertyMap())
        .containsEntry("peer_ip", remoteAddr);
  }

  @Test
  void ignoresSpoofedForwardingHeaders() throws Exception {
    MockHttpServletRequest request = requestWithRemoteAddr("192.0.2.41");
    request.addHeader("X-Forwarded-For", "203.0.113.99, 198.51.100.8");
    request.addHeader("Forwarded", "for=203.0.113.100;proto=https");

    new HttpAccessLogFilter()
        .doFilter(request, new MockHttpServletResponse(), (ignoredRequest, ignoredResponse) -> {});

    assertThat(accessEventForPath("/audit-peer").getMDCPropertyMap())
        .containsEntry("peer_ip", "192.0.2.41");
  }

  @Test
  void capturesServletPeerAddressBeforeCallingDownstreamChain() throws Exception {
    MockHttpServletRequest request = requestWithRemoteAddr("198.51.100.22");

    new HttpAccessLogFilter()
        .doFilter(
            request,
            new MockHttpServletResponse(),
            (downstreamRequest, ignoredResponse) ->
                ((MockHttpServletRequest) downstreamRequest).setRemoteAddr("203.0.113.77"));

    assertThat(accessEventForPath("/audit-peer").getMDCPropertyMap())
        .containsEntry("peer_ip", "198.51.100.22");
  }

  @ParameterizedTest
  @NullAndEmptySource
  @ValueSource(
      strings = {
        "   ",
        "localhost",
        "999.999.999.999",
        "203.0.113.7\nforged-log-field"
      })
  void logsUnknownForMissingOrInvalidServletPeerAddress(String remoteAddr) throws Exception {
    MockHttpServletRequest request = requestWithRemoteAddr(remoteAddr);
    request.addHeader("X-Forwarded-For", "198.51.100.200");

    new HttpAccessLogFilter()
        .doFilter(request, new MockHttpServletResponse(), (ignoredRequest, ignoredResponse) -> {});

    assertThat(accessEventForPath("/audit-peer").getMDCPropertyMap())
        .containsEntry("peer_ip", "unknown");
  }

  @Test
  void clearsPeerIpMdcWhenDownstreamChainThrows() {
    MockHttpServletRequest request = requestWithRemoteAddr("2001:db8::44");
    MDC.put("peer_ip", "stale-peer");

    assertThatThrownBy(
            () ->
                new HttpAccessLogFilter()
                    .doFilter(
                        request,
                        new MockHttpServletResponse(),
                        (ignoredRequest, ignoredResponse) -> {
                          throw new ServletException("downstream failed");
                        }))
        .isInstanceOf(ServletException.class)
        .hasMessage("downstream failed");

    assertThat(accessEventForPath("/audit-peer").getMDCPropertyMap())
        .containsEntry("peer_ip", "2001:db8::44");
    assertThat(MDC.get("peer_ip")).isNull();
  }

  private ILoggingEvent accessEventForPath(String path) {
    return appender.list.stream()
        .filter(event -> path.equals(event.getMDCPropertyMap().get("path")))
        .findFirst()
        .orElseThrow();
  }

  private static MockHttpServletRequest requestWithRemoteAddr(String remoteAddr) {
    MockHttpServletRequest request = new MockHttpServletRequest("GET", "/audit-peer");
    request.setRemoteAddr(remoteAddr);
    return request;
  }
}
