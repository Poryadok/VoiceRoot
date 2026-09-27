package voice.backend.auth.sdkidentity;

/** Only the verified provider identity is imported; email and profile claims are excluded. */
public record VerifiedProviderSubject(String issuer, String subject) {}
