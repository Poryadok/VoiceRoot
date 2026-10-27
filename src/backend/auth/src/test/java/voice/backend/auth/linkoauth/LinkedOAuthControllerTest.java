package voice.backend.auth.linkoauth;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;
import java.time.Instant;
import java.util.*;
import org.junit.jupiter.api.*;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.test.web.servlet.*;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import voice.backend.auth.rest.AuthRestController;
import voice.backend.auth.service.*;

class LinkedOAuthControllerTest {
  final AuthService auth=mock(AuthService.class);
  final LinkedOAuthService service=mock(LinkedOAuthService.class);
  @SuppressWarnings("unchecked") final ObjectProvider<LinkedOAuthService> provider=mock(ObjectProvider.class);
  final TokenClaims claims=new TokenClaims(UUID.randomUUID().toString(),UUID.randomUUID().toString(),List.of(),"free",Instant.now().plusSeconds(300),"jti","regular",7);
  MockMvc mvc;
  @BeforeEach void setup(){
    when(auth.validate("Bearer fixture")).thenReturn(claims);when(provider.getIfAvailable()).thenReturn(service);
    mvc=MockMvcBuilders.standaloneSetup(new LinkedOAuthController(auth,provider)).build();
  }
  @Test void defaultOffAuthenticatesThenReturnsUnavailableAndNeverInvokesLegacyFallback() throws Exception {
    when(provider.getIfAvailable()).thenReturn(null);
    mvc.perform(post("/api/v1/auth/linked-accounts/twitch/link").header("Authorization","Bearer fixture").contentType("application/json")
        .content("{\"client_surface\":\"web\",\"return_challenge\":\""+"v".repeat(43)+"\",\"return_challenge_method\":\"S256\"}"))
        .andExpect(status().isServiceUnavailable()).andExpect(jsonPath("$.error").value("oauth_unavailable"));
    verify(auth).validate("Bearer fixture");verifyNoInteractions(service);
  }
  @Test void malformedDuplicateMixedAndUnknownCallbackParametersNeverReachService() throws Exception {
    String state="s".repeat(43);
    for(String query:List.of("state="+state+"&state=b&code=SECRET","state="+state+"&code=SECRET&error=denied",
        "state="+state+"&code=SECRET&redirect_uri=https://evil.test","code=SECRET",
        "state="+state+"&code=SECRET&scope=a&scope=b","state="+state+"&error=denied&error_description=a&error_description=b",
        "state="+state+"&code=SECRET&scope="+"a".repeat(4097),"state="+state+"&code=SECRET&unknown_metadata=x")) {
      mvc.perform(get("/api/v1/auth/linked-accounts/twitch/oauth-callback?"+query)).andExpect(status().isOk())
          .andExpect(header().string("Cache-Control","no-store")).andExpect(header().string("Referrer-Policy","no-referrer"))
          .andExpect(content().string(org.hamcrest.Matchers.not(org.hamcrest.Matchers.containsString("SECRET"))));
    }
    verifyNoInteractions(service);
  }
  @Test void bridgeContainsOnlyOpaqueReturnScrubsBeforeHandoffAndExplicitlyEmptiesFragment() throws Exception {
    UUID id=UUID.randomUUID();String handle="h".repeat(43);
    when(service.callback("twitch","s".repeat(43),"provider-code",null)).thenReturn(new LinkedOAuthService.Return(id,handle,"https://app.example.test/settings/oauth-return","web"));
    String html=mvc.perform(get("/api/v1/auth/linked-accounts/twitch/oauth-callback").param("state","s".repeat(43)).param("code","provider-code"))
        .andExpect(status().isOk()).andExpect(header().string("X-Content-Type-Options","nosniff")).andReturn().getResponse().getContentAsString();
    assertThat(html).contains("location.hash.length>1","history.replaceState(null,'',location.pathname)","if(!rejected)location.replace(","result_handle="+handle+"#");
    assertThat(html.indexOf("history.replaceState")).isLessThan(html.indexOf("location.replace("));
    assertThat(html).doesNotContain("provider-code","s".repeat(43),"access_token","return_verifier","client_secret");
  }
  @Test void legacyAuthenticatedCallbacksCannotMutateSource() throws Exception {
    var linked=mock(LinkedAccountsService.class);
    mvc=MockMvcBuilders.standaloneSetup(new AuthRestController(auth,linked,mock(OtpService.class))).build();
    for(String p:List.of("twitch","youtube"))mvc.perform(post("/api/v1/auth/linked-accounts/"+p+"/callback").header("Authorization","Bearer fixture")
        .contentType("application/json").content("{\"code\":\"mock-code\",\"redirect_uri\":\"https://evil.test\"}"))
        .andExpect(status().isServiceUnavailable()).andExpect(jsonPath("$.error").value("oauth_unavailable"));
    verifyNoInteractions(linked);
  }
  @Test void twitchDocumentedScopeAndErrorDescriptionAreInertBoundedMetadata() throws Exception {
    UUID id=UUID.randomUUID();String state="s".repeat(43),handle="h".repeat(43);
    when(service.callback("twitch",state,"provider-code",null)).thenReturn(new LinkedOAuthService.Return(id,handle,"https://app.example.test/settings/oauth-return","web"));
    when(service.callback("twitch",state,null,"access_denied")).thenReturn(new LinkedOAuthService.Return(id,handle,"https://app.example.test/settings/oauth-return","web"));
    String success=mvc.perform(get("/api/v1/auth/linked-accounts/twitch/oauth-callback").param("state",state).param("code","provider-code").param("scope","user:read:email"))
        .andExpect(status().isOk()).andReturn().getResponse().getContentAsString();
    assertThat(success).contains("result_handle="+handle).doesNotContain("provider-code","user:read:email");
    String denied=mvc.perform(get("/api/v1/auth/linked-accounts/twitch/oauth-callback").param("state",state).param("error","access_denied").param("error_description","PRIVATE-PROVIDER-DESCRIPTION"))
        .andExpect(status().isOk()).andReturn().getResponse().getContentAsString();
    assertThat(denied).contains("result_handle="+handle).doesNotContain("PRIVATE-PROVIDER-DESCRIPTION");
    verify(service).callback("twitch",state,"provider-code",null);verify(service).callback("twitch",state,null,"access_denied");
  }
}
