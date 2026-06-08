package voice.backend.auth.web;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class RequestIdFilterTest {
  @Autowired MockMvc mockMvc;

  @Test
  void generatesRequestIdWhenMissing() throws Exception {
    var result = mockMvc.perform(get("/health")).andExpect(header().exists(RequestIdFilter.HEADER)).andReturn();
    String requestId = result.getResponse().getHeader(RequestIdFilter.HEADER);
    assertThat(requestId).isNotBlank().hasSize(32).matches("[0-9a-f]{32}");
  }

  @Test
  void preservesClientRequestId() throws Exception {
    mockMvc
        .perform(get("/health").header(RequestIdFilter.HEADER, "client-req-123"))
        .andExpect(header().string(RequestIdFilter.HEADER, "client-req-123"));
  }
}
