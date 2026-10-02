package voice.backend.auth.authoritysource;

import java.net.*;
import java.nio.file.*;
import java.time.Duration;
import java.util.List;
import org.springframework.core.env.Environment;

record AuthSourceConfig(Path certificate,Path key,Path clientCA,URI jwks,Path jwksCA,
    String redisHost,int redisPort,String redisPassword,InetSocketAddress listen,
    Duration refresh,Duration hardExpiry,Duration cooldown) {
  private static final String PREFIX="AUTH_AUTHORITY_SOURCE_";
  static AuthSourceConfig load(Environment env) {
    String enabled=env.getProperty(PREFIX+"ENABLED");
    if(enabled==null) {
      for(String field:List.of("TLS_CERT_FILE","TLS_KEY_FILE","CLIENT_CA_FILE","JWKS_CA_FILE","REPLAY_REDIS_ADDR","REPLAY_REDIS_PASSWORD","GRPC_LISTEN","JWKS_REFRESH_AFTER","JWKS_HARD_EXPIRY","UNKNOWN_KID_COOLDOWN")) if(env.containsProperty(PREFIX+field))throw invalid();
      return null;
    }
    if(enabled.equals("false"))return null;
    if(!enabled.equals("true"))throw invalid();
    try {
      Path cert=file(env,"TLS_CERT_FILE",true),key=file(env,"TLS_KEY_FILE",true),ca=file(env,"CLIENT_CA_FILE",true),jwksCA=file(env,"JWKS_CA_FILE",false);
      var endpoints=AuthSourceVerifier.JSON.readTree(env.getRequiredProperty("S2S_JWKS_URLS_JSON"));
      if(endpoints==null || !endpoints.isObject())throw invalid();
      URI endpoint=URI.create(AuthSourceVerifier.text(endpoints,"federation"));
      if(!endpoint.getScheme().equals("https") || endpoint.getHost()==null || endpoint.getRawUserInfo()!=null || endpoint.getRawFragment()!=null)throw invalid();
      var redis=address(env.getRequiredProperty(PREFIX+"REPLAY_REDIS_ADDR"),false);
      var listen=address(env.getProperty(PREFIX+"GRPC_LISTEN",":9097"),true);
      Duration refresh=duration(env,"JWKS_REFRESH_AFTER",Duration.ofSeconds(30)),hard=duration(env,"JWKS_HARD_EXPIRY",Duration.ofMinutes(2)),cooldown=duration(env,"UNKNOWN_KID_COOLDOWN",Duration.ofSeconds(5));
      if(hard.compareTo(refresh)<0)throw invalid();
      return new AuthSourceConfig(cert,key,ca,endpoint,jwksCA,redis.getHostString(),redis.getPort(),env.getProperty(PREFIX+"REPLAY_REDIS_PASSWORD",""),listen,refresh,hard,cooldown);
    } catch(Exception ex){throw invalid();}
  }
  private static Path file(Environment env,String field,boolean required) {
    String value=env.getProperty(PREFIX+field);if(value==null && !required)return null;
    if(value==null || value.isBlank())throw invalid();var path=Path.of(value);
    if(!Files.isRegularFile(path) || !Files.isReadable(path))throw invalid();return path;
  }
  private static InetSocketAddress address(String raw,boolean listener) {
    int separator=raw.lastIndexOf(':');if(separator<0 || raw.chars().anyMatch(Character::isWhitespace))throw invalid();
    String host=raw.substring(0,separator),port=raw.substring(separator+1);
    if(host.startsWith("[") && host.endsWith("]"))host=host.substring(1,host.length()-1);
    if(host.isEmpty()){if(!listener)throw invalid();host="0.0.0.0";}
    if(host.contains("/") || host.contains("@") || !port.matches("[0-9]{1,5}"))throw invalid();int number=Integer.parseInt(port);
    if(number<0 || number>65535 || (!listener && number==0))throw invalid();return new InetSocketAddress(host,number);
  }
  private static Duration duration(Environment env,String name,Duration fallback) {
    String raw=env.getProperty(PREFIX+name);if(raw==null)return fallback;
    var matcher=java.util.regex.Pattern.compile("([0-9]+)(ms|s|m)").matcher(raw);if(!matcher.matches())throw invalid();long value=Long.parseLong(matcher.group(1));
    Duration result=switch(matcher.group(2)){case "ms"->Duration.ofMillis(value);case "s"->Duration.ofSeconds(value);default->Duration.ofMinutes(value);};
    if(result.isNegative() || result.isZero())throw invalid();return result;
  }
  private static IllegalArgumentException invalid(){return new IllegalArgumentException("invalid Auth authority source configuration");}
}
