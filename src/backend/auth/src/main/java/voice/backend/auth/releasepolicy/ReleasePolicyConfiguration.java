package voice.backend.auth.releasepolicy;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.*;
import org.springframework.beans.factory.SmartInitializingSingleton;
import org.springframework.beans.factory.config.BeanFactoryPostProcessor;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import org.springframework.web.servlet.mvc.method.annotation.RequestMappingHandlerMapping;
import org.springframework.beans.factory.ObjectProvider;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.mail.MailSender;
import voice.backend.auth.mail.ResendMailSender;

@Configuration
public class ReleasePolicyConfiguration {
  /** Runs before ordinary DB, provider, scheduler and listener bean construction. */
  @Bean
  static BeanFactoryPostProcessor releasePolicyBootstrap(Environment environment) {
    return factory -> {
      if (ReleasePolicy.ALPHA.equals(environment.getProperty("VOICE_RELEASE_PROFILE"))) {
        String target = environment.getProperty("VOICE_RELEASE_SNAPSHOT_FILE");
        if (target == null || !Path.of(target).isAbsolute())
          throw new IllegalArgumentException("alpha requires private snapshot target");
      }
      factory.registerSingleton("ownerReleasePolicy",
          ReleasePolicy.load(environment::getProperty, packagedBuildSource()));
    };
  }

  private static String packagedBuildSource() {
    try (var input = ReleasePolicyConfiguration.class.getResourceAsStream("/voice-build-source.sha")) {
      return input == null ? "" : new String(input.readNBytes(41), StandardCharsets.US_ASCII);
    } catch (java.io.IOException ex) { throw new IllegalStateException("unreadable build source"); }
  }

  @Bean
  SmartInitializingSingleton authReleaseSnapshot(ReleasePolicy policy, AuthService auth,
      OtpService otp, MailSender mail, AuthGrpcService grpc, Environment env,
      voice.backend.auth.repository.AccountRepository accounts,
      voice.backend.auth.repository.RefreshTokenRepository refreshTokens,
      voice.backend.auth.repository.OtpCodeRepository otpCodes,
      ObjectProvider<RequestMappingHandlerMapping> mappings) {
    return () -> {
      if (!ReleasePolicy.ALPHA.equals(policy.profile())) return;
      if (auth.releasePolicy() != policy || otp.releasePolicy() != policy)
        throw new IllegalStateException("release guard identity mismatch");
      if (!"jdbc".equals(env.getProperty("auth.persistence", "jdbc"))
          || !(accounts instanceof voice.backend.auth.repository.JdbcAccountRepository)
          || !(refreshTokens instanceof voice.backend.auth.repository.JdbcRefreshTokenRepository)
          || !(otpCodes instanceof voice.backend.auth.repository.JdbcOtpCodeRepository)
          || !(mail instanceof ResendMailSender)
          || env.getProperty("auth.totp.test-bypass", Boolean.class, false))
        throw new IllegalStateException("alpha requires production Auth adapters");
      List<String> routes = new ArrayList<>();
      routes.addAll(List.of("owner:AuthService.register", "owner:AuthService.login",
          "owner:AuthService.convertGuest", "owner:AuthService.refresh", "owner:AuthService.verify2FA",
          "owner:AuthService.issueSessionForProfile", "owner:OtpService.sendOtp", "owner:OtpService.verifyOtp"));
      grpc.bindService().getMethods().forEach(method -> routes.add("grpc:/" + method.getMethodDescriptor().getFullMethodName()));
      mappings.orderedStream().forEach(mapping -> mapping.getHandlerMethods().forEach((info, handler) ->
          routes.add("rest:" + info.toString())));
      Collections.sort(routes);
      Map<String,String> adapters = Map.of("database", "jdbc", "mail", "real", "phone_provider", "absent");
      writeSnapshot(policy, env.getProperty("VOICE_RELEASE_SNAPSHOT_FILE"),
          instanceIdentity(), routes, adapters);
    };
  }

  private static String instanceIdentity() {
    try { return java.net.InetAddress.getLocalHost().getHostName(); }
    catch (java.net.UnknownHostException ex) { throw new IllegalStateException("unknown instance identity", ex); }
  }

  static void writeSnapshot(ReleasePolicy policy, String file, String instance,
      List<String> routes, Map<String,String> adapters) {
    if (file == null || !Path.of(file).isAbsolute() || instance == null || instance.isBlank())
      throw new IllegalArgumentException("alpha requires private snapshot target and instance identity");
    try {
      ObjectMapper json = new ObjectMapper();
      Map<String,Object> snapshot = new TreeMap<>();
      snapshot.put("schema_version", 2); snapshot.put("service_id", "auth");
      snapshot.put("instance_id", instance); snapshot.put("build_source_sha", policy.sourceSHA());
      snapshot.put("policy_sha256", policy.sha256()); snapshot.put("profile", policy.profile());
      snapshot.put("capability_states", policy.capabilityStates()); snapshot.put("adapter_modes", adapters);
      snapshot.put("guard_ids", List.of("auth.register_phone.v1", "auth.login_phone.v1",
          "auth.convert_guest_phone.v1", "auth.refresh_phone.v1", "auth.verify_2fa_phone.v1",
          "auth.issue_phone_session.v1", "auth.send_otp_phone.v1", "auth.verify_otp_phone.v1"));
      snapshot.put("route_inventory_sha256", ReleasePolicy.digest(json.writeValueAsBytes(routes)));
      snapshot.put("config_projection_sha256", ReleasePolicy.digest(json.writeValueAsBytes(new TreeMap<>(adapters))));
      byte[] data = json.writeValueAsBytes(snapshot);
      // Atomic exclusive creation, including symlink rejection; permissions exist before bytes.
      try (var channel = Files.newByteChannel(Path.of(file),
          Set.of(StandardOpenOption.CREATE_NEW, StandardOpenOption.WRITE, LinkOption.NOFOLLOW_LINKS),
          PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rw-------")))) {
        ByteBuffer buffer = ByteBuffer.wrap(data); while (buffer.hasRemaining()) channel.write(buffer);
      }
    } catch (java.io.IOException ex) { throw new IllegalStateException("cannot create private release snapshot", ex); }
  }
}
