package voice.backend.auth.service;

import java.util.List;

public record TotpEnrollment(String totpUri, String secretBackupHint, List<String> backupCodes) {}
