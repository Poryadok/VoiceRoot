package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import io.grpc.netty.shaded.io.netty.buffer.Unpooled;
import io.grpc.netty.shaded.io.netty.channel.Channel;
import io.grpc.netty.shaded.io.netty.channel.ChannelHandlerContext;
import io.grpc.netty.shaded.io.netty.handler.codec.http2.DefaultHttp2Headers;
import io.grpc.netty.shaded.io.netty.handler.codec.http2.Http2FrameLogger;
import io.grpc.netty.shaded.io.netty.handler.logging.LogLevel;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
import org.slf4j.LoggerFactory;
import org.springframework.boot.logging.LoggingInitializationContext;
import org.springframework.boot.logging.LoggingSystem;
import org.springframework.http.MediaType;
import org.springframework.mock.env.MockEnvironment;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;
import voice.backend.auth.web.HttpAccessLogFilter;

class LogbackCredentialPrivacyTest {
  @ParameterizedTest
  @ValueSource(strings = {"DEBUG", "TRACE"})
  void diagnosticRootDoesNotExposeBodiesOrFrames(String rootLevel) throws Exception {
    // A separate JVM exercises Boot's real XML loader without resetting other tests' logging.
    Path output = java.nio.file.Files.createTempFile("auth-logging-", ".log");
    Process process =
        new ProcessBuilder(
                Path.of(System.getProperty("java.home"), "bin", "java").toString(),
                "-cp",
                System.getProperty("surefire.test.class.path", System.getProperty("java.class.path")),
                LoggingProbe.class.getName(),
                rootLevel)
            .redirectErrorStream(true)
            .redirectOutput(output.toFile())
            .start();
    try {
      assertThat(process.waitFor(30, TimeUnit.SECONDS)).isTrue();
      String logs = java.nio.file.Files.readString(output, StandardCharsets.UTF_8);
      assertThat(process.exitValue()).as(logs).isZero();
      assertThat(logs)
          .doesNotContain("synthetic-password-canary", "synthetic-token-canary")
          .doesNotContain(java.util.HexFormat.of().formatHex(
              "synthetic-password-canary".getBytes(StandardCharsets.UTF_8)))
          .contains("safe-application-debug", "safe-transport-warning", "http request")
          .contains("\"event\":\"http_access\"", "\"method\":\"POST\"",
              "\"path\":\"/logging-probe\"", "\"status\":\"200\"",
              "\"request_id\":\"safe-request-id\"");
    } finally {
      process.destroyForcibly();
      java.nio.file.Files.deleteIfExists(output);
    }
  }

  public static class LoggingProbe {
    public static void main(String[] args) throws Exception {
      var environment = new MockEnvironment().withProperty("LOG_LEVEL", args[0]);
      LoggingSystem logging = LoggingSystem.get(LoggingProbe.class.getClassLoader());
      logging.beforeInitialize();
      logging.initialize(new LoggingInitializationContext(environment),
          "classpath:logback-spring.xml", null);
      try {
        var app = LoggerFactory.getLogger(LoggingProbe.class);
        app.debug("safe-application-debug");
        var channelContext = mock(ChannelHandlerContext.class);
        when(channelContext.channel()).thenReturn(mock(Channel.class));
        for (String name : new String[] {
            "io.grpc.netty.shaded.io.grpc.netty.NettyServerHandler",
            "io.grpc.netty.shaded.io.grpc.netty.NettyClientHandler"}) {
          var transport = LoggerFactory.getLogger(name);
          for (LogLevel level : new LogLevel[] {LogLevel.DEBUG, LogLevel.TRACE}) {
            var frames = new Http2FrameLogger(level, name);
            var data = Unpooled.copiedBuffer("synthetic-password-canary", StandardCharsets.UTF_8);
            try {
              frames.logData(Http2FrameLogger.Direction.INBOUND, channelContext, 1, data, 0, false);
              frames.logHeaders(Http2FrameLogger.Direction.INBOUND, channelContext, 1,
                  new DefaultHttp2Headers().add("authorization", "Bearer synthetic-token-canary"),
                  0, true);
            } finally {
              data.release();
            }
          }
          transport.warn("safe-transport-warning");
        }
        var mvc = MockMvcBuilders.standaloneSetup(new EchoController())
            .addFilters(new HttpAccessLogFilter()).build();
        org.slf4j.MDC.put("request_id", "safe-request-id");
        var response = mvc.perform(org.springframework.test.web.servlet.request.MockMvcRequestBuilders
                .post("/logging-probe").contentType(MediaType.TEXT_PLAIN)
                .content("synthetic-password-canary"))
            .andReturn().getResponse();
        if (response.getStatus() != 200
            || !response.getContentAsString().equals("synthetic-token-canary")) {
          throw new AssertionError("Logging probe did not exercise request/response conversion");
        }
      } finally {
        org.slf4j.MDC.clear();
        logging.cleanUp();
      }
    }
  }

  @RestController
  static class EchoController {
    @PostMapping("/logging-probe")
    String echo(@RequestBody String body) {
      if (!body.equals("synthetic-password-canary")) {
        throw new IllegalArgumentException("Invalid synthetic fixture");
      }
      return "synthetic-token-canary";
    }
  }
}
