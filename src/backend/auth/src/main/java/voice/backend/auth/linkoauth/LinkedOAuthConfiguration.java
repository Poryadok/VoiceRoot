package voice.backend.auth.linkoauth;

import java.net.URI;
import java.time.*;
import java.util.*;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.*;
import org.springframework.core.env.Environment;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.scheduling.annotation.EnableScheduling;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.LinkedAccountsService;
import voice.backend.auth.userdb.*;

@Configuration(proxyBeanMethods=false)
@ConditionalOnProperty(name="auth.linked-oauth.enabled",havingValue="true")
@EnableScheduling
public class LinkedOAuthConfiguration {
  @Bean LinkedOAuthSettings linkedOAuthSettings(Environment e) {
    if(!"jdbc".equals(e.getProperty("auth.persistence","jdbc"))) throw new IllegalStateException("linked OAuth requires Auth PostgreSQL");
    String prefix="auth.linked-oauth.";
    Map<String,LinkedOAuthSettings.Provider> providers=new HashMap<>();
    for(String name:List.of("twitch","youtube")) {
      String p=prefix+name+"."; boolean enabled=e.getProperty(p+"enabled",Boolean.class,false);
      if(!enabled)continue;
      Map<String,String> returns=new HashMap<>();
      if(e.getProperty(p+"web.enabled",Boolean.class,false)) {
        String target=e.getRequiredProperty(p+"web.return-uri"),origin=e.getRequiredProperty(prefix+"web-origin");
        LinkedOAuthSettings.exactHttps(target);
        URI u=URI.create(target),o=URI.create(origin);
        if(!"https".equals(o.getScheme()) || o.getHost()==null || o.getRawUserInfo()!=null || o.getRawQuery()!=null || o.getRawFragment()!=null
            || (o.getPath()!=null && !o.getPath().isEmpty()) || !o.getScheme().equals(u.getScheme()) || !o.getRawAuthority().equals(u.getRawAuthority()))
          throw new IllegalArgumentException("linked OAuth web return must match configured app origin");
        returns.put("web",target);
      }
      if(e.getProperty(p+"windows.enabled",Boolean.class,false)) {
        String target=e.getRequiredProperty(p+"windows.return-uri");LinkedOAuthSettings.exactWindows(target);returns.put("windows",target);
      }
      if(e.getProperty(p+"mobile.enabled",Boolean.class,false))throw new IllegalArgumentException("linked OAuth mobile return is not qualified");
      providers.put(name,new LinkedOAuthSettings.Provider(true,e.getProperty(p+"s256",Boolean.class,false),
          e.getRequiredProperty(p+"authorization-url"),e.getRequiredProperty(p+"token-url"),e.getRequiredProperty(p+"identity-url"),
          e.getRequiredProperty(p+"client-id"),e.getRequiredProperty(p+"client-secret"),e.getRequiredProperty(p+"callback-uri"),returns));
    }
    if(providers.isEmpty())throw new IllegalArgumentException("linked OAuth requires an enabled registration");
    var settings=new LinkedOAuthSettings(e.getRequiredProperty(prefix+"revision"),Duration.parse(e.getProperty(prefix+"attempt-ttl","PT300S")),
        Duration.parse(e.getProperty(prefix+"result-ttl","PT60S")),Base64.getDecoder().decode(e.getRequiredProperty(prefix+"encryption-key-base64")),providers);
    settings.validateProduction(); return settings;
  }
  @Bean LinkedOAuthService linkedOAuthService(NamedParameterJdbcTemplate jdbc,PlatformTransactionManager transactions,
      LinkedOAuthSettings settings,TokenBlacklist blacklist,ProfileSwitchValidator profiles,Clock clock,LinkedAccountsService linked) {
    if(profiles instanceof NoOpProfileSwitchValidator)throw new IllegalStateException("linked OAuth requires current User profile authority");
    linked.configureLinkedOAuthSourceTokens(new LinkedOAuthSourceTokens(settings.encryptionKey()));
    var twitch=settings.providers().get("twitch");
    if(twitch!=null && twitch.enabled())linked.configureLinkedOAuthTwitchRecheck(twitch.identityUrl(),twitch.clientId());
    return new LinkedOAuthService(jdbc,new TransactionTemplate(transactions),settings,blacklist,profiles,clock,new LinkedOAuthHttpProvider(settings));
  }
}
