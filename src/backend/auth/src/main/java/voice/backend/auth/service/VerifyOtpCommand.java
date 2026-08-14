package voice.backend.auth.service;

public record VerifyOtpCommand(
    String email, String phone, String code, String otpType, String accessToken) {}
